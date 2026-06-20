// Package source fetches git repositories into the workspace for building.
package source

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"edp/internal/sh"
)

// authURL injects a token into an https git URL so private repos can be cloned
// without an interactive credential prompt. Non-https URLs are returned as-is.
func authURL(repo, token string) (string, string) {
	if token == "" {
		return repo, repo
	}
	u, err := url.Parse(repo)
	if err != nil || u.Scheme != "https" {
		return repo, repo
	}
	display := *u
	u.User = url.UserPassword("x-access-token", token)
	display.User = url.UserPassword("x-access-token", "***")
	return u.String(), display.String()
}

// Ensure clones repo into dir (or fetches if already present), checks out ref,
// and returns the resulting commit SHA. ref may be a branch, tag, or sha; empty
// means the remote default branch.
func Ensure(ctx context.Context, out io.Writer, dir, repo, ref, token string) (string, error) {
	real, display := authURL(repo, token)

	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return "", err
		}
		if err := sh.Stream(ctx, out, sh.Opts{EchoArgs: []string{"clone", display, dir}},
			"git", "clone", real, dir); err != nil {
			return "", fmt.Errorf("git clone: %w", err)
		}
	} else {
		if err := sh.Stream(ctx, out, sh.Opts{Dir: dir, EchoArgs: []string{"fetch", "origin"}},
			"git", "-C", dir, "fetch", "--prune", real); err != nil {
			return "", fmt.Errorf("git fetch: %w", err)
		}
	}

	if ref != "" {
		// Prefer the remote-tracking ref when ref is a branch; fall back to ref.
		target := ref
		if _, err := sh.Capture(ctx, "git", "-C", dir, "rev-parse", "--verify", "origin/"+ref); err == nil {
			target = "origin/" + ref
		}
		if err := sh.Stream(ctx, out, sh.Opts{Dir: dir}, "git", "-C", dir, "checkout", "-f", target); err != nil {
			return "", fmt.Errorf("git checkout %s: %w", ref, err)
		}
	}

	sha, err := sh.Capture(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}
