package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
	"github.com/yourname/bookstore-microservices/orders/internal/models"
)

type BooksClient interface {
	GetBook(ctx context.Context, bookID int64) (*models.Book, error)
	HealthCheck(ctx context.Context) error
}

type HTTPBooksClient struct {
	baseURL     string
	httpClient  *http.Client
	breaker     *gobreaker.CircuitBreaker
}

func NewBooksClient(baseURL string, timeout time.Duration, failureThreshold uint32) BooksClient {
	settings := gobreaker.Settings{
		Name:        "books-service",
		MaxRequests: 3,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= failureThreshold
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit breaker %s changed from %s to %s\n", name, from, to)
		},
	}

	return &HTTPBooksClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		breaker: gobreaker.NewCircuitBreaker(settings),
	}
}

func (c *HTTPBooksClient) GetBook(ctx context.Context, bookID int64) (*models.Book, error) {
	result, err := c.breaker.Execute(func() (interface{}, error) {
		return c.getBook(ctx, bookID)
	})
	
	if err != nil {
		// If it's a business logic error (BookNotFound, BookNotActive), return as-is
		// without wrapping with "books service unavailable"
		switch err.(type) {
		case *BookNotFoundError, *BookNotActiveError:
			return nil, err
		default:
			return nil, fmt.Errorf("books service unavailable: %w", err)
		}
	}
	
	return result.(*models.Book), nil
}

func (c *HTTPBooksClient) getBook(ctx context.Context, bookID int64) (*models.Book, error) {
	url := fmt.Sprintf("%s/v1/books/%d", c.baseURL, bookID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	if requestID := ctx.Value("request_id"); requestID != nil {
		req.Header.Set("X-Request-ID", requestID.(string))
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	// Business logic errors should not trigger circuit breaker
	if resp.StatusCode == http.StatusNotFound {
		return nil, &BookNotFoundError{BookID: bookID}
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("books service returned status %d", resp.StatusCode)
	}
	
	var book models.Book
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Business logic errors should not trigger circuit breaker
	if !book.Active {
		return nil, &BookNotActiveError{BookID: bookID}
	}
	
	return &book, nil
}

func (c *HTTPBooksClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("books service health check returned status %d", resp.StatusCode)
	}
	
	return nil
}

type BookNotFoundError struct {
	BookID int64
}

func (e *BookNotFoundError) Error() string {
	return fmt.Sprintf("book with ID %d not found", e.BookID)
}

type BookNotActiveError struct {
	BookID int64
}

func (e *BookNotActiveError) Error() string {
	return fmt.Sprintf("book with ID %d is not active", e.BookID)
}