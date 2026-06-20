package store

import (
	"context"
	"database/sql"
)

const deployColumns = `id,env_id,trigger,reason,status,started_at,finished_at,log,commit_sha,image_digest,duration_ms,ready_ms`

func scanDeployment(sc interface{ Scan(...any) error }) (*Deployment, error) {
	var d Deployment
	var started, finished sql.NullString
	err := sc.Scan(&d.ID, &d.EnvID, &d.Trigger, &d.Reason, &d.Status, &started, &finished,
		&d.Log, &d.CommitSHA, &d.ImageDigest, &d.DurationMs, &d.ReadyMs)
	if err != nil {
		return nil, err
	}
	d.StartedAt = parseTimePtr(started)
	d.FinishedAt = parseTimePtr(finished)
	return &d, nil
}

// CreateDeployment inserts a new deployment row (typically status=queued).
func (s *Store) CreateDeployment(ctx context.Context, d *Deployment) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO deployments(env_id,trigger,reason,status,log) VALUES(?,?,?,?,'')`,
		d.EnvID, d.Trigger, d.Reason, firstNonEmpty(d.Status, StatusQueued))
	if err != nil {
		return err
	}
	d.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) GetDeployment(ctx context.Context, id int64) (*Deployment, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+deployColumns+` FROM deployments WHERE id=?`, id)
	d, err := scanDeployment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

func (s *Store) ListDeployments(ctx context.Context, envID int64, limit int) ([]*Deployment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+deployColumns+` FROM deployments WHERE env_id=? ORDER BY id DESC LIMIT ?`, envID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Deployment{} // non-nil so an empty result marshals to [] not null
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// LatestDeployment returns the most recent deployment for an env, or nil.
func (s *Store) LatestDeployment(ctx context.Context, envID int64) (*Deployment, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+deployColumns+` FROM deployments WHERE env_id=? ORDER BY id DESC LIMIT 1`, envID)
	d, err := scanDeployment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

func (s *Store) MarkDeploymentRunning(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE deployments SET status=?, started_at=? WHERE id=?`, StatusRunning, nowStr(), id)
	return err
}

func (s *Store) AppendDeploymentLog(ctx context.Context, id int64, chunk string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE deployments SET log = log || ? WHERE id=?`, chunk, id)
	return err
}

func (s *Store) FinishDeployment(ctx context.Context, id int64, status, commitSHA, imageDigest string, durationMs, readyMs int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE deployments SET status=?, finished_at=?, commit_sha=?, image_digest=?, duration_ms=?, ready_ms=? WHERE id=?`,
		status, nowStr(), commitSHA, imageDigest, durationMs, readyMs, id)
	return err
}

// EstimateDurationMs returns a simple ETA for an env's next deploy: the average
// total duration of its last few successful deployments (0 if none yet).
func (s *Store) EstimateDurationMs(ctx context.Context, envID int64) int64 {
	rows, err := s.db.QueryContext(ctx,
		`SELECT duration_ms FROM deployments
		 WHERE env_id=? AND status=? AND duration_ms>0 ORDER BY id DESC LIMIT 3`, envID, StatusSuccess)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var sum, n int64
	for rows.Next() {
		var d int64
		if rows.Scan(&d) == nil {
			sum += d
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}
