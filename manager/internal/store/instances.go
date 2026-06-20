package store

import (
	"context"
	"database/sql"
	"strings"
)

const instanceCols = `id,label,base_url,api_token,created_at,updated_at`

func (s *Store) scanInstance(sc interface{ Scan(...any) error }) (*Instance, error) {
	var i Instance
	var created, updated string
	if err := sc.Scan(&i.ID, &i.Label, &i.BaseURL, &i.APIToken, &created, &updated); err != nil {
		return nil, err
	}
	i.CreatedAt = parseTime(created)
	i.UpdatedAt = parseTime(updated)
	tok, err := s.box.Decrypt(i.APIToken)
	if err != nil {
		return nil, err
	}
	i.APIToken = tok
	return &i, nil
}

func (s *Store) ListInstances(ctx context.Context) ([]*Instance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+instanceCols+` FROM instances ORDER BY label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Instance
	for rows.Next() {
		i, err := s.scanInstance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) GetInstance(ctx context.Context, id int64) (*Instance, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+instanceCols+` FROM instances WHERE id=?`, id)
	i, err := s.scanInstance(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return i, err
}

// InstanceByLabel returns the instance with the given label, or nil. Used by the
// config-file seeder to stay idempotent.
func (s *Store) InstanceByLabel(ctx context.Context, label string) (*Instance, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+instanceCols+` FROM instances WHERE label=?`, label)
	i, err := s.scanInstance(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return i, err
}

func (s *Store) CreateInstance(ctx context.Context, i *Instance) error {
	tok, err := s.box.Encrypt(i.APIToken)
	if err != nil {
		return err
	}
	now := nowStr()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO instances(label,base_url,api_token,created_at,updated_at) VALUES(?,?,?,?,?)`,
		i.Label, normalizeBase(i.BaseURL), tok, now, now)
	if err != nil {
		return err
	}
	i.ID, _ = res.LastInsertId()
	return nil
}

// UpdateInstance updates label/base_url, and the token only when a new non-empty
// one is supplied (so the UI can edit an instance without re-sending the secret).
func (s *Store) UpdateInstance(ctx context.Context, i *Instance) error {
	if i.APIToken != "" {
		tok, err := s.box.Encrypt(i.APIToken)
		if err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx,
			`UPDATE instances SET label=?,base_url=?,api_token=?,updated_at=? WHERE id=?`,
			i.Label, normalizeBase(i.BaseURL), tok, nowStr(), i.ID)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE instances SET label=?,base_url=?,updated_at=? WHERE id=?`,
		i.Label, normalizeBase(i.BaseURL), nowStr(), i.ID)
	return err
}

func (s *Store) DeleteInstance(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM instances WHERE id=?`, id)
	return err
}

// normalizeBase trims a trailing slash and defaults a missing scheme to http://
// so an entry like "edp-1.internal:8080" still resolves.
func normalizeBase(u string) string {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	if u != "" && !strings.Contains(u, "://") {
		u = "http://" + u
	}
	return u
}
