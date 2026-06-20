// Package store is the SQLite persistence layer for edp-manager. Like edp it uses
// the pure-Go modernc.org/sqlite driver so the binary needs no CGO.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"edp-manager/internal/secret"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	db  *sql.DB
	box *secret.Box
}

// Open opens (creating if needed) the SQLite database under dataDir, applies the
// schema, and loads the encryption key used to protect instance tokens at rest.
func Open(dataDir string) (*Store, error) {
	dsn := filepath.Join(dataDir, "edp-manager.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite single-writer; keep it serialized.
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	box, err := secret.Load(dataDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("load secret key: %w", err)
	}
	return &Store{db: db, box: box}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func nowStr() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
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

// GetOrCreateSecret returns the setting at key, generating and persisting a
// random 32-byte hex value on first use (used for the manager's API token).
func (s *Store) GetOrCreateSecret(ctx context.Context, key string) (string, error) {
	if v, ok, err := s.GetSetting(ctx, key); err != nil {
		return "", err
	} else if ok {
		return v, nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	v := hex.EncodeToString(b)
	return v, s.SetSetting(ctx, key, v)
}
