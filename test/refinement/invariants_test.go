package refinement_test

import (
	"math/rand"
	"testing"
	"testing/quick"
)

// Order mirrors TLA+ clientDB entries.
type Order struct {
	ID string
}

// OutboxEvent mirrors TLA+ outbox sequence elements.
type OutboxEvent struct {
	ID          string
	AggregateID string
}

// SystemState mirrors the TLA+ state variables for refinement testing.
type SystemState struct {
	ClientDB      map[string]*Order
	Outbox        []*OutboxEvent
	ServerDB      map[string]*Order
	NetworkActive bool
}

func newSystemState() *SystemState {
	return &SystemState{
		ClientDB:      make(map[string]*Order),
		Outbox:        nil,
		ServerDB:      make(map[string]*Order),
		NetworkActive: true,
	}
}

// CheckNoOrphanInvariants enforces TLA+ safety: every server record exists in clientDB.
func CheckNoOrphanInvariants(s *SystemState) bool {
	for id := range s.ServerDB {
		if _, ok := s.ClientDB[id]; !ok {
			return false
		}
	}
	return true
}

// LocalWrite adds an order and matching outbox event (atomic in Go implementation).
func LocalWrite(s *SystemState, orderID string) {
	if orderID == "" {
		orderID = randomID()
	}
	s.ClientDB[orderID] = &Order{ID: orderID}
	s.Outbox = append(s.Outbox, &OutboxEvent{
		ID:          randomID(),
		AggregateID: orderID,
	})
}

// RelayOutbox moves the oldest pending outbox event to the server when network is up.
func RelayOutbox(s *SystemState) {
	if !s.NetworkActive || len(s.Outbox) == 0 {
		return
	}
	ev := s.Outbox[0]
	s.Outbox = s.Outbox[1:]
	s.ServerDB[ev.AggregateID] = &Order{ID: ev.AggregateID}
}

// NetworkDrop disables relay.
func NetworkDrop(s *SystemState) {
	s.NetworkActive = false
}

// NetworkRestore re-enables relay.
func NetworkRestore(s *SystemState) {
	s.NetworkActive = true
}

// ServerProcess is a no-op placeholder for gateway-side processing in the model.
func ServerProcess(s *SystemState) {
	_ = s
}

func randomID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type stepFn func(*SystemState)

var steps = []stepFn{
	func(s *SystemState) { LocalWrite(s, randomID()) },
	func(s *SystemState) { RelayOutbox(s) },
	func(s *SystemState) { NetworkDrop(s) },
	func(s *SystemState) { NetworkRestore(s) },
	func(s *SystemState) { ServerProcess(s) },
}

func applyRandomSequence(s *SystemState, n int) {
	for i := 0; i < n; i++ {
		steps[rand.Intn(len(steps))](s)
	}
}

func TestCheckNoOrphanInvariantsQuick(t *testing.T) {
	f := func(seed int64, steps int) bool {
		rand.Seed(seed)
		if steps < 0 {
			steps = -steps
		}
		steps = steps % 50
		s := newSystemState()
		applyRandomSequence(s, steps+1)
		return CheckNoOrphanInvariants(s)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
}

func TestLocalWriteThenRelayPreservesInvariant(t *testing.T) {
	s := newSystemState()
	for i := 0; i < 20; i++ {
		LocalWrite(s, randomID())
		RelayOutbox(s)
		if !CheckNoOrphanInvariants(s) {
			t.Fatalf("orphan detected after step %d", i)
		}
	}
}

func TestNetworkDropDoesNotCreateOrphans(t *testing.T) {
	s := newSystemState()
	LocalWrite(s, "order-1")
	NetworkDrop(s)
	RelayOutbox(s)
	if len(s.ServerDB) != 0 {
		t.Fatal("relay should not succeed when network down")
	}
	if !CheckNoOrphanInvariants(s) {
		t.Fatal("network drop introduced orphan")
	}
}
