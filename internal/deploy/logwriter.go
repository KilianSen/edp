package deploy

import (
	"context"
	"fmt"

	"edp/internal/logbus"
	"edp/internal/store"
)

// logWriter is an io.Writer that persists deploy output to the store and fans
// it out to live SSE subscribers via the log bus.
type logWriter struct {
	st       *store.Store
	bus      *logbus.Bus
	deployID int64
}

func newLogWriter(st *store.Store, bus *logbus.Bus, deployID int64) *logWriter {
	return &logWriter{st: st, bus: bus, deployID: deployID}
}

func (w *logWriter) Write(p []byte) (int, error) {
	s := string(p)
	_ = w.st.AppendDeploymentLog(context.Background(), w.deployID, s)
	w.bus.Publish(w.deployID, s)
	return len(p), nil
}

func (w *logWriter) Printf(format string, args ...any) {
	_, _ = w.Write([]byte(fmt.Sprintf(format, args...)))
}
