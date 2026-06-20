// Package edpclient talks to a single edp instance's JSON API, injecting that
// instance's Bearer token. It backs both the fan-out aggregator (GET helpers)
// and the per-instance pass-through proxy (ReverseProxy).
package edpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	base  string
	token string
	hc    *http.Client
}

func New(base, token string) *Client {
	return &Client{
		base:  strings.TrimRight(base, "/"),
		token: token,
		hc:    &http.Client{Timeout: 0}, // per-call timeout comes from the context
	}
}

// GetJSON fetches base+path and returns the raw body. Non-2xx is an error that
// includes a snippet of the body, so a fan-out can report which instance failed.
func (c *Client) GetJSON(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode/100 != 2 {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("edp %s returned %d: %s", path, res.StatusCode, snippet)
	}
	return body, nil
}

// Ping checks that the instance is reachable and the token is accepted.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	_, err := c.GetJSON(ctx, "/api/status")
	return err
}

// ReverseProxy builds a proxy that forwards <stripPrefix>/api/... to this
// instance, injecting the Bearer token. FlushInterval -1 streams SSE promptly.
func (c *Client) ReverseProxy(stripPrefix string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(c.base)
	if err != nil {
		return nil, err
	}
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.FlushInterval = -1 // immediate flush so SSE log streams pass through live
	token := c.token
	inner := rp.Director
	rp.Director = func(req *http.Request) {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		inner(req)
		req.Host = target.Host
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return rp, nil
}
