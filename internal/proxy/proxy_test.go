package proxy

import (
	"net/http/httptest"
	"testing"
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
