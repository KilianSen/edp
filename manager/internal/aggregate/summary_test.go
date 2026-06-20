package aggregate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"edp-manager/internal/store"
)

func TestSummarizeCountsAndReachability(t *testing.T) {
	prod := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"id":1,"status":"success"},{"id":2,"status":"failed"},{"id":3,"status":"success"}]`))
	}))
	defer prod.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer down.Close()

	insts := []*store.Instance{
		{ID: 1, Label: "prod", BaseURL: prod.URL},
		{ID: 2, Label: "down", BaseURL: down.URL},
	}
	res := Summarize(context.Background(), insts, "/api/overview", 3*time.Second)

	if res.Totals.Instances != 2 || res.Totals.Reachable != 1 {
		t.Fatalf("totals = %+v, want 2 instances / 1 reachable", res.Totals)
	}
	if res.Totals.Environments != 3 {
		t.Fatalf("env total = %d, want 3", res.Totals.Environments)
	}
	if res.Totals.ByStatus["success"] != 2 || res.Totals.ByStatus["failed"] != 1 {
		t.Fatalf("by_status = %+v", res.Totals.ByStatus)
	}
	// instances are sorted by label: down, prod
	if res.Instances[0].Label != "down" || res.Instances[0].Reachable {
		t.Errorf("expected 'down' unreachable first, got %+v", res.Instances[0])
	}
	if res.Instances[0].Error == "" {
		t.Error("unreachable instance should carry an error")
	}
	if !res.Instances[1].Reachable || res.Instances[1].Environments != 3 {
		t.Errorf("prod health wrong: %+v", res.Instances[1])
	}
}
