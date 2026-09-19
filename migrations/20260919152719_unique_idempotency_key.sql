-- +goose Up
CREATE UNIQUE INDEX orders_idempotency_key_key ON orders (idempotency_key);

-- +goose Down
DROP INDEX orders_idempotency_key_key;
