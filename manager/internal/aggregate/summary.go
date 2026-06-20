package aggregate

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"edp-manager/internal/edpclient"
	"edp-manager/internal/store"
)

// InstanceHealth is a per-instance roll-up: whether it's reachable and how many
// environments it has in each status.
type InstanceHealth struct {
	InstanceID   int64          `json:"instance_id"`
	Label        string         `json:"instance_label"`
	BaseURL      string         `json:"base_url"`
	Reachable    bool           `json:"reachable"`
	Error        string         `json:"error,omitempty"`
	Environments int            `json:"environments"`
	ByStatus     map[string]int `json:"by_status"`
}

// Totals aggregates the per-instance health across the whole fleet.
type Totals struct {
	Instances    int            `json:"instances"`
	Reachable    int            `json:"reachable"`
	Environments int            `json:"environments"`
	ByStatus     map[string]int `json:"by_status"`
}

// SummaryResult is the manager's fleet health overview.
type SummaryResult struct {
	Instances []InstanceHealth `json:"instances"`
	Totals    Totals           `json:"totals"`
}

// Summarize fans out to each instance's <path> (an env array), and rolls the
// results up per instance and across the fleet. Unreachable instances are marked
// reachable=false with their error, not dropped.
func Summarize(ctx context.Context, instances []*store.Instance, path string, perCallTimeout time.Duration) SummaryResult {
	healths := make([]InstanceHealth, len(instances))
	var wg sync.WaitGroup
	for idx, inst := range instances {
		wg.Add(1)
		go func(idx int, inst *store.Instance) {
			defer wg.Done()
			h := InstanceHealth{
				InstanceID: inst.ID,
				Label:      inst.Label,
				BaseURL:    inst.BaseURL,
				ByStatus:   map[string]int{},
			}
			cctx, cancel := context.WithTimeout(ctx, perCallTimeout)
			defer cancel()

			body, err := edpclient.New(inst.BaseURL, inst.APIToken).GetJSON(cctx, path)
			if err != nil {
				h.Error = err.Error()
				healths[idx] = h
				return
			}
			var rows []map[string]any
			if err := json.Unmarshal(body, &rows); err != nil {
				h.Error = "decode: " + err.Error()
				healths[idx] = h
				return
			}
			h.Reachable = true
			h.Environments = len(rows)
			for _, row := range rows {
				st, _ := row["status"].(string)
				if st == "" {
					st = "unknown"
				}
				h.ByStatus[st]++
			}
			healths[idx] = h
		}(idx, inst)
	}
	wg.Wait()

	res := SummaryResult{Instances: healths, Totals: Totals{Instances: len(healths), ByStatus: map[string]int{}}}
	for _, h := range healths {
		if h.Reachable {
			res.Totals.Reachable++
		}
		res.Totals.Environments += h.Environments
		for st, n := range h.ByStatus {
			res.Totals.ByStatus[st] += n
		}
	}
	sort.Slice(res.Instances, func(i, j int) bool { return res.Instances[i].Label < res.Instances[j].Label })
	return res
}
