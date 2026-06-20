// Package store is the SQLite persistence layer for edp. It uses the pure-Go
// modernc.org/sqlite driver so the final binary needs no CGO.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"edp/internal/secret"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	db  *sql.DB
	box *secret.Box
}

// Open opens (creating if needed) the SQLite database under dataDir and applies
// the schema. WAL mode and foreign keys are enabled. It also loads the
// encryption key used to protect credentials at rest.
func Open(dataDir string) (*Store, error) {
	dsn := filepath.Join(dataDir, "edp.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite single-writer; keep it simple and serialized.
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	box, err := secret.Load(dataDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("load secret key: %w", err)
	}
	return &Store{db: db, box: box}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// migrate adds columns introduced after the initial schema. ADD COLUMN is
// idempotent here: on a fresh DB the columns already exist (from schema.sql) and
// the "duplicate column" error is ignored; on an older DB they get added.
func migrate(db *sql.DB) error {
	alters := []string{
		`ALTER TABLE environments ADD COLUMN health_type TEXT NOT NULL DEFAULT 'none'`,
		`ALTER TABLE environments ADD COLUMN health_target TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE deployments ADD COLUMN reason TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE deployments ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE deployments ADD COLUMN ready_ms INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE environments ADD COLUMN proxy_host TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN proxy_port TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN listen_port TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN clear_site_data TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN entrypoint TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN command TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN dockerfile_content TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN compose_override TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN entrypoint_script TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE environments ADD COLUMN auto_deploy INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE environments ADD COLUMN splash_url TEXT NOT NULL DEFAULT ''`,
	}
	for _, a := range alters {
		if _, err := db.Exec(a); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("%s: %w", a, err)
		}
	}
	return nil
}

func nowStr() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// ResetInterrupted marks deploys/hook-runs that were in flight when edp stopped
// as failed, and clears the matching env/hook "running" status. Without this, a
// crash mid-deploy would leave an env stuck "running" forever (the proxy would
// show the splash and the scheduler would skip it).
func (s *Store) ResetInterrupted(ctx context.Context) error {
	stmts := []string{
		`UPDATE deployments SET status='failed', finished_at=? WHERE status IN ('running','queued')`,
		`UPDATE hook_runs   SET status='failed', finished_at=? WHERE status IN ('running','queued')`,
		`UPDATE environments SET status='idle' WHERE status='running'`,
		`UPDATE timed_hooks  SET status='idle' WHERE status='running'`,
	}
	now := nowStr()
	for _, q := range stmts {
		arg := []any{}
		if strings.Contains(q, "finished_at=?") {
			arg = append(arg, now)
		}
		if _, err := s.db.ExecContext(ctx, q, arg...); err != nil {
			return err
		}
	}
	return nil
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func parseTimePtr(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t := parseTime(s.String)
	return &t
}

// ---- settings ----

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings(key,value) VALUES(?,?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// InstanceID returns this edp instance's stable identifier, generating and
// persisting one (in /data, which survives container recreation) on first call.
// It is stamped as the edp.instance label on every container edp creates so a
// teardown (edp reap) removes only the containers this instance owns — important
// when several edp instances share a host.
func (s *Store) InstanceID(ctx context.Context) (string, error) {
	if v, ok, err := s.GetSetting(ctx, "instance_id"); err != nil {
		return "", err
	} else if ok && v != "" {
		return v, nil
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	if err := s.SetSetting(ctx, "instance_id", id); err != nil {
		return "", err
	}
	return id, nil
}
