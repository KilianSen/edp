// Package aggregate fans a read request out to every registered edp instance in
// parallel and merges the results, tagging each row with the instance it came
// from and collecting per-instance errors so one unreachable edp degrades
// gracefully instead of failing the whole response.
package aggregate

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"edp-manager/internal/edpclient"
	"edp-manager/internal/store"
)

// InstanceError reports that one instance failed during a fan-out.
type InstanceError struct {
	InstanceID int64  `json:"instance_id"`
	Label      string `json:"instance_label"`
	Error      string `json:"error"`
}

// Result is the merged response: rows from all healthy instances plus the errors
// from any that failed.
type Result struct {
	Items  []map[string]any `json:"items"`
	Errors []InstanceError  `json:"errors"`
}

// Fanout calls GET <path> on every instance concurrently, expecting each to
// return a JSON array of objects. Every object is tagged with instance_id and
// instance_label and appended to Items. perCallTimeout bounds each instance.
func Fanout(ctx context.Context, instances []*store.Instance, path string, perCallTimeout time.Duration) Result {
	var mu sync.Mutex
	res := Result{Items: []map[string]any{}, Errors: []InstanceError{}}

	var wg sync.WaitGroup
	for _, inst := range instances {
		wg.Add(1)
		go func(inst *store.Instance) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, perCallTimeout)
			defer cancel()

			body, err := edpclient.New(inst.BaseURL, inst.APIToken).GetJSON(cctx, path)
			if err != nil {
				mu.Lock()
				res.Errors = append(res.Errors, InstanceError{InstanceID: inst.ID, Label: inst.Label, Error: err.Error()})
				mu.Unlock()
				return
			}
			var rows []map[string]any
			if err := json.Unmarshal(body, &rows); err != nil {
				mu.Lock()
				res.Errors = append(res.Errors, InstanceError{InstanceID: inst.ID, Label: inst.Label, Error: "decode: " + err.Error()})
				mu.Unlock()
				return
			}
			mu.Lock()
			for _, row := range rows {
				row["instance_id"] = inst.ID
				row["instance_label"] = inst.Label
				res.Items = append(res.Items, row)
			}
			mu.Unlock()
		}(inst)
	}
	wg.Wait()
	return res
}
