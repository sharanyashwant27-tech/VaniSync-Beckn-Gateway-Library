package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
)

func TestReclaimInFlightResetsOrphans(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := store.NowMillis()
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO sync_queue (id, aggregate_id, payload_json, signature, status, attempt_count, created_at, in_flight_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"q1", "order-1", "{}", "sig", store.QueueStatusInFlight, 0, now, now)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	n, err := st.CountInFlight(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count in-flight = %d, err = %v", n, err)
	}

	if err := st.ReclaimInFlight(ctx); err != nil {
		t.Fatalf("reclaim: %v", err)
	}

	n, err = st.CountInFlight(ctx)
	if err != nil || n != 0 {
		t.Fatalf("after reclaim count = %d, err = %v", n, err)
	}
}

func TestMarkQueueInFlightAllowsOnlyOne(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "one.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := store.NowMillis()
	for _, id := range []string{"q1", "q2"} {
		_, err := st.DB().ExecContext(ctx, `
			INSERT INTO sync_queue (id, aggregate_id, payload_json, signature, status, attempt_count, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, "order-"+id, "{}", "sig", store.QueueStatusPending, 0, now)
		if err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}

	if err := st.MarkQueueInFlight(ctx, "q1"); err != nil {
		t.Fatalf("mark q1: %v", err)
	}
	if err := st.MarkQueueInFlight(ctx, "q2"); err == nil {
		t.Fatal("expected second in-flight mark to fail")
	}
}
