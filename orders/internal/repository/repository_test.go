//go:build repository_disabled
// +build repository_disabled

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock"
)

// helper to must parse time
func mustTime() time.Time { return time.Now().UTC() }

// NewOrdersRepository accepts *pgxpool.Pool; pgxmock.NewPool returns *pgxmock.PgxPool which is
// compatible due to embedding; we can pass it directly.

func TestCreateOrderWithIdempotency_NewKey(t *testing.T) {
	pool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer pool.Close()

	order := &models.Order{TotalPrice: "39.98", Items: []models.OrderItem{{BookID: 1, BookTitle: "T", BookAuthor: "A", Quantity: 2, UnitPrice: "19.99", TotalPrice: "39.98"}}}

	// Begin
	pool.ExpectBegin()
	// Idempotency check (no rows)
	pool.ExpectQuery(`SELECT order_id, request_hash FROM idempotency_keys WHERE key = \$1`).
		WithArgs("idem-key").
		WillReturnError(pgx.ErrNoRows)
	// Insert order
	created := mustTime()
	pool.ExpectQuery(`INSERT INTO orders \(total_price\) VALUES \(\$1\) RETURNING id, created_at`).
		WithArgs(order.TotalPrice).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(42), created))
	// Insert item (QueryRow pattern)
	pool.ExpectQuery(`INSERT INTO order_items`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(7), created))
	// Insert idempotency key
	pool.ExpectExec(`INSERT INTO idempotency_keys`).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	pool.ExpectCommit()

	repo := NewOrdersRepository(pool)
	if err := repo.CreateOrderWithIdempotency(context.Background(), order, "idem-key", "hash123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.ID != 42 {
		t.Fatalf("want order.ID=42 got %d", order.ID)
	}
	if len(order.Items) != 1 || order.Items[0].ID != 7 {
		t.Fatalf("item IDs not populated: %+v", order.Items)
	}
	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateOrderWithIdempotency_ExistingSameHash(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	defer pool.Close()

	order := &models.Order{TotalPrice: "19.99"}
	created := mustTime()

	pool.ExpectBegin()
	// Existing key returns same hash
	pool.ExpectQuery(`SELECT order_id, request_hash FROM idempotency_keys WHERE key = \$1`).
		WithArgs("k").
		WillReturnRows(pgxmock.NewRows([]string{"order_id", "request_hash"}).AddRow(int64(55), "h1"))
	// After rollback, GetOrderByID is called (select order + items)
	pool.ExpectQuery(`SELECT id, total_price, created_at FROM orders WHERE id = \$1`).
		WithArgs(int64(55)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "total_price", "created_at"}).AddRow(int64(55), "19.99", created))
	pool.ExpectQuery(`SELECT id, order_id, book_id, book_title, book_author, quantity, unit_price, total_price, created_at FROM order_items`).
		WithArgs(int64(55)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "order_id", "book_id", "book_title", "book_author", "quantity", "unit_price", "total_price", "created_at"}))

	repo := NewOrdersRepository(pool)
	if err := repo.CreateOrderWithIdempotency(context.Background(), order, "k", "h1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if order.ID != 55 {
		t.Fatalf("want reused order 55 got %d", order.ID)
	}
	if err := pool.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateOrderWithIdempotency_Conflict(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	defer pool.Close()
	pool.ExpectBegin()
	pool.ExpectQuery(`SELECT order_id, request_hash FROM idempotency_keys WHERE key = \$1`).
		WithArgs("k").
		WillReturnRows(pgxmock.NewRows([]string{"order_id", "request_hash"}).AddRow(int64(55), "different"))
	order := &models.Order{TotalPrice: "10.00"}
	repo := NewOrdersRepository(pool)
	err := repo.CreateOrderWithIdempotency(context.Background(), order, "k", "h1")
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	var conf *IdempotencyConflictError
	if !errors.As(err, &conf) {
		t.Fatalf("wrong error type: %v", err)
	}
}

