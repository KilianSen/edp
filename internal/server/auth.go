package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	keyAdminHash     = "admin_password_hash"
	keySessionSecret = "session_secret"
	keyAPIToken      = "api_token"
	sessionCookie    = "edp_session"
	sessionTTL       = 30 * 24 * time.Hour
)

// ensureSecrets loads (generating on first run) the session secret and API
// token, and applies the admin password from config.
//
// EDP_ADMIN_PASSWORD is authoritative: when set, it is (re)applied on every boot
// if it differs from what's stored. This means a forgotten password can always
// be recovered by setting the env var and restarting — there is no dead-end. If
// the env var is empty, the first-run flow lets the user set the password in the
// browser, and any later change persists.
func (s *Server) ensureSecrets(ctx context.Context) error {
	secret, err := s.getOrCreateSecret(ctx, keySessionSecret)
	if err != nil {
		return err
	}
	s.sessionSecret = []byte(secret)

	tok, err := s.getOrCreateSecret(ctx, keyAPIToken)
	if err != nil {
		return err
	}
	s.apiToken = tok

	if s.cfg.AdminPassword != "" {
		hash, ok, _ := s.st.GetSetting(ctx, keyAdminHash)
		switch {
		case !ok:
			log.Print("seeding admin password from EDP_ADMIN_PASSWORD")
		case bcrypt.CompareHashAndPassword([]byte(hash), []byte(s.cfg.AdminPassword)) != nil:
			log.Print("resetting admin password from EDP_ADMIN_PASSWORD (env var differs from stored)")
		default:
			return nil // already matches; nothing to do
		}
		if err := s.setAdminPassword(ctx, s.cfg.AdminPassword); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) getOrCreateSecret(ctx context.Context, key string) (string, error) {
	if v, ok, err := s.st.GetSetting(ctx, key); err != nil {
		return "", err
	} else if ok {
		return v, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	v := hex.EncodeToString(b)
	return v, s.st.SetSetting(ctx, key, v)
}

func (s *Server) adminConfigured(ctx context.Context) bool {
	_, ok, _ := s.st.GetSetting(ctx, keyAdminHash)
	return ok
}

func (s *Server) setAdminPassword(ctx context.Context, pw string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.st.SetSetting(ctx, keyAdminHash, string(hash))
}

func (s *Server) checkPassword(ctx context.Context, pw string) bool {
	hash, ok, _ := s.st.GetSetting(ctx, keyAdminHash)
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// ---- signed session cookie: "<expiryUnix>.<hmac>" ----

func (s *Server) signSession(exp int64) string {
	payload := strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, s.sessionSecret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) validSession(cookie string) bool {
	parts := strings.SplitN(cookie, ".", 2)
	if len(parts) != 2 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	mac := hmac.New(sha256.New, s.sessionSecret)
	mac.Write([]byte(parts[0]))
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(parts[1]))
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request) {
	exp := time.Now().Add(sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.signSession(exp.Unix()),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.externalIsHTTPS(r), // browser↔proxy is TLS even though proxy↔edp is plain
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	})
}

func (s *Server) loggedIn(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	return err == nil && s.validSession(c.Value)
}

func (s *Server) validAPIToken(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(after)), []byte(s.apiToken)) == 1
	}
	return false
}

// requireWeb wraps a page handler, redirecting to /login when not authenticated.
func (s *Server) requireWeb(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.loggedIn(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

// requireAPI wraps an API handler, accepting a session cookie or Bearer token.
func (s *Server) requireAPI(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.loggedIn(r) && !s.validAPIToken(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func (s *Server) pageLogin(w http.ResponseWriter, r *http.Request) {
	if s.loggedIn(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "login.html", map[string]any{
		"FirstRun": !s.adminConfigured(r.Context()),
	})
}

func (s *Server) doLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pw := r.FormValue("password")
	// First-run: set the admin password.
	if !s.adminConfigured(ctx) {
		if len(pw) < 8 {
			s.render(w, "login.html", map[string]any{"FirstRun": true, "Error": "Password must be at least 8 characters."})
			return
		}
		if err := s.setAdminPassword(ctx, pw); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.setSessionCookie(w, r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !s.checkPassword(ctx, pw) {
		s.render(w, "login.html", map[string]any{"Error": "Incorrect password."})
		return
	}
	s.setSessionCookie(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) doLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// randomToken returns a hex token for webhooks/etc.
func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}
