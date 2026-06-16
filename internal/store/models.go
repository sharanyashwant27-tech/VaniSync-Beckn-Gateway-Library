package store

const (
	OrderStatusPending = "PENDING"
	OrderStatusSynced  = "SYNCED"
	OrderStatusFailed  = "FAILED"

	QueueStatusPending  = "PENDING"
	QueueStatusInFlight = "IN_FLIGHT"
	QueueStatusSent     = "SENT"
	QueueStatusFailed   = "FAILED"
)

// LocalOrder is a domain retail order persisted locally.
type LocalOrder struct {
	ID          string
	BecknAction string
	PayloadJSON string
	Status      string
	UpdatedAt   int64
	CreatedAt   int64
}

// SyncQueueItem is an outbox row awaiting relay to the Beckn gateway.
type SyncQueueItem struct {
	ID           string
	AggregateID  string
	PayloadJSON  string
	Signature    string
	Status       string
	AttemptCount int
	CreatedAt    int64
}
