// Package scheduler triggers periodic work: full redeploys (per environment)
// and timed hooks (per hook) that run without redeploying. A schedule may be a
// Go duration ("30m", "6h") or a standard cron expression ("0 */6 * * *").
package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"edp/internal/store"
)

const tick = 30 * time.Second

type Engine interface {
	Trigger(ctx context.Context, envID int64, trigger, reason string) (int64, error)
}

type HookEngine interface {
	TriggerHook(ctx context.Context, hookID int64, trigger string) (int64, error)
}

type Scheduler struct {
	st     *store.Store
	engine Engine
	hooks  HookEngine
	parser cron.Parser

	mu       sync.Mutex
	envFire  map[int64]time.Time
	hookFire map[int64]time.Time
	stop     chan struct{}
}

func New(st *store.Store, engine Engine, hooks HookEngine) *Scheduler {
	return &Scheduler{
		st:       st,
		engine:   engine,
		hooks:    hooks,
		parser:   cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		envFire:  map[int64]time.Time{},
		hookFire: map[int64]time.Time{},
		stop:     make(chan struct{}),
	}
}

func (s *Scheduler) Start() { go s.loop() }

func (s *Scheduler) Stop() { close(s.stop) }

func (s *Scheduler) loop() {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			now := time.Now()
			s.evaluateEnvs(now)
			s.evaluateHooks(now)
		}
	}
}

func (s *Scheduler) evaluateEnvs(now time.Time) {
	ctx := context.Background()
	envs, err := s.st.ListEnvironments(ctx)
	if err != nil {
		log.Printf("scheduler: list envs: %v", err)
		return
	}
	live := make(map[int64]bool, len(envs))
	for _, e := range envs {
		live[e.ID] = true
	}
	s.pruneFire(s.envFire, live)
	for _, e := range envs {
		if e.RedeploySchedule == "" || e.Status == store.StatusRunning {
			continue
		}
		last := s.envLastRun(ctx, e)
		if s.due(e.RedeploySchedule, last, now, s.envFire, e.ID) {
			s.markFired(s.envFire, e.ID, now)
			if _, err := s.engine.Trigger(ctx, e.ID, store.TriggerSchedule, "scheduled ("+e.RedeploySchedule+")"); err != nil {
				log.Printf("scheduler: trigger env %d: %v", e.ID, err)
			} else {
				log.Printf("scheduler: redeploying env %d (%s)", e.ID, e.Name)
			}
		}
	}
}

func (s *Scheduler) evaluateHooks(now time.Time) {
	ctx := context.Background()
	hooks, err := s.st.ListAllTimedHooks(ctx)
	if err != nil {
		log.Printf("scheduler: list hooks: %v", err)
		return
	}
	live := make(map[int64]bool, len(hooks))
	for _, h := range hooks {
		live[h.ID] = true
	}
	s.pruneFire(s.hookFire, live)
	for _, h := range hooks {
		if !h.Enabled || h.Schedule == "" || h.Status == store.StatusRunning {
			continue
		}
		last := s.hookLastRun(ctx, h)
		if s.due(h.Schedule, last, now, s.hookFire, h.ID) {
			s.markFired(s.hookFire, h.ID, now)
			if _, err := s.hooks.TriggerHook(ctx, h.ID, store.TriggerSchedule); err != nil {
				log.Printf("scheduler: trigger hook %d: %v", h.ID, err)
			} else {
				log.Printf("scheduler: running timed hook %d (%s)", h.ID, h.Name)
			}
		}
	}
}

// due reports whether something with the given schedule is due at now, given the
// reference time of its last run and a debounce map keyed by id.
func (s *Scheduler) due(schedule string, last, now time.Time, fire map[int64]time.Time, id int64) bool {
	s.mu.Lock()
	if lf, ok := fire[id]; ok && lf.After(last) {
		last = lf
	}
	s.mu.Unlock()

	if d, err := time.ParseDuration(schedule); err == nil {
		return now.Sub(last) >= d
	}
	if sched, err := s.parser.Parse(schedule); err == nil {
		return !sched.Next(last).After(now)
	}
	return false // unparseable schedule: skip
}

func (s *Scheduler) markFired(fire map[int64]time.Time, id int64, now time.Time) {
	s.mu.Lock()
	fire[id] = now
	s.mu.Unlock()
}

// pruneFire drops debounce entries for ids that no longer exist, so the maps
// don't leak as environments/hooks are deleted over time.
func (s *Scheduler) pruneFire(fire map[int64]time.Time, live map[int64]bool) {
	s.mu.Lock()
	for id := range fire {
		if !live[id] {
			delete(fire, id)
		}
	}
	s.mu.Unlock()
}

func (s *Scheduler) envLastRun(ctx context.Context, e *store.Environment) time.Time {
	if d, _ := s.st.LatestDeployment(ctx, e.ID); d != nil && d.StartedAt != nil {
		return *d.StartedAt
	}
	return e.UpdatedAt
}

func (s *Scheduler) hookLastRun(ctx context.Context, h *store.TimedHook) time.Time {
	if r, _ := s.st.LatestHookRun(ctx, h.ID); r != nil && r.StartedAt != nil {
		return *r.StartedAt
	}
	return h.UpdatedAt
}
