package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
)

// Mock service for testing
type MockOrdersService struct {
	mock.Mock
}

func (m *MockOrdersService) CreateOrder(ctx context.Context, req *models.CreateOrderRequest, idempotencyKey string) (*models.Order, error) {
	args := m.Called(ctx, req, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *MockOrdersService) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *MockOrdersService) ListOrders(ctx context.Context) ([]*models.Order, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Order), args.Error(1)
}

func (m *MockOrdersService) ListOrdersPaginated(ctx context.Context, pagination *models.PaginationRequest) (*models.PaginatedResponse[*models.Order], error) {
	args := m.Called(ctx, pagination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PaginatedResponse[*models.Order]), args.Error(1)
}

func TestListOrders_Pagination_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(mockService, logger)

	// Sample orders
	orders := []*models.Order{
		{ID: 1, TotalPrice: "19.99", Items: []models.OrderItem{}},
		{ID: 2, TotalPrice: "29.99", Items: []models.OrderItem{}},
	}

	paginationReq := &models.PaginationRequest{Limit: 50, Offset: 0}
	response := &models.PaginatedResponse[*models.Order]{
		Data:   orders,
		Total:  150,
		Limit:  50,
		Offset: 0,
	}

	mockService.On("ListOrdersPaginated", mock.Anything, paginationReq).Return(response, nil)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/orders?limit=50&offset=0", nil)
	rec := httptest.NewRecorder()

	// Setup Gin context
	router := gin.New()
	router.GET("/v1/orders", handler.ListOrders)
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rec.Code)

	var result models.PaginatedResponse[*models.Order]
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(result.Data))
	assert.Equal(t, 150, result.Total)
	assert.Equal(t, 50, result.Limit)
	assert.Equal(t, 0, result.Offset)

	// Verify headers
	assert.Equal(t, "150", rec.Header().Get("X-Total-Count"))
	linkHeader := rec.Header().Get("Link")
	assert.Contains(t, linkHeader, `rel="next"`)
	assert.Contains(t, linkHeader, "offset=50")

	mockService.AssertExpectations(t)
}

func TestListOrders_Pagination_InvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(mockService, logger)

	// Create request with invalid limit
	req := httptest.NewRequest(http.MethodGet, "/v1/orders?limit=0&offset=0", nil)
	rec := httptest.NewRecorder()

	// Setup Gin context
	router := gin.New()
	router.GET("/v1/orders", handler.ListOrders)
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var result models.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)

	assert.Equal(t, "VALIDATION_ERROR", result.Error)
	assert.Contains(t, result.Message, "Invalid limit parameter")
}

func TestListOrders_Pagination_InvalidOffset(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(mockService, logger)

	// Create request with invalid offset
	req := httptest.NewRequest(http.MethodGet, "/v1/orders?limit=20&offset=-1", nil)
	rec := httptest.NewRecorder()

	// Setup Gin context
	router := gin.New()
	router.GET("/v1/orders", handler.ListOrders)
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var result models.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)

	assert.Equal(t, "VALIDATION_ERROR", result.Error)
	assert.Contains(t, result.Message, "Invalid offset parameter")
}

func TestListOrders_Pagination_LimitCapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(mockService, logger)

	// Sample orders
	orders := []*models.Order{
		{ID: 1, TotalPrice: "19.99", Items: []models.OrderItem{}},
	}

	// Expect capped limit of 200
	paginationReq := &models.PaginationRequest{Limit: 200, Offset: 0}
	response := &models.PaginatedResponse[*models.Order]{
		Data:   orders,
		Total:  1,
		Limit:  200,
		Offset: 0,
	}

	mockService.On("ListOrdersPaginated", mock.Anything, paginationReq).Return(response, nil)

	// Create request with limit > 200
	req := httptest.NewRequest(http.MethodGet, "/v1/orders?limit=500&offset=0", nil)
	rec := httptest.NewRecorder()

	// Setup Gin context
	router := gin.New()
	router.GET("/v1/orders", handler.ListOrders)
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rec.Code)

	var result models.PaginatedResponse[*models.Order]
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)

	// Limit should be capped to 200
	assert.Equal(t, 200, result.Limit)

	mockService.AssertExpectations(t)
}

