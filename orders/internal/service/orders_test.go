package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"log/slog"
	"os"

	"github.com/yourname/bookstore-microservices/orders/internal/clients"
	"github.com/yourname/bookstore-microservices/orders/internal/config"
	"github.com/yourname/bookstore-microservices/orders/internal/models"
	"github.com/yourname/bookstore-microservices/orders/internal/repository"
)

// Mock implementations
type MockBooksClient struct {
	mock.Mock
}

func (m *MockBooksClient) GetBook(ctx context.Context, bookID int64) (*models.Book, error) {
	args := m.Called(ctx, bookID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Book), args.Error(1)
}

func (m *MockBooksClient) GetBooks(ctx context.Context, bookIDs []int64) (map[int64]*models.Book, error) {
	args := m.Called(ctx, bookIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int64]*models.Book), args.Error(1)
}

type MockOrdersRepository struct {
	mock.Mock
}

func (m *MockOrdersRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockOrdersRepository) CreateOrderWithIdempotency(ctx context.Context, order *models.Order, idempotencyKey string, requestHash string) error {
	args := m.Called(ctx, order, idempotencyKey, requestHash)
	return args.Error(0)
}

func (m *MockOrdersRepository) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *MockOrdersRepository) GetOrderByIdempotencyKey(ctx context.Context, idempotencyKey string) (*models.Order, error) {
	args := m.Called(ctx, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *MockOrdersRepository) CheckIdempotencyKey(ctx context.Context, idempotencyKey string, requestHash string) (*models.Order, error) {
	args := m.Called(ctx, idempotencyKey, requestHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *MockOrdersRepository) ListOrders(ctx context.Context) ([]*models.Order, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Order), args.Error(1)
}

func (m *MockOrdersRepository) ListOrdersPaginated(ctx context.Context, limit, offset int) ([]*models.Order, int, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.Order), args.Get(1).(int), args.Error(2)
}

func TestCreateOrder_ValidMultiBookOrder(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: true} // Enable idempotency for this test
	service := NewOrdersService(mockRepo, mockBooksClient, logger, cfg)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	// Test data
	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 2},
			{BookID: 2, Quantity: 3},
		},
	}

	books := map[int64]*models.Book{
		1: {ID: 1, Title: "Book 1", Author: "Author 1", Price: "19.99", Active: true},
		2: {ID: 2, Title: "Book 2", Author: "Author 2", Price: "24.99", Active: true},
	}

	// Mock expectations
	mockBooksClient.On("GetBooks", ctx, []int64{1, 2}).Return(books, nil)
	mockRepo.On("CreateOrderWithIdempotency", ctx, mock.AnythingOfType("*models.Order"), "", mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			order := args.Get(1).(*models.Order)
			order.ID = 123
			order.CreatedAt = time.Now()
			for i := range order.Items {
				order.Items[i].ID = int64(i + 1)
				order.Items[i].OrderID = 123
				order.Items[i].CreatedAt = time.Now()
			}
		}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, "")

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(123), result.ID)
	assert.Equal(t, "114.95", result.TotalPrice) // 2*19.99 + 3*24.99 = 39.98 + 74.97 = 114.95 (exact decimal arithmetic)
	assert.Len(t, result.Items, 2)

	// Verify calculations
	item1 := result.Items[0]
	assert.Equal(t, int64(1), item1.BookID)
	assert.Equal(t, "Book 1", item1.BookTitle)
	assert.Equal(t, "Author 1", item1.BookAuthor)
	assert.Equal(t, 2, item1.Quantity)
	assert.Equal(t, "19.99", item1.UnitPrice)
	assert.Equal(t, "39.98", item1.TotalPrice) // 2 * 19.99 = 39.98 (exact decimal arithmetic)

	mockRepo.AssertExpectations(t)
	mockBooksClient.AssertExpectations(t)
}

func TestCreateOrder_BookNotFound(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: false} // Disable idempotency for this test
	service := NewOrdersService(mockRepo, mockBooksClient, logger, cfg)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 999, Quantity: 1},
		},
	}

	// Mock expectations
	mockBooksClient.On("GetBooks", ctx, []int64{999}).Return(nil, &clients.BookNotFoundError{BookID: 999})

	// Execute
	result, err := service.CreateOrder(ctx, req, "")

	// Verify
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.IsType(t, &BookNotFoundError{}, err)
	assert.Equal(t, int64(999), err.(*BookNotFoundError).BookID)

	mockBooksClient.AssertExpectations(t)
}

