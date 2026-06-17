package refinement_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"testing/quick"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/beckn"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/crypto"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/outbox"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/sync"
)

type toggleProbe struct {
	active bool
}

func (p *toggleProbe) IsActive(_ context.Context) bool {
	return p.active
}

type serverRelay struct {
	serverDB map[string]struct{}
	fail     bool
}

func (r *serverRelay) Relay(_ context.Context, req beckn.RelayRequest) error {
	if r.fail {
		return errors.New("relay failed")
	}
	var payload struct {
		Message struct {
			Order struct {
				ID string `json:"id"`
			} `json:"order"`
		} `json:"message"`
	}
	if err := json.Unmarshal(req.Payload, &payload); err == nil && payload.Message.Order.ID != "" {
		r.serverDB[payload.Message.Order.ID] = struct{}{}
	}
	return nil
}

type realSystem struct {
	ctx    context.Context
	store  *store.Store
	writer *outbox.Writer
	engine *sync.Engine
	probe  *toggleProbe
	relay  *serverRelay
}

func newRealSystem(t *testing.T) *realSystem {
	t.Helper()
	sys, err := setupRealSystem(t.TempDir())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { _ = sys.store.Close() })
	return sys
}

func setupRealSystem(dir string) (*realSystem, error) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(dir, "refinement.db"))
	if err != nil {
		return nil, err
	}

	keys, err := crypto.NewSimpleKeyManager()
	if err != nil {
		_ = st.Close()
		return nil, err
	}

	probe := &toggleProbe{active: true}
	relay := &serverRelay{serverDB: make(map[string]struct{})}

	engine := sync.NewEngine(sync.Config{
		Store: st,
		Relay: relay,
		Probe: probe,
		Keys:  keys,
	})

	return &realSystem{
		ctx:    ctx,
		store:  st,
		writer: outbox.NewWriter(st, keys),
		engine: engine,
		probe:  probe,
		relay:  relay,
	}, nil
}

func (s *realSystem) localWrite(orderID string) error {
	if orderID == "" {
		orderID = randomID()
	}
	payload, err := beckn.BuildRetailConfirmPayload(beckn.ConfirmOrderRequest{
		OrderID:    orderID,
		ProviderID: "provider",
		ItemID:     "item",
		Quantity:   1,
	})
	if err != nil {
		return err
	}
	_, err = s.writer.WriteOrderWithOutbox(s.ctx, store.LocalOrder{
		ID:          orderID,
		BecknAction: beckn.ActionConfirm,
		PayloadJSON: string(payload),
	}, payload)
	return err
}

func (s *realSystem) relayOutbox() error {
	return s.engine.ProcessOnce(s.ctx)
}

func (s *realSystem) networkDrop() {
	s.probe.active = false
	s.relay.fail = true
}

func (s *realSystem) networkRestore() {
	s.probe.active = true
	s.relay.fail = false
}

func (s *realSystem) checkNoOrphans() bool {
	for orderID := range s.relay.serverDB {
		if _, err := s.store.GetOrder(s.ctx, orderID); err != nil {
			return false
		}
	}
	return true
}

func TestRealStoreSyncNoOrphansQuick(t *testing.T) {
	f := func(seed int64, steps int) bool {
		rand.Seed(seed)
		if steps < 0 {
			steps = -steps
		}
		steps = steps%20 + 1

		dir, err := os.MkdirTemp("", "refinement-quick-*")
		if err != nil {
			return false
		}
		defer os.RemoveAll(dir)

		sys, err := setupRealSystem(dir)
		if err != nil {
			return false
		}
		defer sys.store.Close()

		actions := []func() error{
			func() error { return sys.localWrite(randomID()) },
			func() error { return sys.relayOutbox() },
			func() error { sys.networkDrop(); return nil },
			func() error { sys.networkRestore(); return nil },
		}

		for i := 0; i < steps; i++ {
			if err := actions[rand.Intn(len(actions))](); err != nil {
				return false
			}
		}
		return sys.checkNoOrphans()
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

func TestRealStoreOfflineThenSync(t *testing.T) {
	sys := newRealSystem(t)

	sys.networkDrop()
	if err := sys.localWrite("order-offline"); err != nil {
		t.Fatalf("local write: %v", err)
	}
	if err := sys.relayOutbox(); err != nil {
		t.Fatalf("relay while offline: %v", err)
	}
	if len(sys.relay.serverDB) != 0 {
		t.Fatal("server should be empty while offline")
	}

	sys.networkRestore()
	if err := sys.relayOutbox(); err != nil {
		t.Fatalf("relay after restore: %v", err)
	}
	if _, ok := sys.relay.serverDB["order-offline"]; !ok {
		t.Fatal("expected order on server after sync")
	}
	if !sys.checkNoOrphans() {
		t.Fatal("orphan detected")
	}

	order, err := sys.store.GetOrder(sys.ctx, "order-offline")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.Status != store.OrderStatusSynced {
		t.Fatalf("order status = %q", order.Status)
	}
}

func TestRealStoreFIFORelayOrder(t *testing.T) {
	sys := newRealSystem(t)

	for _, id := range []string{"first", "second"} {
		if err := sys.localWrite(id); err != nil {
			t.Fatalf("write %s: %v", id, err)
		}
	}

	if err := sys.relayOutbox(); err != nil {
		t.Fatalf("relay first: %v", err)
	}
	if _, ok := sys.relay.serverDB["first"]; !ok {
		t.Fatal("expected first order relayed")
	}
	if _, ok := sys.relay.serverDB["second"]; ok {
		t.Fatal("second order should not relay before first completes")
	}

	if err := sys.relayOutbox(); err != nil {
		t.Fatalf("relay second: %v", err)
	}
	if _, ok := sys.relay.serverDB["second"]; !ok {
		t.Fatal("expected second order relayed")
	}
}
