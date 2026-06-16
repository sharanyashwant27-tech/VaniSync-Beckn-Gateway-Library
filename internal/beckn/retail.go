package beckn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	ActionConfirm       = "confirm"
	HeaderGatewaySig    = "X-Gateway-Signature"
	HeaderIdempotency   = "X-Idempotency-Key"
	HeaderGatewayPubKey = "X-Gateway-Public-Key"
)

// ConfirmOrderRequest holds structured retail confirm inputs.
type ConfirmOrderRequest struct {
	OrderID     string
	ProviderID  string
	ItemID      string
	Quantity    int
	UpdatedAtMs int64
}

// BuildRetailConfirmPayload builds a Beckn retail confirm JSON body.
func BuildRetailConfirmPayload(req ConfirmOrderRequest) ([]byte, error) {
	if req.OrderID == "" {
		return nil, fmt.Errorf("order id required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	payload := map[string]any{
		"context": map[string]any{
			"action": ActionConfirm,
			"domain": "retail",
			"timestamp": now,
		},
		"message": map[string]any{
			"order": map[string]any{
				"id": req.OrderID,
				"provider": map[string]any{
					"id": req.ProviderID,
				},
				"items": []map[string]any{
					{
						"id":       req.ItemID,
						"quantity": req.Quantity,
					},
				},
				"updated_at": req.UpdatedAtMs,
			},
		},
	}
	return json.Marshal(payload)
}

// RelayRequest is sent to the Beckn gateway.
type RelayRequest struct {
	IdempotencyKey string
	Payload        []byte
	Signature      string
	PublicKeyB64   string
}

// RelayClient relays signed Beckn payloads to a gateway.
type RelayClient interface {
	Relay(ctx context.Context, req RelayRequest) error
}

// HTTPRelayClient posts signed payloads to a Beckn-compatible endpoint.
type HTTPRelayClient struct {
	endpoint   string
	httpClient *http.Client
}

// NewHTTPRelayClient creates a relay client for the given gateway URL.
func NewHTTPRelayClient(endpoint string, httpClient *http.Client) *HTTPRelayClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPRelayClient{endpoint: endpoint, httpClient: httpClient}
}

// Relay POSTs payload with Beckn-style signature headers.
func (c *HTTPRelayClient) Relay(ctx context.Context, req RelayRequest) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(req.Payload))
	if err != nil {
		return fmt.Errorf("create relay request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(HeaderGatewaySig, req.Signature)
	httpReq.Header.Set(HeaderIdempotency, req.IdempotencyKey)
	if req.PublicKeyB64 != "" {
		httpReq.Header.Set(HeaderGatewayPubKey, req.PublicKeyB64)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("relay request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("relay failed: status %d", resp.StatusCode)
	}
	return nil
}

// MockRelayClient records relay calls for tests.
type MockRelayClient struct {
	Calls []RelayRequest
	Err   error
}

// Relay appends the request to Calls and returns Err if set.
func (m *MockRelayClient) Relay(ctx context.Context, req RelayRequest) error {
	m.Calls = append(m.Calls, req)
	return m.Err
}
