-- +goose Up
ALTER TABLE orders ADD COLUMN idempotency_key TEXT;

-- +goose Down
ALTER TABLE orders DROP COLUMN idempotency_key;
