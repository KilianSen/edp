package aggregate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"edp-manager/internal/store"
)

func TestFanoutMergesAndTags(t *testing.T) {
	// two healthy edp stubs returning env arrays, one broken stub
	prod := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"id":1,"name":"api"},{"id":2,"name":"web"}]`))
	}))
	defer prod.Close()
	staging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"id":1,"name":"api"}]`))
	}))
	defer staging.Close()
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer broken.Close()

	insts := []*store.Instance{
		{ID: 1, Label: "prod", BaseURL: prod.URL},
		{ID: 2, Label: "staging", BaseURL: staging.URL},
		{ID: 3, Label: "broken", BaseURL: broken.URL},
	}

	res := Fanout(context.Background(), insts, "/api/environments", 3*time.Second)

	if len(res.Items) != 3 {
		t.Fatalf("items = %d, want 3 (2 prod + 1 staging)", len(res.Items))
	}
	if len(res.Errors) != 1 || res.Errors[0].InstanceID != 3 {
		t.Fatalf("errors = %+v, want one for instance 3", res.Errors)
	}
	// every row must be tagged with its origin instance
	for _, row := range res.Items {
		if row["instance_id"] == nil || row["instance_label"] == nil {
			t.Errorf("row missing instance tag: %+v", row)
		}
		if row["name"] == nil {
			t.Errorf("row lost original env fields: %+v", row)
		}
	}
}
