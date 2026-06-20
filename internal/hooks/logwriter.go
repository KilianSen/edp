package hooks

import (
	"context"
	"fmt"

	"edp/internal/logbus"
	"edp/internal/store"
)

// logWriter persists hook-run output to the store and fans it out to live SSE
// subscribers via the hook log bus.
type logWriter struct {
	st    *store.Store
	bus   *logbus.Bus
	runID int64
}

func newLogWriter(st *store.Store, bus *logbus.Bus, runID int64) *logWriter {
	return &logWriter{st: st, bus: bus, runID: runID}
}

func (w *logWriter) Write(p []byte) (int, error) {
	s := string(p)
	_ = w.st.AppendHookRunLog(context.Background(), w.runID, s)
	w.bus.Publish(w.runID, s)
	return len(p), nil
}

func (w *logWriter) Printf(format string, args ...any) {
	_, _ = w.Write([]byte(fmt.Sprintf(format, args...)))
}
