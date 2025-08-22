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
	CreateOrderWithItems(ctx context.Context, order *models.Order, items []models.OrderItem) error
	GetOrderByID(ctx context.Context, id int64) (*models.Order, error)
	GetOrderWithItems(ctx context.Context, id int64) (*models.Order, error)
	ListOrders(ctx context.Context, limit, offset int) ([]*models.Order, error)
	ListOrdersWithItems(ctx context.Context, limit, offset int) ([]*models.Order, error)
}

type PostgresOrdersRepository struct {
	pool *pgxpool.Pool
}

func NewOrdersRepository(pool *pgxpool.Pool) OrdersRepository {
	return &PostgresOrdersRepository{
		pool: pool,
	}
}

// Legacy CreateOrder for backward compatibility (single item orders)
func (r *PostgresOrdersRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	// This is now a wrapper around CreateOrderWithItems for compatibility
	if len(order.Items) == 0 {
		return fmt.Errorf("order must contain at least one item")
	}
	
	return r.CreateOrderWithItems(ctx, order, order.Items)
}

// CreateOrderWithItems creates a new order with multiple items in a transaction
func (r *PostgresOrdersRepository) CreateOrderWithItems(ctx context.Context, order *models.Order, items []models.OrderItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	
	// Create the order
	orderQuery := `
		INSERT INTO orders (customer_id, status)
		VALUES ($1, $2)
		RETURNING id, total_amount, created_at, updated_at
	`
	
	row := tx.QueryRow(ctx, orderQuery, order.CustomerID, "pending")
	err = row.Scan(&order.ID, &order.TotalAmount, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}
	
	// Create order items
	itemQuery := `
		INSERT INTO order_items (order_id, book_id, book_title, book_author, quantity, unit_price)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, line_total, created_at
	`
	
	for i := range items {
		items[i].OrderID = order.ID
		row := tx.QueryRow(ctx, itemQuery,
			items[i].OrderID,
			items[i].BookID,
			items[i].BookTitle,
			items[i].BookAuthor,
			items[i].Quantity,
			items[i].UnitPrice,
		)
		
		err = row.Scan(&items[i].ID, &items[i].LineTotal, &items[i].CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to create order item: %w", err)
		}
	}
	
	// Get updated order total (calculated by database triggers)
	totalQuery := `SELECT total_amount FROM orders WHERE id = $1`
	err = tx.QueryRow(ctx, totalQuery, order.ID).Scan(&order.TotalAmount)
	if err != nil {
		return fmt.Errorf("failed to get order total: %w", err)
	}
	
	// Set the items on the order
	order.Items = items
	order.Status = "pending"
	
	return tx.Commit(ctx)
}

// Legacy GetOrderByID for backward compatibility
func (r *PostgresOrdersRepository) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	return r.GetOrderWithItems(ctx, id)
}

// GetOrderWithItems retrieves an order with all its items
func (r *PostgresOrdersRepository) GetOrderWithItems(ctx context.Context, id int64) (*models.Order, error) {
	// Get order information
	orderQuery := `
		SELECT id, customer_id, status, total_amount, created_at, updated_at
		FROM orders
		WHERE id = $1
	`
	
	row := r.pool.QueryRow(ctx, orderQuery, id)
	
	order := &models.Order{}
	err := row.Scan(
		&order.ID,
		&order.CustomerID,
		&order.Status,
		&order.TotalAmount,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &OrderNotFoundError{ID: id}
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	
	// Get order items
	itemsQuery := `
		SELECT id, order_id, book_id, book_title, book_author, quantity, unit_price, line_total, created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at ASC
	`
	
	rows, err := r.pool.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer rows.Close()
	
	var items []models.OrderItem
	for rows.Next() {
		item := models.OrderItem{}
		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.BookID,
			&item.BookTitle,
			&item.BookAuthor,
			&item.Quantity,
			&item.UnitPrice,
			&item.LineTotal,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		items = append(items, item)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over order items: %w", err)
	}
	
	order.Items = items
	return order, nil
}

// Legacy ListOrders for backward compatibility
func (r *PostgresOrdersRepository) ListOrders(ctx context.Context, limit, offset int) ([]*models.Order, error) {
	return r.ListOrdersWithItems(ctx, limit, offset)
}

// ListOrdersWithItems retrieves orders with their items
func (r *PostgresOrdersRepository) ListOrdersWithItems(ctx context.Context, limit, offset int) ([]*models.Order, error) {
	// Get orders
	orderQuery := `
		SELECT id, customer_id, status, total_amount, created_at, updated_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	
	rows, err := r.pool.Query(ctx, orderQuery, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()
	
	var orders []*models.Order
	orderIDs := make([]int64, 0)
	
	for rows.Next() {
		order := &models.Order{}
		err := rows.Scan(
			&order.ID,
			&order.CustomerID,
			&order.Status,
			&order.TotalAmount,
			&order.CreatedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		
		orders = append(orders, order)
		orderIDs = append(orderIDs, order.ID)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over orders: %w", err)
	}
	
	if len(orderIDs) == 0 {
		return orders, nil
	}
	
	// Get all items for these orders in one query
	itemsQuery := `
		SELECT id, order_id, book_id, book_title, book_author, quantity, unit_price, line_total, created_at
		FROM order_items
		WHERE order_id = ANY($1)
		ORDER BY order_id, created_at ASC
	`
	
	itemRows, err := r.pool.Query(ctx, itemsQuery, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer itemRows.Close()
	
	// Group items by order ID
	itemsByOrderID := make(map[int64][]models.OrderItem)
	for itemRows.Next() {
		item := models.OrderItem{}
		err := itemRows.Scan(
			&item.ID,
			&item.OrderID,
			&item.BookID,
			&item.BookTitle,
			&item.BookAuthor,
			&item.Quantity,
			&item.UnitPrice,
			&item.LineTotal,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		
		itemsByOrderID[item.OrderID] = append(itemsByOrderID[item.OrderID], item)
	}
	
	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over order items: %w", err)
	}
	
	// Assign items to orders
	for _, order := range orders {
		if items, exists := itemsByOrderID[order.ID]; exists {
			order.Items = items
		}
	}
	
	return orders, nil
}

type OrderNotFoundError struct {
	ID int64
}

func (e *OrderNotFoundError) Error() string {
	return fmt.Sprintf("order with ID %d not found", e.ID)
}