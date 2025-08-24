package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
)

// Metrics interface for Books client
type BooksMetrics interface {
	IncBooksRequest(result string)
	ObserveBooksLatency(duration time.Duration)
}

type BooksClient interface {
	GetBook(ctx context.Context, bookID int64) (*models.Book, error)
	GetBooks(ctx context.Context, bookIDs []int64) (map[int64]*models.Book, error)
}

type HTTPBooksClient struct {
	http    *http.Client
	base    string
	logger  *slog.Logger
	metrics BooksMetrics

	// Circuit breaker state
	circuitMutex    sync.RWMutex
	circuitOpen     bool
	circuitOpenTime time.Time
	failureCount    int
	threshold       int
	cooldownPeriod  time.Duration
}

type CircuitBreakerError struct {
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return e.Message
}

type BookNotFoundError struct {
	BookID int64
}

func (e *BookNotFoundError) Error() string {
	return fmt.Sprintf("book with ID %d not found", e.BookID)
}

type BookInactiveError struct {
	BookID int64
}

func (e *BookInactiveError) Error() string {
	return fmt.Sprintf("book with ID %d is inactive", e.BookID)
}

type ServiceUnavailableError struct {
	Message string
}

func (e *ServiceUnavailableError) Error() string {
	return e.Message
}

type simpleMetrics struct{}

func (m *simpleMetrics) IncBooksRequest(result string)              {}
func (m *simpleMetrics) ObserveBooksLatency(duration time.Duration) {}

func NewHTTPBooksClient(base string, timeout time.Duration, logger *slog.Logger) *HTTPBooksClient {
	return NewHTTPBooksClientWithMetrics(base, timeout, logger, &simpleMetrics{})
}

func NewHTTPBooksClientWithMetrics(base string, timeout time.Duration, logger *slog.Logger, metrics BooksMetrics) *HTTPBooksClient {
	return &HTTPBooksClient{
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		base:           base,
		logger:         logger,
		metrics:        metrics,
		threshold:      5,
		cooldownPeriod: 30 * time.Second,
	}
}

func (c *HTTPBooksClient) isCircuitOpen() bool {
	c.circuitMutex.RLock()
	defer c.circuitMutex.RUnlock()

	if !c.circuitOpen {
		return false
	}

	// Check if cooldown period has passed
	if time.Since(c.circuitOpenTime) > c.cooldownPeriod {
		return false // Allow one request to test service health
	}

	return true
}

func (c *HTTPBooksClient) recordSuccess() {
	c.circuitMutex.Lock()
	defer c.circuitMutex.Unlock()

	c.failureCount = 0
	c.circuitOpen = false
}

func (c *HTTPBooksClient) recordFailure() {
	c.circuitMutex.Lock()
	defer c.circuitMutex.Unlock()

	c.failureCount++
	if c.failureCount >= c.threshold {
		c.circuitOpen = true
		c.circuitOpenTime = time.Now()
		c.logger.Warn("Books service circuit breaker opened",
			slog.Int("failure_count", c.failureCount),
			slog.Int("threshold", c.threshold))
	}
}