func TestCreateOrder_BookInactive(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewOrdersService(mockRepo, mockBooksClient, logger)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 1},
		},
	}

	// Mock expectations
	mockBooksClient.On("GetBooks", ctx, []int64{1}).Return(nil, &clients.BookInactiveError{BookID: 1})

	// Execute
	result, err := service.CreateOrder(ctx, req, "")

	// Verify
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.IsType(t, &BookNotOrderableError{}, err)
	assert.Equal(t, int64(1), err.(*BookNotOrderableError).BookID)

	mockBooksClient.AssertExpectations(t)
}

func TestCreateOrder_ServiceUnavailable(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewOrdersService(mockRepo, mockBooksClient, logger)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 1},
		},
	}

	// Mock expectations
	mockBooksClient.On("GetBooks", ctx, []int64{1}).Return(nil, &clients.ServiceUnavailableError{Message: "Service timeout"})

	// Execute
	result, err := service.CreateOrder(ctx, req, "")

	// Verify
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.IsType(t, &ServiceUnavailableError{}, err)

	mockBooksClient.AssertExpectations(t)
}

func TestCreateOrder_DuplicateBookIDs(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewOrdersService(mockRepo, mockBooksClient, logger)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	// Request with duplicate book IDs
	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 2},
			{BookID: 1, Quantity: 3}, // Duplicate - should be merged
		},
	}

	books := map[int64]*models.Book{
		1: {ID: 1, Title: "Book 1", Author: "Author 1", Price: "19.99", Active: true},
	}

	// Mock expectations - should only call GetBooks once for unique book ID
	mockBooksClient.On("GetBooks", ctx, []int64{1}).Return(books, nil)
	mockRepo.On("CreateOrderWithIdempotency", ctx, mock.AnythingOfType("*models.Order"), "", mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) {
			order := args.Get(1).(*models.Order)
			// Verify merged quantity
			assert.Len(t, order.Items, 1)
			assert.Equal(t, 5, order.Items[0].Quantity) // 2 + 3 = 5

			order.ID = 123
			order.CreatedAt = time.Now()
			order.Items[0].ID = 1
			order.Items[0].OrderID = 123
			order.Items[0].CreatedAt = time.Now()
		}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, "")

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, 5, result.Items[0].Quantity)
	assert.Equal(t, "99.90", result.Items[0].LineTotal) // 5 * 19.99 with integer cents math

	mockRepo.AssertExpectations(t)
	mockBooksClient.AssertExpectations(t)
}

func TestCreateOrder_IdempotencySameKeyAndBody(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewOrdersService(mockRepo, mockBooksClient, logger)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 1},
		},
	}

	existingOrder := &models.Order{
		ID: 456,
		Items: []models.OrderItem{
			{ID: 1, OrderID: 456, BookID: 1, BookTitle: "Book 1", BookAuthor: "Author 1",
				Quantity: 1, UnitPrice: "19.99", LineTotal: "19.99"},
		},
		TotalAmount: "19.99",
		CreatedAt:   time.Now(),
	}

	// Mock expectations
	mockRepo.On("CheckIdempotencyKey", ctx, "test-key", mock.AnythingOfType("string")).Return(existingOrder, nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, "test-key")

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(456), result.ID)
	assert.Equal(t, "19.99", result.TotalAmount)

	mockRepo.AssertExpectations(t)
	// Books client should not be called since order already exists
	mockBooksClient.AssertNotCalled(t, "GetBooks")
}

func TestCreateOrder_IdempotencyConflict(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewOrdersService(mockRepo, mockBooksClient, logger)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 1},
		},
	}

	// Mock expectations
	mockRepo.On("CheckIdempotencyKey", ctx, "test-key", mock.AnythingOfType("string")).
		Return(nil, &repository.IdempotencyConflictError{Key: "test-key"})

	// Execute
	result, err := service.CreateOrder(ctx, req, "test-key")

	// Verify
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.IsType(t, &IdempotencyConflictError{}, err)
	assert.Equal(t, "test-key", err.(*IdempotencyConflictError).Key)

	mockRepo.AssertExpectations(t)
}

func TestCreateOrder_ValidationError(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewOrdersService(mockRepo, mockBooksClient, logger)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	// Invalid request
	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 0, Quantity: 1}, // Invalid book ID
		},
	}

	// Execute
	result, err := service.CreateOrder(ctx, req, "")

	// Verify
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.IsType(t, &ValidationError{}, err)
	assert.Contains(t, err.Error(), "book_id must be greater than 0")

	// No external calls should be made for validation errors
	mockRepo.AssertNotCalled(t, "CheckIdempotencyKey")
	mockBooksClient.AssertNotCalled(t, "GetBooks")
}
