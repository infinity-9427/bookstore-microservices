package models

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type Order struct {
	ID         int64       `json:"id" db:"id"`
	Items      []OrderItem `json:"items"`
	TotalPrice string      `json:"total_price" db:"total_price"` // Renamed from total_amount, always 2dp string
	CreatedAt  time.Time   `json:"created_at" db:"created_at"`
}

type OrderItem struct {
	ID         int64     `json:"id" db:"id"`
	OrderID    int64     `json:"order_id" db:"order_id"`
	BookID     int64     `json:"book_id" db:"book_id"`
	BookTitle  string    `json:"book_title" db:"book_title"`
	BookAuthor string    `json:"book_author" db:"book_author"`
	Quantity   int       `json:"quantity" db:"quantity"`
	UnitPrice  string    `json:"unit_price" db:"unit_price"`   // Always 2dp string from decimal
	TotalPrice string    `json:"total_price" db:"total_price"` // Renamed from line_total, always 2dp string
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type CreateOrderRequest struct {
	Items []CreateOrderItemRequest `json:"items" binding:"required,min=1"`
}

type CreateOrderItemRequest struct {
	BookID   int64 `json:"book_id" binding:"required"`
	Quantity int   `json:"quantity" binding:"required"`
}

func (r *CreateOrderRequest) Validate() error {
	if len(r.Items) == 0 {
		return fmt.Errorf("order must contain at least one item")
	}

	for i, item := range r.Items {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("item %d: %w", i+1, err)
		}
	}

	// Normalize duplicate book IDs by summing quantities
	bookItems := make(map[int64]*CreateOrderItemRequest)
	for _, item := range r.Items {
		if existing, exists := bookItems[item.BookID]; exists {
			existing.Quantity += item.Quantity
		} else {
			bookItems[item.BookID] = &CreateOrderItemRequest{
				BookID:   item.BookID,
				Quantity: item.Quantity,
			}
		}
	}

	// Replace items with normalized list
	r.Items = make([]CreateOrderItemRequest, 0, len(bookItems))
	for _, item := range bookItems {
		r.Items = append(r.Items, *item)
	}

	return nil
}

func (r *CreateOrderItemRequest) Validate() error {
	if r.BookID <= 0 {
		return fmt.Errorf("book_id must be greater than 0")
	}
	if r.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}
	if r.Quantity > 10000 {
		return fmt.Errorf("quantity cannot exceed 10000")
	}
	return nil
}

type Book struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Price       string `json:"price"` // String price from Books API - never use floats
	Active      bool   `json:"active"`
}

// GetPriceDecimal returns the price as an exact decimal for precise calculations
// Never uses float64 - parses string directly to decimal.Decimal
func (b *Book) GetPriceDecimal() (decimal.Decimal, error) {
	price, err := decimal.NewFromString(b.Price)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid price format '%s': %w", b.Price, err)
	}
	return price, nil
}

// FormatPrice formats a decimal price as a 2-decimal-place string
func FormatPrice(d decimal.Decimal) string {
	return d.StringFixed(2)
}

// ParsePrice parses a price string into decimal.Decimal
func ParsePrice(priceStr string) (decimal.Decimal, error) {
	return decimal.NewFromString(priceStr)
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// PaginationRequest represents pagination parameters
type PaginationRequest struct {
	Limit  int `form:"limit,default=20" binding:"min=1,max=100"`
	Offset int `form:"offset,default=0" binding:"min=0"`
}

// PaginatedResponse wraps paginated data with metadata
type PaginatedResponse[T any] struct {
	Data   []T `json:"data"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
