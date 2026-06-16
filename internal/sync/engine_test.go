package sync_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yashwant/vanisync-beckn/internal/beckn"
	"github.com/yashwant/vanisync-beckn/internal/crypto"
	"github.com/yashwant/vanisync-beckn/internal/outbox"
	"github.com/yashwant/vanisync-beckn/internal/store"
	"github.com/yashwant/vanisync-beckn/internal/sync"
)

func TestEngineRelaysFIFOWhenNetworkUp(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	keys, _ := crypto.NewSimpleKeyManager()
	writer := outbox.NewWriter(st, keys)
	mock := &beckn.MockRelayClient{}

	for _, id := range []string{"a", "b"} {
		payload := []byte(`{"order":"` + id + `"}`)
		_, err := writer.WriteOrderWithOutbox(ctx, store.LocalOrder{
			ID:          id,
			BecknAction: "confirm",
			PayloadJSON: string(payload),
		}, payload)
		if err != nil {
			t.Fatalf("write %s: %v", id, err)
		}
	}

	engine := sync.NewEngine(sync.Config{
		Store: st,
		Relay: mock,
		Probe: sync.StaticProbe{Active: true},
		Keys:  keys,
	})

	if err := engine.ProcessOnce(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].IdempotencyKey == "" {
		t.Fatalf("expected one relay, got %+v", mock.Calls)
	}

	order, err := st.GetOrder(ctx, "a")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.Status != store.OrderStatusSynced {
		t.Fatalf("order a status = %q", order.Status)
	}
}

func TestEngineSkipsWhenNetworkDown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "offline.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	keys, _ := crypto.NewSimpleKeyManager()
	writer := outbox.NewWriter(st, keys)
	mock := &beckn.MockRelayClient{}
	payload := []byte(`{"order":"x"}`)
	_, err = writer.WriteOrderWithOutbox(ctx, store.LocalOrder{
		ID: "x", BecknAction: "confirm", PayloadJSON: string(payload),
	}, payload)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	engine := sync.NewEngine(sync.Config{
		Store: st,
		Relay: mock,
		Probe: sync.StaticProbe{Active: false},
		Keys:  keys,
	})

	if err := engine.ProcessOnce(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Fatal("expected no relay when network down")
	}
}
