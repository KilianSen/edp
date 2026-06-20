package store

import (
	"context"
	"database/sql"
)

const hookColumns = `id,env_id,name,schedule,script,enabled,status,created_at,updated_at`

func scanHook(sc interface{ Scan(...any) error }) (*TimedHook, error) {
	var h TimedHook
	var enabled int
	var created, updated string
	if err := sc.Scan(&h.ID, &h.EnvID, &h.Name, &h.Schedule, &h.Script, &enabled,
		&h.Status, &created, &updated); err != nil {
		return nil, err
	}
	h.Enabled = enabled != 0
	h.CreatedAt = parseTime(created)
	h.UpdatedAt = parseTime(updated)
	return &h, nil
}

func (s *Store) ListTimedHooks(ctx context.Context, envID int64) ([]*TimedHook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+hookColumns+` FROM timed_hooks WHERE env_id=? ORDER BY name`, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectHooks(rows)
}

// ListAllTimedHooks returns every hook (used by the scheduler).
func (s *Store) ListAllTimedHooks(ctx context.Context) ([]*TimedHook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+hookColumns+` FROM timed_hooks ORDER BY env_id, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectHooks(rows)
}

func collectHooks(rows *sql.Rows) ([]*TimedHook, error) {
	out := []*TimedHook{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		h, err := scanHook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) GetTimedHook(ctx context.Context, id int64) (*TimedHook, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+hookColumns+` FROM timed_hooks WHERE id=?`, id)
	h, err := scanHook(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

func (s *Store) CreateTimedHook(ctx context.Context, h *TimedHook) error {
	now := nowStr()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO timed_hooks(env_id,name,schedule,script,enabled,status,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		h.EnvID, h.Name, h.Schedule, h.Script, boolToInt(h.Enabled), firstNonEmpty(h.Status, "idle"), now, now)
	if err != nil {
		return err
	}
	h.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UpdateTimedHook(ctx context.Context, h *TimedHook) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE timed_hooks SET name=?,schedule=?,script=?,enabled=?,status=?,updated_at=? WHERE id=?`,
		h.Name, h.Schedule, h.Script, boolToInt(h.Enabled), h.Status, nowStr(), h.ID)
	return err
}

func (s *Store) SetTimedHookStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE timed_hooks SET status=?, updated_at=? WHERE id=?`,
		status, nowStr(), id)
	return err
}

func (s *Store) DeleteTimedHook(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM timed_hooks WHERE id=?`, id)
	return err
}

// ---- hook runs ----

const hookRunColumns = `id,hook_id,trigger,status,started_at,finished_at,log`

func scanHookRun(sc interface{ Scan(...any) error }) (*HookRun, error) {
	var r HookRun
	var started, finished sql.NullString
	if err := sc.Scan(&r.ID, &r.HookID, &r.Trigger, &r.Status, &started, &finished, &r.Log); err != nil {
		return nil, err
	}
	r.StartedAt = parseTimePtr(started)
	r.FinishedAt = parseTimePtr(finished)
	return &r, nil
}

func (s *Store) CreateHookRun(ctx context.Context, r *HookRun) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO hook_runs(hook_id,trigger,status,log) VALUES(?,?,?,'')`,
		r.HookID, r.Trigger, firstNonEmpty(r.Status, StatusQueued))
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) GetHookRun(ctx context.Context, id int64) (*HookRun, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+hookRunColumns+` FROM hook_runs WHERE id=?`, id)
	r, err := scanHookRun(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func (s *Store) ListHookRuns(ctx context.Context, hookID int64, limit int) ([]*HookRun, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+hookRunColumns+` FROM hook_runs WHERE hook_id=? ORDER BY id DESC LIMIT ?`, hookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*HookRun{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		r, err := scanHookRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) LatestHookRun(ctx context.Context, hookID int64) (*HookRun, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+hookRunColumns+` FROM hook_runs WHERE hook_id=? ORDER BY id DESC LIMIT 1`, hookID)
	r, err := scanHookRun(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func (s *Store) MarkHookRunRunning(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE hook_runs SET status=?, started_at=? WHERE id=?`, StatusRunning, nowStr(), id)
	return err
}

func (s *Store) AppendHookRunLog(ctx context.Context, id int64, chunk string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE hook_runs SET log = log || ? WHERE id=?`, chunk, id)
	return err
}

func (s *Store) FinishHookRun(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE hook_runs SET status=?, finished_at=? WHERE id=?`, status, nowStr(), id)
	return err
}
