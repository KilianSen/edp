package scheduler

import (
	"testing"
	"time"
)

func newTestScheduler() *Scheduler { return New(nil, nil, nil) }

func TestDueDuration(t *testing.T) {
	s := newTestScheduler()
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	fire := map[int64]time.Time{}

	if !s.due("30m", now.Add(-31*time.Minute), now, fire, 1) {
		t.Error("expected due when 31m elapsed for a 30m schedule")
	}
	if s.due("30m", now.Add(-10*time.Minute), now, fire, 1) {
		t.Error("expected NOT due when only 10m elapsed for a 30m schedule")
	}
}

func TestDueCron(t *testing.T) {
	s := newTestScheduler()
	now := time.Date(2026, 6, 19, 12, 0, 30, 0, time.UTC)
	fire := map[int64]time.Time{}

	// every minute: last run two minutes ago -> due now
	if !s.due("*/1 * * * *", now.Add(-2*time.Minute), now, fire, 1) {
		t.Error("expected due for every-minute cron after 2m")
	}
	// daily at midnight: last run at 12:00 -> next is tomorrow midnight, not due
	if s.due("0 0 * * *", now, now, fire, 1) {
		t.Error("expected NOT due for midnight cron just after a run")
	}
}

func TestDueDebounce(t *testing.T) {
	s := newTestScheduler()
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	// last real run was long ago, but we just fired 1 minute ago -> debounced
	fire := map[int64]time.Time{1: now.Add(-1 * time.Minute)}
	if s.due("30m", now.Add(-2*time.Hour), now, fire, 1) {
		t.Error("expected debounce to suppress a due check right after firing")
	}
}

func TestDueUnparseableSchedule(t *testing.T) {
	s := newTestScheduler()
	now := time.Now()
	if s.due("not-a-schedule", now.Add(-time.Hour), now, map[int64]time.Time{}, 1) {
		t.Error("unparseable schedule should never be due")
	}
}

func TestPruneFire(t *testing.T) {
	s := newTestScheduler()
	fire := map[int64]time.Time{1: time.Now(), 2: time.Now(), 3: time.Now()}
	s.pruneFire(fire, map[int64]bool{2: true})
	if len(fire) != 1 {
		t.Fatalf("expected 1 entry after prune, got %d", len(fire))
	}
	if _, ok := fire[2]; !ok {
		t.Error("expected live id 2 to remain")
	}
}
