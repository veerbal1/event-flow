package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type handlers struct {
	pool *pgxpool.Pool
}

type orderItem struct {
	SKU        string `json:"sku"`
	Qty        int64  `json:"qty"`
	PriceCents int64  `json:"price_cents"`
}

type createOrderRequest struct {
	CustomerID string      `json:"customer_id"`
	Items      []orderItem `json:"items"`
}

func (h *handlers) healthz(w http.ResponseWriter, r *http.Request) {
	if err := h.pool.Ping(r.Context()); err != nil {
		slog.Error("ping database", "error", err)
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

func (h *handlers) createOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.CustomerID == "" {
		http.Error(w, "customer_id is required", http.StatusBadRequest)
		return
	}
	if len(req.Items) == 0 {
		http.Error(w, "at least one item is required", http.StatusBadRequest)
		return
	}

	var totalCents int64
	for _, item := range req.Items {
		if item.Qty <= 0 {
			http.Error(w, "item qty must be greater than zero", http.StatusBadRequest)
			return
		}
		if item.PriceCents < 0 {
			http.Error(w, "item price_cents must not be negative", http.StatusBadRequest)
			return
		}
		totalCents += item.Qty * item.PriceCents
	}

	itemsJSON, err := json.Marshal(req.Items)
	if err != nil {
		slog.Error("marshal items", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	id := uuid.New()

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		slog.Error("begin transaction", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	if _, err := tx.Exec(r.Context(),
		`INSERT INTO orders (id, customer_id, items, total_cents) VALUES ($1, $2, $3, $4)`,
		id, req.CustomerID, itemsJSON, totalCents,
	); err != nil {
		slog.Error("insert order", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		slog.Error("commit transaction", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"id":          id.String(),
		"status":      "PLACED",
		"total_cents": totalCents,
	}); err != nil {
		slog.Error("write response", "error", err)
	}
}