func (c *HTTPBooksClient) GetBook(ctx context.Context, id int64) (*models.Book, error) {
	start := time.Now()
	defer func() {
		c.metrics.ObserveBooksLatency(time.Since(start))
	}()

	if c.isCircuitOpen() {
		c.metrics.IncBooksRequest("circuit_open")
		return nil, &CircuitBreakerError{Message: "Books service circuit breaker is open"}
	}

	requestID := ctx.Value("request_id")
	if requestID == nil {
		requestID = "unknown"
	}

	url := fmt.Sprintf("%s/v1/books/%d", c.base, id)

	c.logger.InfoContext(ctx, "Calling Books API",
		slog.String("request_id", fmt.Sprintf("%v", requestID)),
		slog.String("method", "GET"),
		slog.String("url", url),
		slog.Int64("book_id", id))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.recordFailure()
		c.metrics.IncBooksRequest("client_error")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "orders-service/1.0")
	if requestID != nil {
		req.Header.Set("X-Request-ID", fmt.Sprintf("%v", requestID))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.recordFailure()
		c.metrics.IncBooksRequest("timeout")
		c.logger.ErrorContext(ctx, "Books API request failed",
			slog.String("request_id", fmt.Sprintf("%v", requestID)),
			slog.String("url", url),
			slog.String("error", err.Error()))
		return nil, &ServiceUnavailableError{Message: "Books service unavailable"}
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	c.logger.InfoContext(ctx, "Books API response received",
		slog.String("request_id", fmt.Sprintf("%v", requestID)),
		slog.String("url", url),
		slog.Int("status", resp.StatusCode),
		slog.Duration("latency", time.Since(start)))

	switch resp.StatusCode {
	case http.StatusOK:
		var book models.Book
		if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
			c.recordFailure()
			c.metrics.IncBooksRequest("client_error")
			c.logger.ErrorContext(ctx, "Failed to decode book response",
				slog.String("request_id", fmt.Sprintf("%v", requestID)),
				slog.String("error", err.Error()))
			return nil, fmt.Errorf("failed to decode book response: %w", err)
		}

		c.recordSuccess()

		// Validate book data and determine status
		if !book.Active {
			c.metrics.IncBooksRequest("inactive")
			return nil, &BookInactiveError{BookID: id}
		}

		c.metrics.IncBooksRequest("active")
		return &book, nil

	case http.StatusNotFound:
		c.recordSuccess() // 404 is a valid response, not a service failure
		c.metrics.IncBooksRequest("not_found")
		return nil, &BookNotFoundError{BookID: id}

	case http.StatusGone: // 410 - book tombstoned
		c.recordSuccess() // 410 is a valid response, not a service failure
		c.metrics.IncBooksRequest("tombstone")
		return nil, &BookInactiveError{BookID: id}

	default:
		if resp.StatusCode >= 500 {
			c.recordFailure()
			c.metrics.IncBooksRequest("upstream_error")
			c.logger.ErrorContext(ctx, "Books API server error",
				slog.String("request_id", fmt.Sprintf("%v", requestID)),
				slog.String("url", url),
				slog.Int("status", resp.StatusCode))
			return nil, &ServiceUnavailableError{Message: fmt.Sprintf("Books service returned status %d", resp.StatusCode)}
		} else {
			c.recordSuccess() // 4xx errors are client errors, not service failures
			c.metrics.IncBooksRequest("client_error")
			c.logger.WarnContext(ctx, "Books API client error",
				slog.String("request_id", fmt.Sprintf("%v", requestID)),
				slog.String("url", url),
				slog.Int("status", resp.StatusCode))
			return nil, fmt.Errorf("books API returned status %d", resp.StatusCode)
		}
	}
}

// GetBooks retrieves multiple books concurrently with limited parallelism
func (c *HTTPBooksClient) GetBooks(ctx context.Context, bookIDs []int64) (map[int64]*models.Book, error) {
	if len(bookIDs) == 0 {
		return make(map[int64]*models.Book), nil
	}

	// Limit concurrency to avoid overwhelming the Books service
	const maxConcurrency = 5
	semaphore := make(chan struct{}, maxConcurrency)

	type result struct {
		id   int64
		book *models.Book
		err  error
	}

	results := make(chan result, len(bookIDs))
	var wg sync.WaitGroup

	for _, id := range bookIDs {
		wg.Add(1)
		go func(bookID int64) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			book, err := c.GetBook(ctx, bookID)
			results <- result{id: bookID, book: book, err: err}
		}(id)
	}

	// Wait for all requests to complete
	wg.Wait()
	close(results)

	books := make(map[int64]*models.Book)
	var firstError error

	for res := range results {
		if res.err != nil {
			if firstError == nil {
				firstError = res.err
			}
			continue
		}
		books[res.id] = res.book
	}

	// If any book failed to load, return the first error
	if firstError != nil {
		return nil, firstError
	}

	return books, nil
}
