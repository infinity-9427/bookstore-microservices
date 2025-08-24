package service

// Centralized test mocks to avoid duplicate type redefinitions across test files.
// NOTE: These mirror the previous mock implementations found in feature_test.go and orders_test.go.

import (
	"context"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
	"github.com/stretchr/testify/mock"
)

// MockBooksClient provides a testify-based mock for the books client used by the service layer.
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

// MockOrdersRepository provides a testify-based mock for the orders repository interface.
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
