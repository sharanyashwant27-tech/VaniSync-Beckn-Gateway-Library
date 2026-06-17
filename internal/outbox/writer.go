package outbox

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/crypto"
	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/store"
)

// Writer performs atomic domain + outbox writes.
type Writer struct {
	store *store.Store
	keys  *crypto.SimpleKeyManager
}

// NewWriter creates an outbox writer bound to store and key manager.
func NewWriter(s *store.Store, keys *crypto.SimpleKeyManager) *Writer {
	return &Writer{store: s, keys: keys}
}

// WriteOrderWithOutbox inserts local_orders and sync_queue in one transaction.
// The Beckn payload is signed before the outbox row is written.
func (w *Writer) WriteOrderWithOutbox(ctx context.Context, order store.LocalOrder, becknPayload []byte) (string, error) {
	signature, err := w.keys.Sign(becknPayload)
	if err != nil {
		return "", fmt.Errorf("sign beckn payload: %w", err)
	}

	outboxID := uuid.NewString()
	now := store.NowMillis()
	if order.CreatedAt == 0 {
		order.CreatedAt = now
	}
	if order.UpdatedAt == 0 {
		order.UpdatedAt = now
	}
	if order.Status == "" {
		order.Status = store.OrderStatusPending
	}

	item := store.SyncQueueItem{
		ID:           outboxID,
		AggregateID:  order.ID,
		PayloadJSON:  string(becknPayload),
		Signature:    signature,
		Status:       store.QueueStatusPending,
		AttemptCount: 0,
		CreatedAt:    now,
	}

	err = w.store.WithTx(ctx, func(tx *sql.Tx) error {
		if err := store.InsertOrder(ctx, tx, order); err != nil {
			return err
		}
		return store.InsertSyncQueueItem(ctx, tx, item)
	})
	if err != nil {
		return "", err
	}
	return outboxID, nil
}
