package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourname/bookstore-microservices/orders/internal/models"
)

type OrdersRepository interface {
	CreateOrder(ctx context.Context, order *models.Order) error
	GetOrderByID(ctx context.Context, id int64) (*models.Order, error)
	ListOrders(ctx context.Context, limit, offset int) ([]*models.Order, error)
}

type PostgresOrdersRepository struct {
	pool *pgxpool.Pool
}

func NewOrdersRepository(pool *pgxpool.Pool) OrdersRepository {
	return &PostgresOrdersRepository{
		pool: pool,
	}
}

func (r *PostgresOrdersRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	query := `
		INSERT INTO orders (book_id, book_title, book_author, quantity, unit_price)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, total_price, created_at
	`
	
	row := r.pool.QueryRow(ctx, query, 
		order.BookID, 
		order.BookTitle, 
		order.BookAuthor, 
		order.Quantity, 
		order.UnitPrice,
	)
	
	err := row.Scan(&order.ID, &order.TotalPrice, &order.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}
	
	return nil
}

func (r *PostgresOrdersRepository) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	query := `
		SELECT id, book_id, book_title, book_author, quantity, unit_price, total_price, created_at
		FROM orders
		WHERE id = $1
	`
	
	row := r.pool.QueryRow(ctx, query, id)
	
	order := &models.Order{}
	err := row.Scan(
		&order.ID,
		&order.BookID,
		&order.BookTitle,
		&order.BookAuthor,
		&order.Quantity,
		&order.UnitPrice,
		&order.TotalPrice,
		&order.CreatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &OrderNotFoundError{ID: id}
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	
	return order, nil
}

func (r *PostgresOrdersRepository) ListOrders(ctx context.Context, limit, offset int) ([]*models.Order, error) {
	query := `
		SELECT id, book_id, book_title, book_author, quantity, unit_price, total_price, created_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()
	
	var orders []*models.Order
	
	for rows.Next() {
		order := &models.Order{}
		err := rows.Scan(
			&order.ID,
			&order.BookID,
			&order.BookTitle,
			&order.BookAuthor,
			&order.Quantity,
			&order.UnitPrice,
			&order.TotalPrice,
			&order.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		
		orders = append(orders, order)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over orders: %w", err)
	}
	
	return orders, nil
}

type OrderNotFoundError struct {
	ID int64
}

func (e *OrderNotFoundError) Error() string {
	return fmt.Sprintf("order with ID %d not found", e.ID)
}