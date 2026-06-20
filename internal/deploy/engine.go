// Package deploy is the heart of edp: it turns an environment definition into a
// running container or compose stack. Jobs run serially per environment (one
// in-flight deploy per env) while different envs deploy concurrently.
package deploy

import (
	"context"
	"sync"
	"time"

	"edp/internal/clearsite"
	"edp/internal/docker"
	"edp/internal/logbus"
	"edp/internal/store"
)

const deployTimeout = 30 * time.Minute

type Engine struct {
	st        *store.Store
	dk        *docker.Client
	bus       *logbus.Bus
	clear     *clearsite.Flags
	workspace string
	pythonBin string

	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func New(st *store.Store, dk *docker.Client, bus *logbus.Bus, clear *clearsite.Flags, workspace, pythonBin string) *Engine {
	if pythonBin == "" {
		pythonBin = "python3"
	}
	return &Engine{st: st, dk: dk, bus: bus, clear: clear, workspace: workspace, pythonBin: pythonBin, locks: map[int64]*sync.Mutex{}}
}

func (e *Engine) lockFor(envID int64) *sync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.locks[envID] == nil {
		e.locks[envID] = &sync.Mutex{}
	}
	return e.locks[envID]
}

// Trigger queues a deployment for an env and starts it asynchronously. It
// returns the new deployment ID immediately. reason is a human-readable note
// (or derived label) explaining why the deploy was triggered.
func (e *Engine) Trigger(ctx context.Context, envID int64, trigger, reason string) (int64, error) {
	d := &store.Deployment{EnvID: envID, Trigger: trigger, Reason: reason, Status: store.StatusQueued}
	if err := e.st.CreateDeployment(ctx, d); err != nil {
		return 0, err
	}
	go e.work(envID, d.ID)
	return d.ID, nil
}

// work runs a single deployment, serialized against other deploys of the same env.
func (e *Engine) work(envID, deployID int64) {
	lock := e.lockFor(envID)
	lock.Lock()
	defer lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	start := time.Now()
	lw := newLogWriter(e.st, e.bus, deployID)
	defer e.bus.Close(deployID)

	_ = e.st.MarkDeploymentRunning(ctx, deployID)
	_ = e.st.SetEnvironmentStatus(ctx, envID, store.StatusRunning)

	env, err := e.st.GetEnvironment(ctx, envID)
	if err != nil || env == nil {
		lw.Printf("error: environment %d not found\n", envID)
		e.finish(ctx, env, deployID, store.StatusFailed, "", "", time.Since(start), 0)
		return
	}

	commit, digest, readyMs, err := e.deploy(ctx, lw, env, start)
	status := store.StatusSuccess
	if err != nil {
		lw.Printf("\nDEPLOY FAILED: %v\n", err)
		status = store.StatusFailed
	} else {
		lw.Printf("\nDEPLOY OK (commit=%s, %s)\n", short(commit), time.Since(start).Round(time.Millisecond))
		if e.clear != nil && env.ClearSiteData != "" {
			e.clear.Mark(env.ID, env.ClearSiteData)
			lw.Printf("browser data will be cleared on next proxied visit (%s)\n", env.ClearSiteData)
		}
	}
	e.finish(ctx, env, deployID, status, commit, digest, time.Since(start), readyMs)
}

func (e *Engine) finish(ctx context.Context, env *store.Environment, deployID int64, status, commit, digest string, dur time.Duration, readyMs int64) {
	_ = e.st.FinishDeployment(ctx, deployID, status, commit, digest, dur.Milliseconds(), readyMs)
	if env != nil {
		_ = e.st.SetEnvironmentStatus(ctx, env.ID, status)
	}
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
