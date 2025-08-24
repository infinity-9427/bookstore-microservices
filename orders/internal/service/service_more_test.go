package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/config"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetOrderByID_Service(t *testing.T) {
	mockRepo := new(MockOrdersRepository)
	mockBooks := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: true}
	svc := NewOrdersService(mockRepo, mockBooks, logger, cfg)

	order := &models.Order{ID: 10, TotalPrice: "19.99"}
	mockRepo.On("GetOrderByID", mock.Anything, int64(10)).Return(order, nil)
	mockRepo.On("GetOrderByID", mock.Anything, int64(404)).Return(nil, &repository.OrderNotFoundError{ID: 404})

	got, err := svc.GetOrderByID(context.Background(), 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(10), got.ID)

	_, err = svc.GetOrderByID(context.Background(), 404)
	assert.Error(t, err)
	assert.IsType(t, &OrderNotFoundError{}, err)
}

func TestCreateOrder_IdempotencyReuse(t *testing.T) {
	mockRepo := new(MockOrdersRepository)
	mockBooks := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: true}
	svc := NewOrdersService(mockRepo, mockBooks, logger, cfg)

	existing := &models.Order{ID: 77, TotalPrice: "19.99", Items: []models.OrderItem{{BookID: 1, Quantity: 1, UnitPrice: "19.99", TotalPrice: "19.99"}}}
	mockRepo.On("CheckIdempotencyKey", mock.Anything, "key1", mock.AnythingOfType("string")).Return(existing, nil)

	req := &models.CreateOrderRequest{Items: []models.CreateOrderItemRequest{{BookID: 1, Quantity: 1}}}
	got, err := svc.CreateOrder(context.Background(), req, "key1")
	assert.NoError(t, err)
	assert.Equal(t, int64(77), got.ID)
	// Books client should not be queried
	mockBooks.AssertNotCalled(t, "GetBooks")
}

func TestCreateOrder_IdempotencyRepoError(t *testing.T) {
	mockRepo := new(MockOrdersRepository)
	mockBooks := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: true}
	svc := NewOrdersService(mockRepo, mockBooks, logger, cfg)

	repoErr := errors.New("db down")
	mockRepo.On("CheckIdempotencyKey", mock.Anything, "key2", mock.AnythingOfType("string")).Return(nil, repoErr)

	req := &models.CreateOrderRequest{Items: []models.CreateOrderItemRequest{{BookID: 1, Quantity: 1}}}
	// books validation will be attempted, so mock GetBooks
	mockBooks.On("GetBooks", mock.Anything, []int64{int64(1)}).Return(map[int64]*models.Book{1: {ID: 1, Title: "T", Author: "A", Price: "19.99", Active: true}}, nil)
	mockRepo.On("CreateOrder", mock.Anything, mock.AnythingOfType("*models.Order")).Return(nil)

	// Because repo error was not OrderNotFoundError, service should return InternalError BEFORE creation (per code path)
	_, err := svc.CreateOrder(context.Background(), req, "key2")
	assert.Error(t, err)
	assert.IsType(t, &InternalError{}, err)
}

func TestCreateOrder_IdempotentConflict(t *testing.T) {
	mockRepo := new(MockOrdersRepository)
	mockBooks := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: true}
	svc := NewOrdersService(mockRepo, mockBooks, logger, cfg)

	mockRepo.On("CheckIdempotencyKey", mock.Anything, "key3", mock.AnythingOfType("string")).Return(nil, &repository.IdempotencyConflictError{Key: "key3"})
	req := &models.CreateOrderRequest{Items: []models.CreateOrderItemRequest{{BookID: 1, Quantity: 1}}}
	_, err := svc.CreateOrder(context.Background(), req, "key3")
	assert.Error(t, err)
	assert.IsType(t, &IdempotencyConflictError{}, err)
}

func TestCreateOrder_InternalCreateFailure(t *testing.T) {
	mockRepo := new(MockOrdersRepository)
	mockBooks := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: false}
	svc := NewOrdersService(mockRepo, mockBooks, logger, cfg)

	mockBooks.On("GetBooks", mock.Anything, []int64{int64(1)}).Return(map[int64]*models.Book{1: {ID: 1, Title: "T", Author: "A", Price: "19.99", Active: true}}, nil)
	mockRepo.On("CreateOrder", mock.Anything, mock.AnythingOfType("*models.Order")).Return(errors.New("insert failed"))
	req := &models.CreateOrderRequest{Items: []models.CreateOrderItemRequest{{BookID: 1, Quantity: 1}}}
	_, err := svc.CreateOrder(context.Background(), req, "")
	assert.Error(t, err)
	assert.IsType(t, &InternalError{}, err)
}

// Ensure decimal integrity on multi-item order aggregation
func TestCreateOrder_MultiItemTotals(t *testing.T) {
	mockRepo := new(MockOrdersRepository)
	mockBooks := new(MockBooksClient)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{IdempotencyEnabled: false}
	svc := NewOrdersService(mockRepo, mockBooks, logger, cfg)

	mockBooks.On("GetBooks", mock.Anything, []int64{int64(1), int64(2)}).Return(map[int64]*models.Book{
		1: {ID: 1, Title: "B1", Author: "A1", Price: "19.99", Active: true},
		2: {ID: 2, Title: "B2", Author: "A2", Price: "24.99", Active: true},
	}, nil)
	mockRepo.On("CreateOrder", mock.Anything, mock.AnythingOfType("*models.Order")).Run(func(args mock.Arguments) {
		o := args.Get(1).(*models.Order)
		o.ID = 90
		o.CreatedAt = time.Now()
	}).Return(nil)
	req := &models.CreateOrderRequest{Items: []models.CreateOrderItemRequest{{BookID: 1, Quantity: 2}, {BookID: 2, Quantity: 1}}}
	o, err := svc.CreateOrder(context.Background(), req, "")
	assert.NoError(t, err)
	assert.Equal(t, "64.97", o.TotalPrice) // 2*19.99 + 24.99
}
