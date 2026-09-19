package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/veerbal1/event-flow/internal/db"
)

const idempotencyKeyIndex = "orders_idempotency_key_key"

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
		`INSERT INTO orders (id, customer_id, items, total_cents, status, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		o.ID, o.CustomerID, items, o.TotalCents, o.Status, o.IdempotencyKey,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == idempotencyKeyIndex {
			return fmt.Errorf("insert order: %w", ErrDuplicateKey)
		}
		return fmt.Errorf("insert order: %w", err)
	}

	return nil
}

func (s *Store) Get(ctx context.Context, q db.Querier, id string) (Order, error) {
	return s.scanOne(ctx, q,
		`SELECT id, customer_id, items, total_cents, status, idempotency_key, created_at
		 FROM orders WHERE id = $1`,
		id,
	)
}

func (s *Store) GetByIdempotencyKey(ctx context.Context, q db.Querier, key string) (Order, error) {
	o, err := s.scanOne(ctx, q,
		`SELECT id, customer_id, items, total_cents, status, idempotency_key, created_at
		 FROM orders WHERE idempotency_key = $1`,
		key,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, fmt.Errorf("order with idempotency key %q: %w", key, ErrNotFound)
	}
	return o, err
}

func (s *Store) scanOne(ctx context.Context, q db.Querier, sql string, args ...any) (Order, error) {
	var (
		o     Order
		items []byte
	)

	if err := q.QueryRow(ctx, sql, args...).Scan(
		&o.ID, &o.CustomerID, &items, &o.TotalCents, &o.Status, &o.IdempotencyKey, &o.CreatedAt,
	); err != nil {
		return Order{}, fmt.Errorf("select order: %w", err)
	}

	if err := json.Unmarshal(items, &o.Items); err != nil {
		return Order{}, fmt.Errorf("unmarshal items: %w", err)
	}

	return o, nil
}
