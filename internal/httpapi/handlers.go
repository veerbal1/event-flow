package httpapi

import (
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
	var req createOrderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	o, err := order.New(req.CustomerID, req.Items)
	if err != nil {
		respondError(w, err)
		return
	}

	if err := s.store.Create(r.Context(), s.pool, o); err != nil {
		respondError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createOrderResponse{
		ID:         o.ID,
		Status:     o.Status,
		TotalCents: o.TotalCents,
	})
}
