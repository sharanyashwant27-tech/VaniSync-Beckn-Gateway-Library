package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
)

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

func TestReclaimNullInFlightAtResetsOrphans(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "null.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := store.NowMillis()
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO sync_queue (id, aggregate_id, payload_json, signature, status, attempt_count, created_at, in_flight_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		"q-null", "order-null", "{}", "sig", store.QueueStatusInFlight, 0, now)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	n, err := st.ReclaimNullInFlightAt(ctx)
	if err != nil || n != 1 {
		t.Fatalf("reclaim null in_flight_at = %d, err = %v", n, err)
	}

	inFlight, err := st.CountInFlight(ctx)
	if err != nil || inFlight != 0 {
		t.Fatalf("count in-flight = %d, err = %v", inFlight, err)
	}
}

func TestMarkQueueStatusClearsInFlightAt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "status.db"))
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

	if err := st.MarkQueueStatus(ctx, "q1", store.QueueStatusPending); err != nil {
		t.Fatalf("mark status: %v", err)
	}

	var inFlightAt sql.NullInt64
	if err := st.DB().QueryRowContext(ctx, `SELECT in_flight_at FROM sync_queue WHERE id = ?`, "q1").Scan(&inFlightAt); err != nil {
		t.Fatalf("query in_flight_at: %v", err)
	}
	if inFlightAt.Valid {
		t.Fatalf("expected in_flight_at cleared, got %v", inFlightAt.Int64)
	}
}

func TestMarkFailedUpdatesOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "failed.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := store.NowMillis()
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO local_orders (id, beckn_action, payload_json, status, updated_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"order-1", "confirm", "{}", store.OrderStatusPending, now, now)
	if err != nil {
		t.Fatalf("insert order: %v", err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO sync_queue (id, aggregate_id, payload_json, signature, status, attempt_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"q1", "order-1", "{}", "sig", store.QueueStatusInFlight, 9, now)
	if err != nil {
		t.Fatalf("insert queue: %v", err)
	}

	if err := st.MarkFailed(ctx, "q1", "order-1"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	order, err := st.GetOrder(ctx, "order-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.Status != store.OrderStatusFailed {
		t.Fatalf("order status = %q", order.Status)
	}
}
