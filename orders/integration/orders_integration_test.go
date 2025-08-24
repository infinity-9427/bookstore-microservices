//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/config"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/handlers"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/repository"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/service"
)

// minimal fake books client
type fakeBooksClient struct{ price string }

func (f *fakeBooksClient) GetBook(ctx context.Context, id int64) (*models.Book, error) {
	return &models.Book{ID: id, Title: fmt.Sprintf("B%d", id), Author: "A", Price: f.price, Active: true}, nil
}
func (f *fakeBooksClient) GetBooks(ctx context.Context, ids []int64) (map[int64]*models.Book, error) {
	m := make(map[int64]*models.Book)
	for _, id := range ids {
		m[id], _ = f.GetBook(ctx, id)
	}
	return m, nil
}

func setupDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
	)
	require.NoError(t, err)

	host, err := pg.Host(ctx)
	require.NoError(t, err)
	port, err := pg.MappedPort(ctx, "5432")
	require.NoError(t, err)
	dsn := fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port.Port())

	var pool *pgxpool.Pool
	deadline := time.Now().Add(12 * time.Second)
	var lastErr error
	for {
		cfg, perr := pgxpool.ParseConfig(dsn)
		if perr == nil {
			cfg.MaxConns = 4
			pool, perr = pgxpool.NewWithConfig(ctx, cfg)
			if perr == nil {
				if _, perr = pool.Exec(ctx, "SELECT 1"); perr == nil {
					break
				}
				pool.Close()
			}
		}
		lastErr = perr
		if time.Now().After(deadline) {
			t.Skipf("skipping integration test: database not ready (%v)", lastErr)
		}
		time.Sleep(400 * time.Millisecond)
	}

	// Create minimal schema for orders service
	schema := `CREATE TABLE IF NOT EXISTS orders (
        id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        total_price NUMERIC(10,2) NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );
    CREATE TABLE IF NOT EXISTS order_items (
        id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
        book_id BIGINT NOT NULL,
        book_title TEXT NOT NULL,
        book_author TEXT NOT NULL,
        quantity INTEGER NOT NULL,
        unit_price NUMERIC(10,2) NOT NULL,
        total_price NUMERIC(10,2) NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );
    CREATE TABLE IF NOT EXISTS idempotency_keys (
        key TEXT PRIMARY KEY,
        order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
        request_hash TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );`
	_, err = pool.Exec(ctx, schema)
	require.NoError(t, err)

	cleanup := func() { pool.Close(); pg.Terminate(ctx) }
	return pool, cleanup
}

func newOrdersRouter(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.NewOrdersRepository(pool)
	cfg := &config.Config{IdempotencyEnabled: true}
	svc := service.NewOrdersService(repo, &fakeBooksClient{price: "19.99"}, logger, cfg)
	h := handlers.NewOrdersHandler(svc, logger)
	r := gin.New()
	v1 := r.Group("/v1")
	v1.POST("/orders", h.CreateOrder)
	v1.GET("/orders", h.ListOrders)
	v1.GET("/orders/:id", h.GetOrder)
	return r
}

func TestIntegration_OrderIdempotencyAndPagination(t *testing.T) {
	pool, cleanup := setupDB(t)
	defer cleanup()
	router := newOrdersRouter(t, pool)

	// Create order (2 items same book -> quantity accumulated by validation into 1 line; we just send single)
	body := `{"items":[{"book_id":1,"quantity":2}]}`
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/v1/orders", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "k1")
	router.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusCreated, w1.Code, w1.Body.String())

	// Replay same request with same key -> reused order
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/v1/orders", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "k1")
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusCreated, w2.Code)
	require.Equal(t, w1.Body.String(), w2.Body.String())

	// Different body same key -> conflict
	diffBody := `{"items":[{"book_id":1,"quantity":1}]}`
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/v1/orders", bytes.NewBufferString(diffBody))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Idempotency-Key", "k1")
	router.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusConflict, w3.Code)

	// Create a bunch more orders for pagination
	for i := 0; i < 3; i++ { // keep small for speed
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/orders", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", fmt.Sprintf("bulk-%d-%d", i, time.Now().UnixNano()))
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	// List with limit=2 offset=0 expect next link
	wl := httptest.NewRecorder()
	reqL, _ := http.NewRequest("GET", "/v1/orders?limit=2&offset=0", nil)
	router.ServeHTTP(wl, reqL)
	require.Equal(t, http.StatusOK, wl.Code)
	if wl.Header().Get("Link") == "" {
		t.Fatalf("expected Link header for next page")
	}
	if wl.Header().Get("X-Total-Count") == "" {
		t.Fatalf("expected X-Total-Count header")
	}
}
