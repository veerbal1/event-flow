package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veerbal1/event-flow/internal/order"
)

type Server struct {
	pool  *pgxpool.Pool
	store *order.Store
}

func NewServer(pool *pgxpool.Pool, store *order.Store) *Server {
	return &Server{
		pool:  pool,
		store: store,
	}
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /orders", s.createOrder)
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		slog.Error("ping database", "error", err)
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	w.Write([]byte("ok"))
}

type createOrderRequest struct {
	CustomerID string       `json:"customer_id"`
	Items      []order.Item `json:"items"`
}

type createOrderResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	TotalCents int64  `json:"total_cents"`
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")

	if idempotencyKey != "" {
		existing, err := s.store.GetByIdempotencyKey(r.Context(), s.pool, idempotencyKey)
		switch {
		case err == nil:
			writeReplay(w, existing)
			return
		case !errors.Is(err, order.ErrNotFound):
			respondError(w, err)
			return
		}
	}

	var req createOrderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	o, err := order.New(req.CustomerID, req.Items, idempotencyKey)
	if err != nil {
		respondError(w, err)
		return
	}

	if err := s.store.Create(r.Context(), s.pool, o); err != nil {
		if errors.Is(err, order.ErrDuplicateKey) {
			existing, err := s.store.GetByIdempotencyKey(r.Context(), s.pool, idempotencyKey)
			if err != nil {
				respondError(w, err)
				return
			}
			writeReplay(w, existing)
			return
		}
		respondError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createOrderResponse{
		ID:         o.ID,
		Status:     o.Status,
		TotalCents: o.TotalCents,
	})
}

func writeReplay(w http.ResponseWriter, o order.Order) {
	w.Header().Set("Idempotent-Replayed", "true")
	writeJSON(w, http.StatusOK, createOrderResponse{
		ID:         o.ID,
		Status:     o.Status,
		TotalCents: o.TotalCents,
	})
}
