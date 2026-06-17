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

// CountInFlight returns the number of outbox rows currently marked IN_FLIGHT.
func (s *Store) CountInFlight(ctx context.Context) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_queue WHERE status = ?`, QueueStatusInFlight)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, fmt.Errorf("count in-flight: %w", err)
	}
	return n, nil
}

// ReclaimInFlight resets all IN_FLIGHT rows to PENDING (crash recovery on engine start).
func (s *Store) ReclaimInFlight(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_queue
		SET status = ?, in_flight_at = NULL
		WHERE status = ?`, QueueStatusPending, QueueStatusInFlight)
	if err != nil {
		return fmt.Errorf("reclaim in-flight: %w", err)
	}
	return nil
}

// ReclaimStaleInFlight resets IN_FLIGHT rows older than staleBeforeMs to PENDING.
func (s *Store) ReclaimStaleInFlight(ctx context.Context, staleBeforeMs int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sync_queue
		SET status = ?, in_flight_at = NULL
		WHERE status = ? AND in_flight_at IS NOT NULL AND in_flight_at < ?`,
		QueueStatusPending, QueueStatusInFlight, staleBeforeMs)
	if err != nil {
		return 0, fmt.Errorf("reclaim stale in-flight: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("reclaim stale in-flight rows affected: %w", err)
	}
	return n, nil
}

// MarkQueueInFlight marks a pending row IN_FLIGHT when no other row is in flight.
func (s *Store) MarkQueueInFlight(ctx context.Context, id string) error {
	now := NowMillis()
	res, err := s.db.ExecContext(ctx, `
		UPDATE sync_queue
		SET status = ?, in_flight_at = ?
		WHERE id = ? AND status = ?
		AND NOT EXISTS (SELECT 1 FROM sync_queue WHERE status = ? AND id != ?)`,
		QueueStatusInFlight, now, id, QueueStatusPending, QueueStatusInFlight, id)
	if err != nil {
		return fmt.Errorf("mark in-flight: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark in-flight rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("mark in-flight: row %q unavailable or another in-flight exists", id)
	}
	return nil
}

// IncrementAttempt bumps attempt_count and resets status to PENDING for retry.
func (s *Store) IncrementAttempt(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_queue
		SET attempt_count = attempt_count + 1, status = ?, in_flight_at = NULL
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
	_, err := tx.ExecContext(ctx, `
		UPDATE sync_queue SET status = ?, in_flight_at = NULL WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("mark queue status: %w", err)
	}
	return nil
}
