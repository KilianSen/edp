// Package hooks runs timed hooks: scheduled Python scripts that operate on an
// environment WITHOUT redeploying it. The script is handed EDP_* variables
// (including the live container name / compose project) so it can, for example,
// `docker exec` into the running container to do periodic maintenance.
package hooks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"edp/internal/logbus"
	"edp/internal/naming"
	"edp/internal/sh"
	"edp/internal/store"
)

const runTimeout = 30 * time.Minute

type Runner struct {
	st        *store.Store
	bus       *logbus.Bus
	workspace string
	pythonBin string

	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func New(st *store.Store, bus *logbus.Bus, workspace, pythonBin string) *Runner {
	if pythonBin == "" {
		pythonBin = "python3"
	}
	return &Runner{st: st, bus: bus, workspace: workspace, pythonBin: pythonBin, locks: map[int64]*sync.Mutex{}}
}

// Bus exposes the hook-run log bus for the SSE handler.
func (r *Runner) Bus() *logbus.Bus { return r.bus }

func (r *Runner) lockFor(hookID int64) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locks[hookID] == nil {
		r.locks[hookID] = &sync.Mutex{}
	}
	return r.locks[hookID]
}

// TriggerHook queues a hook run and starts it asynchronously, returning the run id.
func (r *Runner) TriggerHook(ctx context.Context, hookID int64, trigger string) (int64, error) {
	run := &store.HookRun{HookID: hookID, Trigger: trigger, Status: store.StatusQueued}
	if err := r.st.CreateHookRun(ctx, run); err != nil {
		return 0, err
	}
	go r.work(hookID, run.ID)
	return run.ID, nil
}

func (r *Runner) work(hookID, runID int64) {
	lock := r.lockFor(hookID)
	lock.Lock()
	defer lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	lw := newLogWriter(r.st, r.bus, runID)
	defer r.bus.Close(runID)

	_ = r.st.MarkHookRunRunning(ctx, runID)
	_ = r.st.SetTimedHookStatus(ctx, hookID, store.StatusRunning)

	status := store.StatusSuccess
	if err := r.run(ctx, lw, hookID); err != nil {
		lw.Printf("\nHOOK FAILED: %v\n", err)
		status = store.StatusFailed
	} else {
		lw.Printf("\nHOOK OK\n")
	}
	_ = r.st.FinishHookRun(ctx, runID, status)
	_ = r.st.SetTimedHookStatus(ctx, hookID, status)
}

func (r *Runner) run(ctx context.Context, lw *logWriter, hookID int64) error {
	hook, err := r.st.GetTimedHook(ctx, hookID)
	if err != nil || hook == nil {
		return fmt.Errorf("hook %d not found", hookID)
	}
	env, err := r.st.GetEnvironment(ctx, hook.EnvID)
	if err != nil || env == nil {
		return fmt.Errorf("environment %d not found", hook.EnvID)
	}

	repoDir := filepath.Join(r.workspace, fmt.Sprintf("env-%d", env.ID))
	hookEnv := []string{
		"EDP_ENV_ID=" + fmt.Sprint(env.ID),
		"EDP_ENV_NAME=" + env.Name,
		"EDP_DEPLOY_TYPE=" + env.DeployType,
		"EDP_SOURCE_TYPE=" + env.SourceType,
		"EDP_REPO_DIR=" + repoDir,
		"EDP_HOOK_NAME=" + hook.Name,
		"EDP_CONTAINER=" + naming.ContainerName(env.Name),
		"EDP_COMPOSE_PROJECT=" + naming.ComposeProject(env.ID),
	}
	// the env's own variables are available to the hook script too
	for _, line := range strings.Split(env.RunEnv, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			hookEnv = append(hookEnv, l)
		}
	}

	lw.Printf("== timed hook %q (env %s) ==\n", hook.Name, env.Name)
	return sh.Stream(ctx, lw, sh.Opts{Env: hookEnv, EchoArgs: []string{"hook", hook.Name}}, r.pythonBin, "-c", hook.Script)
}
