package outbox_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/crypto"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/outbox"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
)

func TestWriteOrderWithOutboxAtomic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	keys, err := crypto.NewSimpleKeyManager()
	if err != nil {
		t.Fatalf("keys: %v", err)
	}

	writer := outbox.NewWriter(st, keys)
	payload := []byte(`{"context":{"action":"confirm"}}`)
	order := store.LocalOrder{
		ID:          "order-1",
		BecknAction: "confirm",
		PayloadJSON: string(payload),
	}

	outboxID, err := writer.WriteOrderWithOutbox(ctx, order, payload)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if outboxID == "" {
		t.Fatal("expected outbox id")
	}

	got, err := st.GetOrder(ctx, "order-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if got.Status != store.OrderStatusPending {
		t.Fatalf("order status = %q", got.Status)
	}

	item, err := st.DequeuePending(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if item == nil || item.ID != outboxID {
		t.Fatalf("unexpected queue item: %+v", item)
	}
	if item.Signature == "" {
		t.Fatal("expected signature on outbox row")
	}
}
