package vanisync

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/beckn"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/crypto"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/sync"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/voice"
)

// LocalOrder is the public view of a persisted retail order.
type LocalOrder struct {
	ID          string
	BecknAction string
	PayloadJSON string
	Status      string
	UpdatedAt   int64
	CreatedAt   int64
}

// ConfirmOrderRequest is the structured retail confirm input.
type ConfirmOrderRequest struct {
	OrderID    string
	ProviderID string
	ItemID     string
	Quantity   int
}

// Options configures a VaniSync client.
type Options struct {
	DBPath        string
	KeyPath       string
	RelayEndpoint string
	Relay         beckn.RelayClient
	Probe         sync.NetworkProbe
	KeyManager    *crypto.SimpleKeyManager
	ASR           voice.ASRProvider
	HTTPClient    *http.Client
	PollInterval  time.Duration
	BaseBackoff   time.Duration
	Logger        *slog.Logger
}
