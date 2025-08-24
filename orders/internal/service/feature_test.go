package service

import (
	"context"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/config"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/repository"
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

// TestIdempotencyFeatureFlag tests that the idempotency feature flag works correctly
func TestIdempotencyFeatureFlag_Enabled(t *testing.T) {
	// Setup with idempotency enabled
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: true}
	service := NewOrdersService(mockRepo, mockBooksClient, logger, cfg)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 1},
		},
	}

	books := map[int64]*models.Book{
		1: {ID: 1, Title: "Test Book", Author: "Test Author", Price: "19.99", Active: true},
	}

	// Mock expectations - should use idempotency methods
	mockRepo.On("CheckIdempotencyKey", ctx, "test-key", mock.AnythingOfType("string")).
		Return(nil, &repository.OrderNotFoundError{ID: 0}) // Key doesn't exist, so create new order
	mockBooksClient.On("GetBooks", ctx, []int64{1}).Return(books, nil)
	mockRepo.On("CreateOrderWithIdempotency", ctx, mock.AnythingOfType("*models.Order"), "test-key", mock.AnythingOfType("string")).
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

	// Execute with idempotency key
	result, err := service.CreateOrder(ctx, req, "test-key")

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(123), result.ID)
	assert.Equal(t, "19.99", result.TotalPrice)

	mockRepo.AssertExpectations(t)
	mockBooksClient.AssertExpectations(t)
}

func TestIdempotencyFeatureFlag_Disabled(t *testing.T) {
	// Setup with idempotency disabled
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: false}
	service := NewOrdersService(mockRepo, mockBooksClient, logger, cfg)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 1},
		},
	}

	books := map[int64]*models.Book{
		1: {ID: 1, Title: "Test Book", Author: "Test Author", Price: "19.99", Active: true},
	}

	// Mock expectations - should use basic create method, even with idempotency key
	mockBooksClient.On("GetBooks", ctx, []int64{1}).Return(books, nil)
	mockRepo.On("CreateOrder", ctx, mock.AnythingOfType("*models.Order")).
		Run(func(args mock.Arguments) {
			order := args.Get(1).(*models.Order)
			order.ID = 124
			order.CreatedAt = time.Now()
			for i := range order.Items {
				order.Items[i].ID = int64(i + 1)
				order.Items[i].OrderID = 124
				order.Items[i].CreatedAt = time.Now()
			}
		}).Return(nil)

	// Execute with idempotency key - should be ignored
	result, err := service.CreateOrder(ctx, req, "test-key")

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(124), result.ID)
	assert.Equal(t, "19.99", result.TotalPrice)

	mockRepo.AssertExpectations(t)
	mockBooksClient.AssertExpectations(t)
}

func TestListOrdersPaginated(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: true}
	service := NewOrdersService(mockRepo, mockBooksClient, logger, cfg)

	ctx := context.Background()

	// Test data
	orders := []*models.Order{
		{ID: 1, TotalPrice: "19.99", Items: []models.OrderItem{}},
		{ID: 2, TotalPrice: "29.99", Items: []models.OrderItem{}},
	}

	pagination := &models.PaginationRequest{
		Limit:  20,
		Offset: 0,
	}

	// Mock expectations
	mockRepo.On("ListOrdersPaginated", ctx, 20, 0).Return(orders, 150, nil)

	// Execute
	result, err := service.ListOrdersPaginated(ctx, pagination)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result.Data))
	assert.Equal(t, 150, result.Total)
	assert.Equal(t, 20, result.Limit)
	assert.Equal(t, 0, result.Offset)
	assert.Equal(t, int64(1), result.Data[0].ID)
	assert.Equal(t, int64(2), result.Data[1].ID)

	mockRepo.AssertExpectations(t)
}

func TestListOrdersPaginated_EmptyResults(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: true}
	service := NewOrdersService(mockRepo, mockBooksClient, logger, cfg)

	ctx := context.Background()

	pagination := &models.PaginationRequest{
		Limit:  20,
		Offset: 100,
	}

	// Mock expectations - empty result
	mockRepo.On("ListOrdersPaginated", ctx, 20, 100).Return([]*models.Order{}, 0, nil)

	// Execute
	result, err := service.ListOrdersPaginated(ctx, pagination)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result.Data))
	assert.Equal(t, 0, result.Total)
	assert.Equal(t, 20, result.Limit)
	assert.Equal(t, 100, result.Offset)

	mockRepo.AssertExpectations(t)
}

// TestDecimalArithmeticAccuracy tests that the service layer correctly uses exact decimal arithmetic
func TestDecimalArithmeticAccuracy(t *testing.T) {
	// Setup
	mockRepo := new(MockOrdersRepository)
	mockBooksClient := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: false}
	service := NewOrdersService(mockRepo, mockBooksClient, logger, cfg)

	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	// Test the regression case: 19.99 × 1 should equal 19.99
	req := &models.CreateOrderRequest{
		Items: []models.CreateOrderItemRequest{
			{BookID: 1, Quantity: 1},
		},
	}

	books := map[int64]*models.Book{
		1: {ID: 1, Title: "Test Book", Author: "Test Author", Price: "19.99", Active: true},
	}

	// Mock expectations
	mockBooksClient.On("GetBooks", ctx, []int64{1}).Return(books, nil)
	mockRepo.On("CreateOrder", ctx, mock.AnythingOfType("*models.Order")).
		Run(func(args mock.Arguments) {
			order := args.Get(1).(*models.Order)
			order.ID = 125
			order.CreatedAt = time.Now()
			for i := range order.Items {
				order.Items[i].ID = int64(i + 1)
				order.Items[i].OrderID = 125
				order.Items[i].CreatedAt = time.Now()
			}
		}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, "")

	// Verify exact decimal arithmetic
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "19.99", result.TotalPrice, "Order total should be exactly 19.99")
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "19.99", result.Items[0].UnitPrice, "Unit price should be exactly 19.99")
	assert.Equal(t, "19.99", result.Items[0].TotalPrice, "Item total should be exactly 19.99")

	mockRepo.AssertExpectations(t)
	mockBooksClient.AssertExpectations(t)
}
