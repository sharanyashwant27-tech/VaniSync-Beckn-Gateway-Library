package vanisync_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yashwant/vanisync-beckn/internal/beckn"
	"github.com/yashwant/vanisync-beckn/internal/sync"
	"github.com/yashwant/vanisync-beckn/pkg/vanisync"
)

func TestConfirmRetailOrderWritesPendingOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &beckn.MockRelayClient{}
	client, err := vanisync.New(vanisync.Options{
		DBPath: filepath.Join(t.TempDir(), "client.db"),
		Relay:  mock,
		Probe:  sync.StaticProbe{Active: false},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	order, err := client.ConfirmRetailOrder(ctx, vanisync.ConfirmOrderRequest{
		ProviderID: "provider-1",
		ItemID:     "item-1",
		Quantity:   2,
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if order.Status != "PENDING" {
		t.Fatalf("status = %q", order.Status)
	}
	if order.ID == "" {
		t.Fatal("expected generated order id")
	}
}

func TestConfirmOrderFromVoiceUsesStubASR(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client, err := vanisync.New(vanisync.Options{
		DBPath: filepath.Join(t.TempDir(), "voice.db"),
		Probe:  sync.StaticProbe{Active: false},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	order, err := client.ConfirmOrderFromVoice(ctx, []byte("audio"), vanisync.ConfirmOrderRequest{
		ProviderID: "p", ItemID: "i", Quantity: 1,
	})
	if err != nil {
		t.Fatalf("confirm from voice: %v", err)
	}
	if order.Status != "PENDING" {
		t.Fatalf("status = %q", order.Status)
	}
}
