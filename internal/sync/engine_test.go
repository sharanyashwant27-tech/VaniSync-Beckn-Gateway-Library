package sync_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/beckn"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/crypto"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/outbox"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/sync"
)

func TestEngineEnforcesSingleInFlight(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "single.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	keys, _ := crypto.NewSimpleKeyManager()
	writer := outbox.NewWriter(st, keys)
	mock := &beckn.MockRelayClient{}

	for _, id := range []string{"one", "two"} {
		payload := []byte(`{"order":"` + id + `"}`)
		_, err := writer.WriteOrderWithOutbox(ctx, store.LocalOrder{
			ID: id, BecknAction: "confirm", PayloadJSON: string(payload),
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
		t.Fatalf("first process: %v", err)
	}
	n, err := st.CountInFlight(ctx)
	if err != nil {
		t.Fatalf("count in-flight: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected no in-flight after successful relay, got %d", n)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected one relay per tick, got %d", len(mock.Calls))
	}
}

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

func TestEngineReclaimsInFlightOnStart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "reclaim.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	keys, _ := crypto.NewSimpleKeyManager()
	writer := outbox.NewWriter(st, keys)
	payload := []byte(`{"order":"stale"}`)
	outboxID, err := writer.WriteOrderWithOutbox(ctx, store.LocalOrder{
		ID: "stale", BecknAction: "confirm", PayloadJSON: string(payload),
	}, payload)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := st.MarkQueueInFlight(ctx, outboxID); err != nil {
		t.Fatalf("mark in-flight: %v", err)
	}

	mock := &beckn.MockRelayClient{}
	engine := sync.NewEngine(sync.Config{
		Store: st,
		Relay: mock,
		Probe: sync.StaticProbe{Active: true},
		Keys:  keys,
	})

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- engine.Run(runCtx)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(mock.Calls) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	<-done

	if len(mock.Calls) != 1 {
		t.Fatalf("expected relay after reclaim, got %d calls", len(mock.Calls))
	}
}

func TestEngineMarksFailedOnGateway4xx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "4xx.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	keys, _ := crypto.NewSimpleKeyManager()
	writer := outbox.NewWriter(st, keys)
	payload := []byte(`{"order":"bad"}`)
	_, err = writer.WriteOrderWithOutbox(ctx, store.LocalOrder{
		ID: "bad", BecknAction: "confirm", PayloadJSON: string(payload),
	}, payload)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	relay := &beckn.MockRelayClient{Err: &beckn.RelayHTTPError{StatusCode: 400}}
	engine := sync.NewEngine(sync.Config{
		Store: st,
		Relay: relay,
		Probe: sync.StaticProbe{Active: true},
		Keys:  keys,
	})

	if err := engine.ProcessOnce(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	order, err := st.GetOrder(ctx, "bad")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.Status != store.OrderStatusFailed {
		t.Fatalf("order status = %q, want FAILED", order.Status)
	}
}

func TestEngineMarksFailedAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "max.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	keys, _ := crypto.NewSimpleKeyManager()
	writer := outbox.NewWriter(st, keys)
	payload := []byte(`{"order":"retry"}`)
	_, err = writer.WriteOrderWithOutbox(ctx, store.LocalOrder{
		ID: "retry", BecknAction: "confirm", PayloadJSON: string(payload),
	}, payload)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	relay := &beckn.MockRelayClient{Err: errors.New("network down")}
	engine := sync.NewEngine(sync.Config{
		Store:       st,
		Relay:       relay,
		Probe:       sync.StaticProbe{Active: true},
		Keys:        keys,
		MaxAttempts: 1,
		BaseBackoff: time.Millisecond,
	})

	if err := engine.ProcessOnce(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	order, err := st.GetOrder(ctx, "retry")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.Status != store.OrderStatusFailed {
		t.Fatalf("order status = %q, want FAILED", order.Status)
	}
}
