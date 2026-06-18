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

// Migrate applies SQL files from dir in lexical order, skipping already-recorded migrations.
func (s *Store) Migrate(ctx context.Context, dir string) error {
	if err := s.ensureSchemaMigrations(ctx); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		name := entry.Name()
		applied, err := s.isMigrationApplied(ctx, name)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		path := filepath.Join(dir, name)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", path, err)
		}
		if _, err := s.db.ExecContext(ctx, string(sqlBytes)); err != nil && !isIgnorableMigrationError(err) {
			return fmt.Errorf("apply migration %s: %w", path, err)
		}
		if err := s.recordMigration(ctx, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) ensureSchemaMigrations(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`)
	return err
}

func (s *Store) isMigrationApplied(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) recordMigration(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`,
		name, NowMillis(),
	)
	return err
}

func isIgnorableMigrationError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column")
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
