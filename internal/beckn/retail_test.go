package beckn_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/beckn"
)

func TestBuildRetailConfirmPayload(t *testing.T) {
	t.Parallel()

	updatedAtMs := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC).UnixMilli()
	payload, err := beckn.BuildRetailConfirmPayload(beckn.ConfirmOrderRequest{
		OrderID: "o1", ProviderID: "p1", ItemID: "i1", Quantity: 3, UpdatedAtMs: updatedAtMs,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ctx, ok := doc["context"].(map[string]any)
	if !ok || ctx["action"] != "confirm" {
		t.Fatalf("unexpected payload: %s", payload)
	}
	wantTS := time.UnixMilli(updatedAtMs).UTC().Format(time.RFC3339)
	if ctx["timestamp"] != wantTS {
		t.Fatalf("timestamp = %v, want %q", ctx["timestamp"], wantTS)
	}
}

func TestHTTPRelayClientSetsSignatureHeader(t *testing.T) {
	t.Parallel()

	var gotSig, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get(beckn.HeaderGatewaySig)
		gotKey = r.Header.Get(beckn.HeaderIdempotency)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := beckn.NewHTTPRelayClient(srv.URL, srv.Client())
	err := client.Relay(context.Background(), beckn.RelayRequest{
		IdempotencyKey: "idem-1",
		Payload:        []byte(`{}`),
		Signature:      "sig-abc",
	})
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	if gotSig != "sig-abc" || gotKey != "idem-1" {
		t.Fatalf("headers sig=%q key=%q", gotSig, gotKey)
	}
}

func TestHTTPRelayClientReturnsClientErrorFor4xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	client := beckn.NewHTTPRelayClient(srv.URL, srv.Client())
	err := client.Relay(context.Background(), beckn.RelayRequest{
		IdempotencyKey: "idem-1",
		Payload:        []byte(`{}`),
		Signature:      "sig",
	})
	if !beckn.IsRelayClientError(err) {
		t.Fatalf("expected client error, got %v", err)
	}
	var he *beckn.RelayHTTPError
	if !errors.As(err, &he) || he.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPRelayClientReturnsServerErrorFor5xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	client := beckn.NewHTTPRelayClient(srv.URL, srv.Client())
	err := client.Relay(context.Background(), beckn.RelayRequest{
		IdempotencyKey: "idem-1",
		Payload:        []byte(`{}`),
		Signature:      "sig",
	})
	if beckn.IsRelayClientError(err) {
		t.Fatalf("expected server error, got client error: %v", err)
	}
	var he *beckn.RelayHTTPError
	if !errors.As(err, &he) || he.StatusCode != http.StatusBadGateway {
		t.Fatalf("unexpected error: %v", err)
	}
}
