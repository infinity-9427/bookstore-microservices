package models

import (
	"fmt"
	"time"
)

type Order struct {
	ID         int64     `json:"id" db:"id"`
	BookID     int64     `json:"book_id" db:"book_id"`
	BookTitle  string    `json:"book_title" db:"book_title"`
	BookAuthor string    `json:"book_author" db:"book_author"`
	Quantity   int       `json:"quantity" db:"quantity"`
	UnitPrice  float64   `json:"unit_price" db:"unit_price"`
	TotalPrice float64   `json:"total_price" db:"total_price"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type CreateOrderRequest struct {
	BookID   int64 `json:"book_id" binding:"required"`
	Quantity int   `json:"quantity" binding:"required"`
}

func (r *CreateOrderRequest) Validate() error {
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