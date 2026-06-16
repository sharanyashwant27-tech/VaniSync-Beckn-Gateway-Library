package store

import (
	"context"
	"database/sql"
	"fmt"
)

// InsertSyncQueueItem inserts an outbox row within an existing transaction.
func InsertSyncQueueItem(ctx context.Context, tx *sql.Tx, item SyncQueueItem) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sync_queue (id, aggregate_id, payload_json, signature, status, attempt_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.AggregateID, item.PayloadJSON, item.Signature, item.Status, item.AttemptCount, item.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert sync_queue: %w", err)
	}
	return nil
}

// DequeuePending returns the oldest pending outbox item, or nil if the queue is empty.
func (s *Store) DequeuePending(ctx context.Context) (*SyncQueueItem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, aggregate_id, payload_json, signature, status, attempt_count, created_at
		FROM sync_queue
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT 1`, QueueStatusPending)

	var item SyncQueueItem
	err := row.Scan(&item.ID, &item.AggregateID, &item.PayloadJSON, &item.Signature,
		&item.Status, &item.AttemptCount, &item.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dequeue pending: %w", err)
	}
	return &item, nil
}

// MarkQueueStatus updates sync_queue status for a row.
func (s *Store) MarkQueueStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sync_queue SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("mark queue status: %w", err)
	}
	return nil
}

// IncrementAttempt bumps attempt_count and resets status to PENDING for retry.
func (s *Store) IncrementAttempt(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_queue
		SET attempt_count = attempt_count + 1, status = ?
		WHERE id = ?`, QueueStatusPending, id)
	if err != nil {
		return fmt.Errorf("increment attempt: %w", err)
	}
	return nil
}

// MarkSent marks the outbox row as sent and updates the linked order to SYNCED atomically.
func (s *Store) MarkSent(ctx context.Context, queueID, orderID string) error {
	now := NowMillis()
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := markQueueStatusTx(ctx, tx, queueID, QueueStatusSent); err != nil {
			return err
		}
		return UpdateOrderStatus(ctx, tx, orderID, OrderStatusSynced, now)
	})
}

func markQueueStatusTx(ctx context.Context, tx *sql.Tx, id, status string) error {
	_, err := tx.ExecContext(ctx, `UPDATE sync_queue SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("mark queue status: %w", err)
	}
	return nil
}
