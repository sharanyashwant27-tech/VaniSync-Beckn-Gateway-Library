package sync

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/beckn"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/crypto"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
)

const defaultInFlightTimeout = 5 * time.Minute

// NetworkProbe reports whether outbound relay is allowed.
type NetworkProbe interface {
	IsActive(ctx context.Context) bool
}

// StaticProbe returns a fixed network availability flag.
type StaticProbe struct {
	Active bool
}

// IsActive implements NetworkProbe.
func (p StaticProbe) IsActive(_ context.Context) bool {
	return p.Active
}

// Engine drains the outbox FIFO when the network is available.
type Engine struct {
	store            *store.Store
	relay            beckn.RelayClient
	probe            NetworkProbe
	keys             *crypto.SimpleKeyManager
	pollInterval     time.Duration
	baseBackoff      time.Duration
	inFlightTimeout  time.Duration
	logger           *slog.Logger
	mu               sync.Mutex
}

// Config configures the sync engine.
type Config struct {
	Store            *store.Store
	Relay            beckn.RelayClient
	Probe            NetworkProbe
	Keys             *crypto.SimpleKeyManager
	PollInterval     time.Duration
	BaseBackoff      time.Duration
	InFlightTimeout  time.Duration
	Logger           *slog.Logger
}

// NewEngine creates a background sync engine.
func NewEngine(cfg Config) *Engine {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = time.Second
	}
	if cfg.InFlightTimeout <= 0 {
		cfg.InFlightTimeout = defaultInFlightTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Probe == nil {
		cfg.Probe = StaticProbe{Active: true}
	}
	return &Engine{
		store:           cfg.Store,
		relay:           cfg.Relay,
		probe:           cfg.Probe,
		keys:            cfg.Keys,
		pollInterval:    cfg.PollInterval,
		baseBackoff:     cfg.BaseBackoff,
		inFlightTimeout: cfg.InFlightTimeout,
		logger:          cfg.Logger,
	}
}

// Run processes the outbox until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	if err := e.store.ReclaimInFlight(ctx); err != nil {
		return fmt.Errorf("sync: reclaim in-flight on start: %w", err)
	}

	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := e.tick(ctx); err != nil {
				e.logger.Warn("sync tick failed", "err", err)
			}
		}
	}
}

func (e *Engine) tick(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	staleBefore := store.NowMillis() - e.inFlightTimeout.Milliseconds()
	if _, err := e.store.ReclaimStaleInFlight(ctx, staleBefore); err != nil {
		return err
	}

	if !e.probe.IsActive(ctx) {
		return nil
	}

	n, err := e.store.CountInFlight(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	item, err := e.store.DequeuePending(ctx)
	if err != nil {
		return err
	}
	if item == nil {
		return nil
	}

	if err := e.store.MarkQueueInFlight(ctx, item.ID); err != nil {
		return err
	}

	pubKey := ""
	if e.keys != nil {
		pubKey = e.keys.PublicKeyBase64()
	}

	relayErr := e.relay.Relay(ctx, beckn.RelayRequest{
		IdempotencyKey: item.ID,
		Payload:        []byte(item.PayloadJSON),
		Signature:      item.Signature,
		PublicKeyB64:   pubKey,
	})
	if relayErr != nil {
		e.logger.Warn("relay failed", "queue_id", item.ID, "attempt", item.AttemptCount+1, "err", relayErr)
		if err := e.store.IncrementAttempt(ctx, item.ID); err != nil {
			return err
		}
		backoff := e.backoffDuration(item.AttemptCount + 1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		return nil
	}

	return e.store.MarkSent(ctx, item.ID, item.AggregateID)
}

func (e *Engine) backoffDuration(attempt int) time.Duration {
	if attempt <= 0 {
		return e.baseBackoff
	}
	d := e.baseBackoff
	for i := 1; i < attempt; i++ {
		d *= 2
		if d > 30*time.Second {
			return 30 * time.Second
		}
	}
	return d
}

// ProcessOnce runs a single sync iteration (useful in tests).
func (e *Engine) ProcessOnce(ctx context.Context) error {
	return e.tick(ctx)
}
