package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
)

func TestOpenEnablesWAL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "wal.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var mode string
	if err := st.DB().QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if strings.ToLower(mode) != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	migDir := t.TempDir()
	migrationSQL := `ALTER TABLE sync_queue ADD COLUMN in_flight_at INTEGER;`
	if err := os.WriteFile(filepath.Join(migDir, "002_in_flight_at.sql"), []byte(migrationSQL), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "migrate.db")
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	for i := 0; i < 2; i++ {
		if err := st.Migrate(ctx, migDir); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}
}
