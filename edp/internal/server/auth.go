package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	keyAdminHash = "admin_password_hash"
	keyAPIToken  = "api_token"
)

// ensureSecrets loads (generating on first run) the global API token and applies
// the admin password from config.
//
// EDP_ADMIN_PASSWORD is authoritative: when set, it is (re)applied on every boot
// if it differs from what's stored. This means a forgotten password can always
// be recovered by setting the env var and restarting — there is no dead-end. If
// the env var is empty, the first login lets the user set the password, and any
// later change persists.
func (s *Server) ensureSecrets(ctx context.Context) error {
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

func (s *Server) validAPIToken(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(after)), []byte(s.apiToken)) == 1
	}
	return false
}

// requireAPI wraps an API handler, accepting the global token as a Bearer header.
func (s *Server) requireAPI(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.validAPIToken(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

// apiAuthStatus reports whether an admin password has been set yet, so the UI can
// show a first-run "set a password" screen instead of a normal login.
func (s *Server) apiAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]bool{"configured": s.adminConfigured(r.Context())})
}

// apiLogin exchanges the admin password for the global API token. On first run
// (no password set yet) it sets the password from the supplied value. This is the
// only unauthenticated write endpoint; it is rate-limited by bcrypt's cost.
func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pw := loginPassword(r)
	firstRun := !s.adminConfigured(ctx)

	if firstRun {
		if len(pw) < 8 {
			writeJSON(w, 400, map[string]string{"error": "password must be at least 8 characters"})
			return
		}
		if err := s.setAdminPassword(ctx, pw); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
	} else if !s.checkPassword(ctx, pw) {
		writeJSON(w, 401, map[string]string{"error": "incorrect password"})
		return
	}
	writeJSON(w, 200, map[string]any{"token": s.apiToken, "first_run": firstRun})
}

// loginPassword reads the password from a JSON body or a posted form.
func loginPassword(r *http.Request) string {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		return body.Password
	}
	return r.FormValue("password")
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
