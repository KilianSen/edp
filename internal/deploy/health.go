package deploy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"edp/internal/docker"
	"edp/internal/naming"
	"edp/internal/store"
)

const (
	healthTimeout  = 2 * time.Minute
	healthInterval = 2 * time.Second
)

var healthClient = &http.Client{Timeout: 5 * time.Second}

// runHealthCheck polls the env's health check until it passes, returning the
// elapsed time since start (the "ready" time). It returns 0,nil when no health
// check is configured, and an error if the check never passes within the
// timeout (which fails the deploy).
func (e *Engine) runHealthCheck(ctx context.Context, lw *logWriter, env *store.Environment, start time.Time) (int64, error) {
	if env.HealthType == "" || env.HealthType == store.HealthNone {
		return 0, nil
	}
	if env.HealthTarget == "" {
		lw.Printf("\nwarning: health_type=%s but no target set; skipping health check\n", env.HealthType)
		return 0, nil
	}
	// Compose stacks have no single edp-<name> container, so the exec check (and
	// name-based http targets) don't apply — use compose's own `healthcheck:`.
	if env.DeployType == store.DeployCompose {
		lw.Printf("\nhealth check skipped for compose stacks (use a compose healthcheck instead)\n")
		return 0, nil
	}

	lw.Printf("\n== Health check (%s: %s) ==\n", env.HealthType, env.HealthTarget)
	deadline := time.Now().Add(healthTimeout)
	attempt := 0
	for {
		attempt++
		ok, detail := e.probe(ctx, env)
		if ok {
			ready := time.Since(start)
			lw.Printf("healthy after %s (%d attempt(s))\n", ready.Round(time.Millisecond), attempt)
			return ready.Milliseconds(), nil
		}
		if time.Now().After(deadline) {
			return time.Since(start).Milliseconds(),
				fmt.Errorf("health check did not pass within %s: %s", healthTimeout, detail)
		}
		lw.Printf("  attempt %d: not ready (%s); retrying in %s\n", attempt, detail, healthInterval)
		select {
		case <-ctx.Done():
			return time.Since(start).Milliseconds(), ctx.Err()
		case <-time.After(healthInterval):
		}
	}
}

// probe runs one health check, returning whether it passed and a short detail.
func (e *Engine) probe(ctx context.Context, env *store.Environment) (bool, string) {
	switch env.HealthType {
	case store.HealthHTTP:
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, env.HealthTarget, nil)
		if err != nil {
			return false, err.Error()
		}
		resp, err := healthClient.Do(req)
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return true, fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)

	case store.HealthExec:
		// run the command inside the env's container via docker exec
		err := e.dk.Run(ctx, nil, docker.RunOpts{},
			"exec", naming.ContainerName(env.Name), "sh", "-c", env.HealthTarget)
		if err != nil {
			return false, err.Error()
		}
		return true, "exit 0"

	default:
		return true, "no check"
	}
}
