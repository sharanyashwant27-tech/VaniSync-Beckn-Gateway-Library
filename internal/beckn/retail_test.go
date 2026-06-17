package beckn_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/beckn"
)

func TestBuildRetailConfirmPayload(t *testing.T) {
	t.Parallel()

	payload, err := beckn.BuildRetailConfirmPayload(beckn.ConfirmOrderRequest{
		OrderID: "o1", ProviderID: "p1", ItemID: "i1", Quantity: 3,
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
