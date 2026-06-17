package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var embeddedSchema string

const defaultMigrationsDir = "migrations"

// Store wraps SQLite access and migration helpers.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database at path and runs migrations.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// WAL improves concurrent read/write on embedded SQLite (see ADR-001 outbox durability).
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL journal mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous=NORMAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set synchronous NORMAL: %w", err)
	}

	s := &Store{db: db}
	if err := s.MigrateEmbedded(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.migrateOptionalColumns(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// MigrateEmbedded applies the embedded schema (works regardless of process cwd).
func (s *Store) MigrateEmbedded(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, embeddedSchema); err != nil {
		return fmt.Errorf("apply embedded schema: %w", err)
	}
	return nil
}

// migrateOptionalColumns applies idempotent schema upgrades for existing databases.
func (s *Store) migrateOptionalColumns(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `ALTER TABLE sync_queue ADD COLUMN in_flight_at INTEGER`)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return fmt.Errorf("add in_flight_at column: %w", err)
	}
	return nil
}

// DB returns the underlying database handle.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Migrate applies SQL files from dir in lexical order.
func (s *Store) Migrate(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", path, err)
		}
		if _, err := s.db.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", path, err)
		}
	}
	return nil
}

// WithTx runs fn inside a SQL transaction.
func (s *Store) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback after %w: %v", err, rbErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// NowMillis returns current Unix time in milliseconds.
func NowMillis() int64 {
	return time.Now().UnixMilli()
}
