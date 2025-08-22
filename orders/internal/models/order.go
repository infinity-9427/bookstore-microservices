package models

import (
	"fmt"
	"time"
)

type Order struct {
	ID          int64       `json:"id" db:"id"`
	CustomerID  *string     `json:"customer_id,omitempty" db:"customer_id"`
	Status      string      `json:"status" db:"status"`
	TotalAmount float64     `json:"total_amount" db:"total_amount"`
	Items       []OrderItem `json:"items,omitempty"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

type OrderItem struct {
	ID         int64     `json:"id" db:"id"`
	OrderID    int64     `json:"order_id" db:"order_id"`
	BookID     int64     `json:"book_id" db:"book_id"`
	BookTitle  string    `json:"book_title" db:"book_title"`
	BookAuthor string    `json:"book_author" db:"book_author"`
	Quantity   int       `json:"quantity" db:"quantity"`
	UnitPrice  float64   `json:"unit_price" db:"unit_price"`
	LineTotal  float64   `json:"line_total" db:"line_total"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type CreateOrderItemRequest struct {
	BookID   int64 `json:"book_id" binding:"required"`
	Quantity int   `json:"quantity" binding:"required"`
}

func (r *CreateOrderItemRequest) Validate() error {
	if r.BookID <= 0 {
		return fmt.Errorf("book_id must be greater than 0")
	}
	if r.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}
	if r.Quantity > 100000 {
		return fmt.Errorf("quantity cannot exceed 100000")
	}
	return nil
}

type CreateOrderRequest struct {
	CustomerID *string                   `json:"customer_id,omitempty"`
	Items      []CreateOrderItemRequest  `json:"items" binding:"required,min=1,dive"`
}

func (r *CreateOrderRequest) Validate() error {
	if len(r.Items) == 0 {
		return fmt.Errorf("order must contain at least one item")
	}
	if len(r.Items) > 50 {
		return fmt.Errorf("order cannot contain more than 50 items")
	}
	
	// Validate each item
	for i, item := range r.Items {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("item %d: %w", i+1, err)
		}
	}
	
	// Check for duplicate book IDs
	bookIDs := make(map[int64]bool)
	for i, item := range r.Items {
		if bookIDs[item.BookID] {
			return fmt.Errorf("item %d: duplicate book_id %d", i+1, item.BookID)
		}
		bookIDs[item.BookID] = true
	}
	
	return nil
}

// Legacy single-book order request for backward compatibility
type CreateLegacyOrderRequest struct {
	BookID   int64 `json:"book_id" binding:"required"`
	Quantity int   `json:"quantity" binding:"required"`
}

func (r *CreateLegacyOrderRequest) Validate() error {
	if r.BookID <= 0 {
		return fmt.Errorf("book_id must be greater than 0")
	}
	if r.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}
	if r.Quantity > 100000 {
		return fmt.Errorf("quantity cannot exceed 100000")
	}
	return nil
}

// Convert legacy request to new format
func (r *CreateLegacyOrderRequest) ToCreateOrderRequest() *CreateOrderRequest {
	return &CreateOrderRequest{
		Items: []CreateOrderItemRequest{
			{
				BookID:   r.BookID,
				Quantity: r.Quantity,
			},
		},
	}
}

type Book struct {
	ID     int64   `json:"id"`
	Title  string  `json:"title"`
	Author string  `json:"author"`
	Price  string  `json:"price"`
	Active bool    `json:"active"`
}

type ErrorResponse struct {
	Error   string      `json:"error"`
	Details interface{} `json:"details,omitempty"`
}

type ListOrdersQuery struct {
	Limit  int `form:"limit"`
	Offset int `form:"offset"`
}

func (q *ListOrdersQuery) SetDefaults() {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
}