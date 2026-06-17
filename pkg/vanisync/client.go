package vanisync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/beckn"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/crypto"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/outbox"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/sync"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/voice"
)

// Client is the public VaniSync Beckn gateway SDK entry point.
type Client struct {
	store    *store.Store
	outbox   *outbox.Writer
	engine   *sync.Engine
	asr      voice.ASRProvider
	logger   *slog.Logger
}

// New creates a Client from Options.
func New(opts Options) (*Client, error) {
	if opts.DBPath == "" {
		opts.DBPath = "data/vanisync.db"
	}
	if opts.KeyPath == "" {
		opts.KeyPath = "data/ed25519.key"
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ASR == nil {
		opts.ASR = voice.StubASRProvider{}
	}

	ctx := context.Background()
	st, err := store.Open(ctx, opts.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	keys := opts.KeyManager
	if keys == nil {
		keys, err = crypto.LoadOrCreateSimpleKeyManager(opts.KeyPath)
		if err != nil {
			_ = st.Close()
			return nil, fmt.Errorf("load key manager: %w", err)
		}
	}

	var relay beckn.RelayClient
	if opts.RelayEndpoint != "" {
		relay = beckn.NewHTTPRelayClient(opts.RelayEndpoint, opts.HTTPClient)
	} else if opts.Relay != nil {
		relay = opts.Relay
	} else {
		relay = &beckn.MockRelayClient{}
	}

	probe := opts.Probe
	if probe == nil {
		probe = sync.StaticProbe{Active: true}
	}

	engine := sync.NewEngine(sync.Config{
		Store:        st,
		Relay:        relay,
		Probe:        probe,
		Keys:         keys,
		PollInterval: opts.PollInterval,
		BaseBackoff:  opts.BaseBackoff,
		Logger:       opts.Logger,
	})

	return &Client{
		store:  st,
		outbox: outbox.NewWriter(st, keys),
		engine: engine,
		asr:    opts.ASR,
		logger: opts.Logger,
	}, nil
}

// Close releases database resources.
func (c *Client) Close() error {
	return c.store.Close()
}

// Start runs the background sync engine until ctx is cancelled.
func (c *Client) Start(ctx context.Context) error {
	c.logger.Info("starting sync engine")
	return c.engine.Run(ctx)
}

// ConfirmRetailOrder writes a retail confirm order and outbox row atomically.
func (c *Client) ConfirmRetailOrder(ctx context.Context, req ConfirmOrderRequest) (*LocalOrder, error) {
	orderID := req.OrderID
	if orderID == "" {
		orderID = uuid.NewString()
	}

	now := store.NowMillis()
	becknPayload, err := beckn.BuildRetailConfirmPayload(beckn.ConfirmOrderRequest{
		OrderID:     orderID,
		ProviderID:  req.ProviderID,
		ItemID:      req.ItemID,
		Quantity:    req.Quantity,
		UpdatedAtMs: now,
	})
	if err != nil {
		return nil, err
	}

	order := store.LocalOrder{
		ID:          orderID,
		BecknAction: beckn.ActionConfirm,
		PayloadJSON: string(becknPayload),
		Status:      store.OrderStatusPending,
		UpdatedAt:   now,
		CreatedAt:   now,
	}

	if _, err := c.outbox.WriteOrderWithOutbox(ctx, order, becknPayload); err != nil {
		return nil, err
	}

	return fromStoreOrder(&order), nil
}

// ConfirmOrderFromVoice transcribes audio then confirms a retail order locally.
func (c *Client) ConfirmOrderFromVoice(ctx context.Context, audio []byte, req ConfirmOrderRequest) (*LocalOrder, error) {
	transcript, err := c.asr.Transcribe(ctx, audio)
	if err != nil {
		return nil, fmt.Errorf("transcribe voice: %w", err)
	}
	c.logger.Info("voice transcript", "text", transcript.Text, "lang", transcript.Language)
	return c.ConfirmRetailOrder(ctx, req)
}

// GetOrder loads a local order by ID.
func (c *Client) GetOrder(ctx context.Context, id string) (*LocalOrder, error) {
	o, err := c.store.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	return fromStoreOrder(o), nil
}

// Engine exposes the sync engine for tests.
func (c *Client) Engine() *sync.Engine {
	return c.engine
}

// Store exposes the underlying store for tests.
func (c *Client) Store() *store.Store {
	return c.store
}

func fromStoreOrder(o *store.LocalOrder) *LocalOrder {
	return &LocalOrder{
		ID:          o.ID,
		BecknAction: o.BecknAction,
		PayloadJSON: o.PayloadJSON,
		Status:      o.Status,
		UpdatedAt:   o.UpdatedAt,
		CreatedAt:   o.CreatedAt,
	}
}
