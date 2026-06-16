package store

import (
	"context"
	"database/sql"
	"fmt"
)

// InsertOrder inserts a local order within an existing transaction.
func InsertOrder(ctx context.Context, tx *sql.Tx, order LocalOrder) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO local_orders (id, beckn_action, payload_json, status, updated_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		order.ID, order.BecknAction, order.PayloadJSON, order.Status, order.UpdatedAt, order.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert local_order: %w", err)
	}
	return nil
}

// UpdateOrderStatus sets the status and updated_at for an order.
func UpdateOrderStatus(ctx context.Context, tx *sql.Tx, id, status string, updatedAt int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE local_orders SET status = ?, updated_at = ? WHERE id = ?`,
		status, updatedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update local_order status: %w", err)
	}
	return nil
}

// GetOrder loads a local order by ID.
func (s *Store) GetOrder(ctx context.Context, id string) (*LocalOrder, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, beckn_action, payload_json, status, updated_at, created_at
		FROM local_orders WHERE id = ?`, id)

	var o LocalOrder
	if err := row.Scan(&o.ID, &o.BecknAction, &o.PayloadJSON, &o.Status, &o.UpdatedAt, &o.CreatedAt); err != nil {
		return nil, fmt.Errorf("get local_order: %w", err)
	}
	return &o, nil
}