func TestListOrders_Pagination_EmptyResults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(mockService, logger)

	paginationReq := &models.PaginationRequest{Limit: 50, Offset: 1000}
	response := &models.PaginatedResponse[*models.Order]{
		Data:   []*models.Order{}, // Empty but not nil
		Total:  10,                // Total is still meaningful
		Limit:  50,
		Offset: 1000,
	}

	mockService.On("ListOrdersPaginated", mock.Anything, paginationReq).Return(response, nil)

	// Create request with high offset
	req := httptest.NewRequest(http.MethodGet, "/v1/orders?limit=50&offset=1000", nil)
	rec := httptest.NewRecorder()

	// Setup Gin context
	router := gin.New()
	router.GET("/v1/orders", handler.ListOrders)
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rec.Code)

	var result models.PaginatedResponse[*models.Order]
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)

	assert.Equal(t, 0, len(result.Data)) // Empty array
	assert.NotNil(t, result.Data)        // But not nil
	assert.Equal(t, 10, result.Total)    // Total is still correct

	// Verify headers
	assert.Equal(t, "10", rec.Header().Get("X-Total-Count"))
	linkHeader := rec.Header().Get("Link")
	assert.Contains(t, linkHeader, `rel="prev"`) // Should have prev link

	mockService.AssertExpectations(t)
}

func TestCreateOrder_LocationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(mockService, logger)

	// Sample order response
	order := &models.Order{
		ID:         123,
		TotalPrice: "19.99",
		Items: []models.OrderItem{
			{ID: 1, BookID: 1, Quantity: 1, UnitPrice: "19.99", TotalPrice: "19.99"},
		},
	}

	mockService.On("CreateOrder", mock.Anything, mock.AnythingOfType("*models.CreateOrderRequest"), "").Return(order, nil)

	// Create request
	reqBody := `{"items":[{"book_id":1,"quantity":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Setup Gin context
	router := gin.New()
	router.POST("/v1/orders", handler.CreateOrder)
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Verify Location header
	location := rec.Header().Get("Location")
	assert.Equal(t, "/v1/orders/123", location)

	var result models.Order
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)

	assert.Equal(t, int64(123), result.ID)
	assert.Equal(t, "19.99", result.TotalPrice) // 2dp string

	mockService.AssertExpectations(t)
}

func TestListOrders_Pagination_LinkHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(mockService, logger)

	// Sample orders - middle page
	orders := []*models.Order{
		{ID: 3, TotalPrice: "19.99", Items: []models.OrderItem{}},
		{ID: 4, TotalPrice: "29.99", Items: []models.OrderItem{}},
	}

	paginationReq := &models.PaginationRequest{Limit: 2, Offset: 2}
	response := &models.PaginatedResponse[*models.Order]{
		Data:   orders,
		Total:  10, // Total records
		Limit:  2,
		Offset: 2, // Middle page
	}

	mockService.On("ListOrdersPaginated", mock.Anything, paginationReq).Return(response, nil)

	// Create request - middle page
	req := httptest.NewRequest(http.MethodGet, "/v1/orders?limit=2&offset=2", nil)
	rec := httptest.NewRecorder()

	// Setup Gin context
	router := gin.New()
	router.GET("/v1/orders", handler.ListOrders)
	router.ServeHTTP(rec, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify Link header has both next and prev
	linkHeader := rec.Header().Get("Link")
	assert.Contains(t, linkHeader, `</v1/orders?limit=2&offset=4>; rel="next"`)
	assert.Contains(t, linkHeader, `</v1/orders?limit=2&offset=0>; rel="prev"`)

	mockService.AssertExpectations(t)
}
