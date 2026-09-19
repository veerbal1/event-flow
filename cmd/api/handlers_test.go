package main

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
)

func TestCreateOrder(t *testing.T) {
	pool, mux := testServer(t)

	t.Run("valid order", func(t *testing.T) {
		body := `{"customer_id":"c1","items":[{"sku":"ABC","qty":2,"price_cents":1999}]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

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
		if resp.Status != "PLACED" {
			t.Errorf("response status = %q, want PLACED", resp.Status)
		}

		var totalCents int64
		var status string
		if err := pool.QueryRow(context.Background(),
			`SELECT total_cents, status FROM orders WHERE id = $1`, resp.ID,
		).Scan(&totalCents, &status); err != nil {
			t.Fatalf("query order %s: %v", resp.ID, err)
		}
		if totalCents != 3998 {
			t.Errorf("stored total_cents = %d, want 3998", totalCents)
		}
		if status != "PLACED" {
			t.Errorf("stored status = %q, want PLACED", status)
		}
	})

	t.Run("empty items", func(t *testing.T) {
		body := `{"customer_id":"c1","items":[]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})
}

func testServer(t *testing.T) (*pgxpool.Pool, *http.ServeMux) {
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

	return pool, newMux(&handlers{pool: pool})
}
