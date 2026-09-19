package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veerbal1/event-flow/internal/db"
	"github.com/veerbal1/event-flow/internal/order"
)

func TestCreateOrder(t *testing.T) {
	pool, handler := testServer(t)
	store := order.NewStore()

	t.Run("valid order", func(t *testing.T) {
		body := `{"customer_id":"c1","items":[{"sku":"ABC","qty":2,"price_cents":1999}]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}

		var resp struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			TotalCents int64  `json:"total_cents"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.TotalCents != 3998 {
			t.Errorf("response total_cents = %d, want 3998", resp.TotalCents)
		}
		if resp.Status != order.StatusPlaced {
			t.Errorf("response status = %q, want %q", resp.Status, order.StatusPlaced)
		}

		stored, err := store.Get(context.Background(), pool, resp.ID)
		if err != nil {
			t.Fatalf("get order %s: %v", resp.ID, err)
		}
		if stored.TotalCents != 3998 {
			t.Errorf("stored total_cents = %d, want 3998", stored.TotalCents)
		}
		if stored.Status != order.StatusPlaced {
			t.Errorf("stored status = %q, want %q", stored.Status, order.StatusPlaced)
		}
	})

	t.Run("empty items", func(t *testing.T) {
		body := `{"customer_id":"c1","items":[]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})
}

func testServer(t *testing.T) (*pgxpool.Pool, http.Handler) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	pool, err := db.Open(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool, NewServer(pool, order.NewStore()).Routes()
}
