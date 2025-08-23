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
	CreateOrderWithIdempotency(ctx context.Context, order *models.Order, idempotencyKey string, requestHash string) error
	GetOrderByID(ctx context.Context, id int64) (*models.Order, error)
	GetOrderByIdempotencyKey(ctx context.Context, idempotencyKey string) (*models.Order, error)
	CheckIdempotencyKey(ctx context.Context, idempotencyKey string, requestHash string) (*models.Order, error)
	ListOrders(ctx context.Context) ([]*models.Order, error)
	ListOrdersPaginated(ctx context.Context, limit, offset int) ([]*models.Order, int, error)
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
	return r.CreateOrderWithIdempotency(ctx, order, "", "")
}

func (r *PostgresOrdersRepository) CreateOrderWithIdempotency(ctx context.Context, order *models.Order, idempotencyKey string, requestHash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Handle idempotency if key is provided
	if idempotencyKey != "" {

		// Check if idempotency key already exists
		var existingOrderID int64
		var existingHash string
		checkQuery := `SELECT order_id, request_hash FROM idempotency_keys WHERE key = $1`
		err = tx.QueryRow(ctx, checkQuery, idempotencyKey).Scan(&existingOrderID, &existingHash)

		if err == nil {
			// Key exists - check if request is the same
			if existingHash != requestHash {
				return &IdempotencyConflictError{Key: idempotencyKey}
			}
			// Same request, fetch the existing order
			tx.Rollback(ctx) // Clean up transaction
			existingOrder, fetchErr := r.GetOrderByID(ctx, existingOrderID)
			if fetchErr != nil {
				return fmt.Errorf("failed to fetch existing order: %w", fetchErr)
			}
			// Copy the existing order data to the provided order struct
			*order = *existingOrder
			return nil // Success - order now contains existing order data
		} else if err != pgx.ErrNoRows {
			return fmt.Errorf("failed to check idempotency key: %w", err)
		}
		// Key doesn't exist, continue with creation
	}

	// Create the order
	orderQuery := `INSERT INTO orders (total_price) VALUES ($1) RETURNING id, created_at`
	err = tx.QueryRow(ctx, orderQuery, order.TotalPrice).Scan(&order.ID, &order.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	// Create order items
	itemQuery := `
		INSERT INTO order_items (order_id, book_id, book_title, book_author, quantity, unit_price, total_price)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	for i := range order.Items {
		order.Items[i].OrderID = order.ID
		err = tx.QueryRow(ctx, itemQuery,
			order.ID,
			order.Items[i].BookID,
			order.Items[i].BookTitle,
			order.Items[i].BookAuthor,
			order.Items[i].Quantity,
			order.Items[i].UnitPrice,
			order.Items[i].TotalPrice, // Renamed from LineTotal
		).Scan(&order.Items[i].ID, &order.Items[i].CreatedAt)

		if err != nil {
			return fmt.Errorf("failed to create order item: %w", err)
		}
	}

	// Store idempotency key if provided
	if idempotencyKey != "" {
		idempotencyQuery := `INSERT INTO idempotency_keys (key, order_id, request_hash, created_at) VALUES ($1, $2, $3, NOW())`
		_, err = tx.Exec(ctx, idempotencyQuery, idempotencyKey, order.ID, requestHash)
		if err != nil {
			return fmt.Errorf("failed to store idempotency key: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresOrdersRepository) GetOrderByIdempotencyKey(ctx context.Context, idempotencyKey string) (*models.Order, error) {
	// Get order ID from idempotency table
	var orderID int64
	idempotencyQuery := `SELECT order_id FROM idempotency_keys WHERE key = $1`
	err := r.pool.QueryRow(ctx, idempotencyQuery, idempotencyKey).Scan(&orderID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &OrderNotFoundError{ID: 0}
		}
		return nil, fmt.Errorf("failed to get order by idempotency key: %w", err)
	}

	// Get the order
	return r.GetOrderByID(ctx, orderID)
}

func (r *PostgresOrdersRepository) CheckIdempotencyKey(ctx context.Context, idempotencyKey string, requestHash string) (*models.Order, error) {
	// Check if idempotency key exists and get the hash
	var existingOrderID int64
	var existingHash string
	checkQuery := `SELECT order_id, request_hash FROM idempotency_keys WHERE key = $1`
	err := r.pool.QueryRow(ctx, checkQuery, idempotencyKey).Scan(&existingOrderID, &existingHash)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &OrderNotFoundError{ID: 0} // Key doesn't exist
		}
		return nil, fmt.Errorf("failed to check idempotency key: %w", err)
	}

	// Key exists - check if request is the same
	if existingHash != requestHash {
		return nil, &IdempotencyConflictError{Key: idempotencyKey}
	}

	// Same request, return existing order
	return r.GetOrderByID(ctx, existingOrderID)
}

func (r *PostgresOrdersRepository) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	// Get order
	orderQuery := `SELECT id, total_price, created_at FROM orders WHERE id = $1`

	var order models.Order
	err := r.pool.QueryRow(ctx, orderQuery, id).Scan(&order.ID, &order.TotalPrice, &order.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &OrderNotFoundError{ID: id}
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Get order items
	itemsQuery := `
		SELECT id, order_id, book_id, book_title, book_author, quantity, unit_price, total_price, created_at
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
		var item models.OrderItem
		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.BookID,
			&item.BookTitle,
			&item.BookAuthor,
			&item.Quantity,
			&item.UnitPrice,
			&item.TotalPrice, // Renamed from LineTotal
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order items: %w", err)
	}

	order.Items = items
	return &order, nil
}

