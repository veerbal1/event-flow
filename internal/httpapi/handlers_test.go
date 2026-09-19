package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veerbal1/event-flow/internal/db"
	"github.com/veerbal1/event-flow/internal/order"
)

func TestCreateOrder(t *testing.T) {
	pool, handler := testServer(t)
	store := order.NewStore()

	t.Run("valid order", func(t *testing.T) {
		body := `{"customer_id":"c1","items":[{"sku":"ABC","qty":2,"price_cents":1999}]}`
		rec := postOrder(handler, body, uuid.NewString())

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
		rec := postOrder(handler, `{"customer_id":"c1","items":[]}`, uuid.NewString())

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("missing idempotency key", func(t *testing.T) {
		body := `{"customer_id":"c1","items":[{"sku":"ABC","qty":1,"price_cents":100}]}`
		rec := postOrder(handler, body, "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("same key twice", func(t *testing.T) {
		body := `{"customer_id":"c2","items":[{"sku":"XYZ","qty":1,"price_cents":500}]}`
		key := uuid.NewString()

		first := postOrder(handler, body, key)
		if first.Code != http.StatusCreated {
			t.Fatalf("first status = %d, want %d; body: %s", first.Code, http.StatusCreated, first.Body.String())
		}
		if got := first.Header().Get("Idempotent-Replayed"); got != "" {
			t.Errorf("first Idempotent-Replayed = %q, want empty", got)
		}

		second := postOrder(handler, body, key)
		if second.Code != http.StatusOK {
			t.Fatalf("second status = %d, want %d; body: %s", second.Code, http.StatusOK, second.Body.String())
		}
		if got := second.Header().Get("Idempotent-Replayed"); got != "true" {
			t.Errorf("second Idempotent-Replayed = %q, want true", got)
		}

		firstID := decodeID(t, first)
		secondID := decodeID(t, second)
		if firstID != secondID {
			t.Errorf("ids differ: first = %s, second = %s", firstID, secondID)
		}

		var count int
		if err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM orders WHERE idempotency_key = $1`, key,
		).Scan(&count); err != nil {
			t.Fatalf("count orders: %v", err)
		}
		if count != 1 {
			t.Errorf("rows with key = %d, want 1", count)
		}
	})
}

func TestConcurrentCreateOrder(t *testing.T) {
	pool, handler := testServer(t)

	const workers = 50

	key := uuid.NewString()
	body := `{"customer_id":"c9","items":[{"sku":"ABC","qty":1,"price_cents":100}]}`

	var (
		wg      sync.WaitGroup
		start   = make(chan struct{})
		codesMu sync.Mutex
		codes   = make([]int, 0, workers)
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", key)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			codesMu.Lock()
			codes = append(codes, rec.Code)
			codesMu.Unlock()
		}()
	}

	close(start)
	wg.Wait()

	var created, replayed int
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			replayed++
		default:
			t.Errorf("unexpected status %d", code)
		}
	}

	if created != 1 {
		t.Errorf("201 responses = %d, want exactly 1", created)
	}
	if replayed != workers-1 {
		t.Errorf("200 responses = %d, want %d", replayed, workers-1)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM orders WHERE idempotency_key = $1`, key,
	).Scan(&count); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if count != 1 {
		t.Errorf("rows with key = %d, want exactly 1", count)
	}
}

func postOrder(handler http.Handler, body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodeID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.ID
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