func TestCreateOrderWithIdempotency_ItemInsertFails(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	defer pool.Close()
	order := &models.Order{TotalPrice: "39.98", Items: []models.OrderItem{{BookID: 1, BookTitle: "T", BookAuthor: "A", Quantity: 2, UnitPrice: "19.99", TotalPrice: "39.98"}}}
	created := mustTime()
	pool.ExpectBegin()
	pool.ExpectQuery(`SELECT order_id, request_hash FROM idempotency_keys WHERE key = \$1`).WithArgs("k").WillReturnError(pgx.ErrNoRows)
	pool.ExpectQuery(`INSERT INTO orders`).WithArgs(order.TotalPrice).WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(70), created))
	pool.ExpectQuery(`INSERT INTO order_items`).WillReturnError(errors.New("item insert failed"))
	repo := NewOrdersRepository(pool)
	if err := repo.CreateOrderWithIdempotency(context.Background(), order, "k", "h"); err == nil {
		t.Fatalf("expected error on item insert")
	}
}

func TestGetOrderByID_NotFound(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	defer pool.Close()
	pool.ExpectQuery(`SELECT id, total_price, created_at FROM orders WHERE id = \$1`).WithArgs(int64(999)).WillReturnError(pgx.ErrNoRows)
	repo := NewOrdersRepository(pool)
	_, err := repo.GetOrderByID(context.Background(), 999)
	if err == nil {
		t.Fatalf("expected not found")
	}
}

func TestGetOrderByID_Found(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	defer pool.Close()
	created := mustTime()
	pool.ExpectQuery(`SELECT id, total_price, created_at FROM orders WHERE id = \$1`).WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "total_price", "created_at"}).AddRow(int64(1), "19.99", created))
	pool.ExpectQuery(`SELECT id, order_id, book_id, book_title, book_author, quantity, unit_price, total_price, created_at FROM order_items`).
		WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "order_id", "book_id", "book_title", "book_author", "quantity", "unit_price", "total_price", "created_at"}).
			AddRow(int64(10), int64(1), int64(100), "T", "A", 1, "19.99", "19.99", created))
	repo := NewOrdersRepository(pool)
	o, err := repo.GetOrderByID(context.Background(), 1)
	if err != nil || o.ID != 1 || len(o.Items) != 1 {
		t.Fatalf("unexpected %+v err=%v", o, err)
	}
}

func TestListOrdersPaginated_Empty(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	defer pool.Close()
	pool.ExpectQuery(`SELECT COUNT\(\*\) FROM orders`).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	pool.ExpectQuery(`SELECT id, total_price, created_at FROM orders ORDER BY created_at DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 0).WillReturnRows(pgxmock.NewRows([]string{"id", "total_price", "created_at"}))
	repo := NewOrdersRepository(pool)
	rows, total, err := repo.ListOrdersPaginated(context.Background(), 20, 0)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("unexpected rows=%v total=%d err=%v", rows, total, err)
	}
}

func TestListOrdersPaginated_Multi(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	defer pool.Close()
	created := mustTime()
	pool.ExpectQuery(`SELECT COUNT\(\*\) FROM orders`).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))
	pool.ExpectQuery(`SELECT id, total_price, created_at FROM orders ORDER BY created_at DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(50, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "total_price", "created_at"}).
			AddRow(int64(2), "29.99", created).AddRow(int64(1), "19.99", created))
	pool.ExpectQuery(`SELECT id, order_id, book_id, book_title, book_author, quantity, unit_price, total_price, created_at FROM order_items`).
		WithArgs([]int64{2, 1}).
		WillReturnRows(pgxmock.NewRows([]string{"id", "order_id", "book_id", "book_title", "book_author", "quantity", "unit_price", "total_price", "created_at"}).
			AddRow(int64(11), int64(2), int64(200), "B2", "A2", 1, "29.99", "29.99", created).
			AddRow(int64(10), int64(1), int64(100), "B1", "A1", 1, "19.99", "19.99", created))
	repo := NewOrdersRepository(pool)
	rows, total, err := repo.ListOrdersPaginated(context.Background(), 50, 0)
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("unexpected rows=%v total=%d err=%v", rows, total, err)
	}
}
