package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"edp/internal/store"
)

func TestExternalHostScheme_TrustToggles(t *testing.T) {
	r := httptest.NewRequest("GET", "http://edp.internal:8080/", nil)
	r.Host = "edp.internal:8080"
	r.Header.Set("X-Forwarded-Host", "app.test.local")
	r.Header.Set("X-Forwarded-Proto", "https")

	// trust OFF: forwarded headers are ignored (no spoofing)
	off := &Proxy{trustProxy: false}
	if got := off.externalHost(r); got != "edp.internal:8080" {
		t.Errorf("trust off host = %q, want request host", got)
	}
	if got := off.externalScheme(r); got != "http" {
		t.Errorf("trust off scheme = %q, want http", got)
	}

	// trust ON: forwarded headers win
	on := &Proxy{trustProxy: true}
	if got := on.externalHost(r); got != "app.test.local" {
		t.Errorf("trust on host = %q, want forwarded host", got)
	}
	if got := on.externalScheme(r); got != "https" {
		t.Errorf("trust on scheme = %q, want https", got)
	}
}

func TestFirstToken(t *testing.T) {
	for in, want := range map[string]string{
		"app.test.local":               "app.test.local",
		"app.test.local, edp.internal": "app.test.local",
		"  https , http ":              "https",
		"":                             "",
	} {
		if got := firstToken(in); got != want {
			t.Errorf("firstToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplashFor(t *testing.T) {
	withGlobal := &Proxy{splashURL: "https://global/ui"}
	if got := withGlobal.splashFor(&store.Environment{}); got != "https://global/ui" {
		t.Errorf("global default = %q", got)
	}
	// per-env overrides the global
	if got := withGlobal.splashFor(&store.Environment{SplashURL: "https://env/ui"}); got != "https://env/ui" {
		t.Errorf("per-env override = %q", got)
	}
	// nothing configured → built-in (empty)
	if got := (&Proxy{}).splashFor(&store.Environment{}); got != "" {
		t.Errorf("no splash = %q, want empty", got)
	}
}

func TestRedirectToSplash(t *testing.T) {
	p := &Proxy{}
	env := &store.Environment{Name: "artemis", WebhookToken: "tok123"}

	cases := []struct {
		name       string
		prefix     string
		wantCtl    string
		wantReturn string
	}{
		{"host-based", "", "http://app.test.local", "http://app.test.local/dash?x=1"},
		{"path-based", "/e/artemis", "http://app.test.local/e/artemis", "http://app.test.local/e/artemis/dash?x=1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := "/dash?x=1"
			if c.prefix != "" {
				path = c.prefix + "/dash?x=1"
			}
			r := httptest.NewRequest("GET", "http://app.test.local"+path, nil)
			r.Host = "app.test.local"
			w := httptest.NewRecorder()

			p.redirectToSplash(w, r, env, c.prefix, "https://ui.example.com/splash", true, 5000, 1000)

			if w.Code != http.StatusFound {
				t.Fatalf("code = %d, want 302", w.Code)
			}
			if w.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Error("token may leak: Referrer-Policy not set to no-referrer")
			}
			u, err := url.Parse(w.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			if u.Host != "ui.example.com" || u.Path != "/splash" {
				t.Fatalf("redirect base = %s", u)
			}
			q := u.Query()
			for k, want := range map[string]string{
				"env":    "artemis",
				"state":  "deploying",
				"token":  "tok123",
				"ctl":    c.wantCtl,
				"return": c.wantReturn,
			} {
				if q.Get(k) != want {
					t.Errorf("%s = %q, want %q", k, q.Get(k), want)
				}
			}
		})
	}
}

func TestSetCORS(t *testing.T) {
	w := httptest.NewRecorder()
	(&Proxy{corsOrigin: "https://ui.example.com"}).setCORS(w)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://ui.example.com" {
		t.Errorf("allow-origin = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("missing allow-methods")
	}
}

func TestIsNavigation(t *testing.T) {
	nav := httptest.NewRequest("GET", "/", nil)
	nav.Header.Set("Accept", "text/html,application/xhtml+xml")
	if !isNavigation(nav) {
		t.Error("expected html GET to be a navigation")
	}
	asset := httptest.NewRequest("GET", "/app.css", nil)
	asset.Header.Set("Accept", "text/css")
	if isNavigation(asset) {
		t.Error("expected css request not to be a navigation")
	}
	post := httptest.NewRequest("POST", "/", nil)
	post.Header.Set("Accept", "text/html")
	if isNavigation(post) {
		t.Error("expected POST not to be a navigation")
	}
	// Sec-Fetch-Dest wins when present
	doc := httptest.NewRequest("GET", "/", nil)
	doc.Header.Set("Sec-Fetch-Dest", "document")
	if !isNavigation(doc) {
		t.Error("expected Sec-Fetch-Dest=document to be a navigation")
	}
}