func (r *PostgresOrdersRepository) ListOrders(ctx context.Context) ([]*models.Order, error) {
	// Get all orders
	ordersQuery := `SELECT id, total_price, created_at FROM orders ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, ordersQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	var orderMap = make(map[int64]*models.Order)
	var orderIDs []int64

	for rows.Next() {
		var order models.Order
		err := rows.Scan(&order.ID, &order.TotalPrice, &order.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		order.Items = make([]models.OrderItem, 0)
		orderMap[order.ID] = &order
		orderIDs = append(orderIDs, order.ID)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating orders: %w", err)
	}

	if len(orderIDs) == 0 {
		return []*models.Order{}, nil
	}

	// Get all order items for these orders
	itemsQuery := `
		SELECT id, order_id, book_id, book_title, book_author, quantity, unit_price, total_price, created_at
		FROM order_items
		WHERE order_id = ANY($1)
		ORDER BY order_id, created_at ASC
	`

	itemRows, err := r.pool.Query(ctx, itemsQuery, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var item models.OrderItem
		err := itemRows.Scan(
			&item.ID,
			&item.OrderID,
			&item.BookID,
			&item.BookTitle,
			&item.BookAuthor,
			&item.Quantity,
			&item.UnitPrice,
			&item.TotalPrice, // Renamed from LineTotal
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}

		if order, exists := orderMap[item.OrderID]; exists {
			order.Items = append(order.Items, item)
		}
	}

	if err = itemRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order items: %w", err)
	}

	// Convert map to slice maintaining order
	orders := make([]*models.Order, 0, len(orderIDs))
	for _, id := range orderIDs {
		orders = append(orders, orderMap[id])
	}

	return orders, nil
}

func (r *PostgresOrdersRepository) ListOrdersPaginated(ctx context.Context, limit, offset int) ([]*models.Order, int, error) {
	// First get total count
	countQuery := `SELECT COUNT(*) FROM orders`
	var total int
	err := r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	// Get paginated orders
	ordersQuery := `SELECT id, total_price, created_at FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, ordersQuery, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	var orderMap = make(map[int64]*models.Order)
	var orderIDs []int64

	for rows.Next() {
		var order models.Order
		err := rows.Scan(&order.ID, &order.TotalPrice, &order.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan order: %w", err)
		}
		order.Items = make([]models.OrderItem, 0)
		orderMap[order.ID] = &order
		orderIDs = append(orderIDs, order.ID)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating orders: %w", err)
	}

	if len(orderIDs) == 0 {
		return []*models.Order{}, total, nil
	}

	// Get all order items for these orders
	itemsQuery := `
		SELECT id, order_id, book_id, book_title, book_author, quantity, unit_price, total_price, created_at
		FROM order_items
		WHERE order_id = ANY($1)
		ORDER BY order_id, created_at ASC
	`

	itemRows, err := r.pool.Query(ctx, itemsQuery, orderIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get order items: %w", err)
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var item models.OrderItem
		err := itemRows.Scan(
			&item.ID,
			&item.OrderID,
			&item.BookID,
			&item.BookTitle,
			&item.BookAuthor,
			&item.Quantity,
			&item.UnitPrice,
			&item.TotalPrice, // Renamed from LineTotal
			&item.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan order item: %w", err)
		}

		if order, exists := orderMap[item.OrderID]; exists {
			order.Items = append(order.Items, item)
		}
	}

	if err = itemRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating order items: %w", err)
	}

	// Convert map to slice maintaining order
	orders := make([]*models.Order, 0, len(orderIDs))
	for _, id := range orderIDs {
		orders = append(orders, orderMap[id])
	}

	return orders, total, nil
}

// Repository error types
type OrderNotFoundError struct {
	ID int64
}

func (e *OrderNotFoundError) Error() string {
	return fmt.Sprintf("order with ID %d not found", e.ID)
}

type IdempotencyConflictError struct {
	Key string
}

func (e *IdempotencyConflictError) Error() string {
	return fmt.Sprintf("idempotency key '%s' already used with different request body", e.Key)
}
