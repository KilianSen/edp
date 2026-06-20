package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st, dir
}

func TestEnvironmentCRUDAndCredentialEncryption(t *testing.T) {
	ctx := context.Background()
	st, dir := openTest(t)

	e := &Environment{
		Name:             "demo",
		SourceType:       SourceGit,
		DeployType:       DeployContainer,
		RepoURL:          "https://example.com/repo.git",
		GitToken:         "ghp_SUPERSECRET",
		RegistryPassword: "registry-pw-SECRET",
		WebhookToken:     "tok",
	}
	if err := st.CreateEnvironment(ctx, e); err != nil {
		t.Fatal(err)
	}
	if e.ID == 0 {
		t.Fatal("expected assigned id")
	}

	got, err := st.GetEnvironment(ctx, e.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if got.GitToken != "ghp_SUPERSECRET" || got.RegistryPassword != "registry-pw-SECRET" {
		t.Errorf("credentials did not round-trip: %q / %q", got.GitToken, got.RegistryPassword)
	}

	// credentials must not be stored in plaintext in the DB file
	raw, err := os.ReadFile(filepath.Join(dir, "edp.db"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ghp_SUPERSECRET") || strings.Contains(string(raw), "registry-pw-SECRET") {
		t.Error("plaintext credential found in DB file — encryption at rest failed")
	}

	// update
	got.GitRef = "main"
	if err := st.UpdateEnvironment(ctx, got); err != nil {
		t.Fatal(err)
	}
	again, _ := st.GetEnvironment(ctx, e.ID)
	if again.GitRef != "main" || again.GitToken != "ghp_SUPERSECRET" {
		t.Errorf("update lost data: ref=%q token=%q", again.GitRef, again.GitToken)
	}

	// list + delete
	if list, _ := st.ListEnvironments(ctx); len(list) != 1 {
		t.Fatalf("expected 1 env, got %d", len(list))
	}
	if err := st.DeleteEnvironment(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := st.ListEnvironments(ctx); len(list) != 0 {
		t.Fatalf("expected 0 envs after delete, got %d", len(list))
	}
}

func TestMarshalJSONHidesSecrets(t *testing.T) {
	e := Environment{GitToken: "secret", RegistryPassword: "secret2"}
	b, err := e.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret") {
		t.Errorf("MarshalJSON leaked a secret: %s", b)
	}
}

func TestDeploymentTimingAndEstimate(t *testing.T) {
	ctx := context.Background()
	st, _ := openTest(t)
	e := &Environment{Name: "svc", SourceType: SourceGit, DeployType: DeployContainer}
	if err := st.CreateEnvironment(ctx, e); err != nil {
		t.Fatal(err)
	}

	// no successful deploys yet -> estimate 0
	if got := st.EstimateDurationMs(ctx, e.ID); got != 0 {
		t.Errorf("expected 0 estimate with no deploys, got %d", got)
	}

	for _, dur := range []int64{1000, 2000, 3000} {
		d := &Deployment{EnvID: e.ID, Trigger: TriggerManual, Reason: "test", Status: StatusQueued}
		if err := st.CreateDeployment(ctx, d); err != nil {
			t.Fatal(err)
		}
		if err := st.FinishDeployment(ctx, d.ID, StatusSuccess, "sha", "digest", dur, dur/2); err != nil {
			t.Fatal(err)
		}
	}
	// average of last 3 successful (1000,2000,3000) = 2000
	if got := st.EstimateDurationMs(ctx, e.ID); got != 2000 {
		t.Errorf("expected estimate 2000, got %d", got)
	}

	latest, _ := st.LatestDeployment(ctx, e.ID)
	if latest == nil || latest.Reason != "test" || latest.DurationMs != 3000 || latest.ReadyMs != 1500 {
		t.Errorf("latest deployment fields wrong: %+v", latest)
	}
}

func TestTimedHooksAndRuns(t *testing.T) {
	ctx := context.Background()
	st, _ := openTest(t)
	e := &Environment{Name: "svc", SourceType: SourceGit, DeployType: DeployContainer}
	if err := st.CreateEnvironment(ctx, e); err != nil {
		t.Fatal(err)
	}

	h := &TimedHook{EnvID: e.ID, Name: "cleanup", Schedule: "15m", Script: "print('hi')", Enabled: true}
	if err := st.CreateTimedHook(ctx, h); err != nil {
		t.Fatal(err)
	}
	if hooks, _ := st.ListTimedHooks(ctx, e.ID); len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if all, _ := st.ListAllTimedHooks(ctx); len(all) != 1 {
		t.Fatalf("expected 1 hook from ListAll, got %d", len(all))
	}

	run := &HookRun{HookID: h.ID, Trigger: TriggerManual, Status: StatusQueued}
	if err := st.CreateHookRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkHookRunRunning(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendHookRunLog(ctx, run.ID, "line1\n"); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishHookRun(ctx, run.ID, StatusSuccess); err != nil {
		t.Fatal(err)
	}
	latest, _ := st.LatestHookRun(ctx, h.ID)
	if latest == nil || latest.Status != StatusSuccess || !strings.Contains(latest.Log, "line1") {
		t.Errorf("hook run not recorded correctly: %+v", latest)
	}

	// deleting the env cascades to hooks (FK ON DELETE CASCADE)
	if err := st.DeleteEnvironment(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if hooks, _ := st.ListTimedHooks(ctx, e.ID); len(hooks) != 0 {
		t.Errorf("expected hooks to cascade-delete with env, got %d", len(hooks))
	}
}

func TestResetInterrupted(t *testing.T) {
	ctx := context.Background()
	st, _ := openTest(t)
	e := &Environment{Name: "svc", SourceType: SourceGit, DeployType: DeployContainer, Status: StatusRunning}
	if err := st.CreateEnvironment(ctx, e); err != nil {
		t.Fatal(err)
	}
	d := &Deployment{EnvID: e.ID, Trigger: TriggerManual, Status: StatusQueued}
	st.CreateDeployment(ctx, d)
	st.MarkDeploymentRunning(ctx, d.ID) // now "running"
	h := &TimedHook{EnvID: e.ID, Name: "h", Status: StatusRunning}
	st.CreateTimedHook(ctx, h)
	run := &HookRun{HookID: h.ID, Trigger: TriggerManual, Status: StatusRunning}
	st.CreateHookRun(ctx, run)
	st.MarkHookRunRunning(ctx, run.ID)

	if err := st.ResetInterrupted(ctx); err != nil {
		t.Fatal(err)
	}

	got, _ := st.GetEnvironment(ctx, e.ID)
	if got.Status == StatusRunning {
		t.Error("env still 'running' after reset")
	}
	dep, _ := st.GetDeployment(ctx, d.ID)
	if dep.Status != StatusFailed || dep.FinishedAt == nil {
		t.Errorf("deployment not failed-out: status=%s finished=%v", dep.Status, dep.FinishedAt)
	}
	hr, _ := st.GetHookRun(ctx, run.ID)
	if hr.Status != StatusFailed {
		t.Errorf("hook run not failed-out: %s", hr.Status)
	}
	gh, _ := st.GetTimedHook(ctx, h.ID)
	if gh.Status == StatusRunning {
		t.Error("hook still 'running' after reset")
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	st, _ := openTest(t)
	if _, ok, _ := st.GetSetting(ctx, "k"); ok {
		t.Error("expected missing setting")
	}
	if err := st.SetSetting(ctx, "k", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, "k", "v2"); err != nil { // upsert
		t.Fatal(err)
	}
	if v, ok, _ := st.GetSetting(ctx, "k"); !ok || v != "v2" {
		t.Errorf("expected v2, got %q ok=%v", v, ok)
	}
}
