package order

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/veerbal1/event-flow/internal/db"
)

type Store struct{}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Create(ctx context.Context, q db.Querier, o Order) error {
	items, err := json.Marshal(o.Items)
	if err != nil {
		return fmt.Errorf("marshal items: %w", err)
	}

	if _, err := q.Exec(ctx,
		`INSERT INTO orders (id, customer_id, items, total_cents, status) VALUES ($1, $2, $3, $4, $5)`,
		o.ID, o.CustomerID, items, o.TotalCents, o.Status,
	); err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	return nil
}

func (s *Store) Get(ctx context.Context, q db.Querier, id string) (Order, error) {
	var (
		o     Order
		items []byte
	)

	if err := q.QueryRow(ctx,
		`SELECT id, customer_id, items, total_cents, status, created_at FROM orders WHERE id = $1`,
		id,
	).Scan(&o.ID, &o.CustomerID, &items, &o.TotalCents, &o.Status, &o.CreatedAt); err != nil {
		return Order{}, fmt.Errorf("select order: %w", err)
	}

	if err := json.Unmarshal(items, &o.Items); err != nil {
		return Order{}, fmt.Errorf("unmarshal items: %w", err)
	}

	return o, nil
}
