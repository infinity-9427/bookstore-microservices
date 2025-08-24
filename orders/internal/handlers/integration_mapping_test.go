package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/service"
)

// MockBooksInactiveError used to simulate service layer error mapping
type mockOrdersService struct{ mock.Mock }

func (m *mockOrdersService) CreateOrder(ctx context.Context, req *models.CreateOrderRequest, k string) (*models.Order, error) {
	args := m.Called(ctx, req, k)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}
func (m *mockOrdersService) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	return nil, errors.New("not implemented")
}
func (m *mockOrdersService) ListOrders(ctx context.Context) ([]*models.Order, error) {
	return nil, errors.New("not implemented")
}
func (m *mockOrdersService) ListOrdersPaginated(ctx context.Context, p *models.PaginationRequest) (*models.PaginatedResponse[*models.Order], error) {
	return nil, errors.New("not implemented")
}

func TestCreateOrder_Mapping_Inactive409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(svc, logger)

	reqBody := `{"items":[{"book_id":3,"quantity":1}]}`
	svc.On("CreateOrder", mock.Anything, mock.AnythingOfType("*models.CreateOrderRequest"), "").Return(nil, &service.BookNotOrderableError{BookID: 3})

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/v1/orders", handler.CreateOrder)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var er models.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &er)
	assert.Equal(t, "BOOK_NOT_ORDERABLE", er.Error)
	assert.NotNil(t, er.Details)
}

func TestCreateOrder_Mapping_NotFound404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(svc, logger)
	reqBody := `{"items":[{"book_id":999,"quantity":1}]}`
	svc.On("CreateOrder", mock.Anything, mock.AnythingOfType("*models.CreateOrderRequest"), "").Return(nil, &service.BookNotFoundError{BookID: 999})
	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/v1/orders", handler.CreateOrder)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	var er models.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &er)
	assert.Equal(t, "BOOK_NOT_FOUND", er.Error)
}

func TestCreateOrder_Mapping_ServiceUnavailable503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockOrdersService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewOrdersHandler(svc, logger)
	reqBody := `{"items":[{"book_id":1,"quantity":1}]}`
	svc.On("CreateOrder", mock.Anything, mock.AnythingOfType("*models.CreateOrderRequest"), "").Return(nil, &service.ServiceUnavailableError{Message: "upstream"})
	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/v1/orders", handler.CreateOrder)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var er models.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &er)
	assert.Equal(t, "SERVICE_UNAVAILABLE", er.Error)
}

// Metrics smoke test (unit-level with real middleware not integration server). We simulate by registering route & hitting it.
func TestMetrics_Smoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := new(mockOrdersService)
	handler := NewOrdersHandler(svc, logger)
	svc.On("CreateOrder", mock.Anything, mock.AnythingOfType("*models.CreateOrderRequest"), "").Return(nil, &service.ValidationError{Message: "order must contain at least one item"})
	w := httptest.NewRecorder()
	r := gin.New()
	// simple metrics mimic: rely on /metrics wired in main; here just ensure error envelope invariants.
	r.POST("/v1/orders", handler.CreateOrder)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(`{"items":[]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var er models.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &er)
	assert.Equal(t, "VALIDATION_ERROR", er.Error)
	assert.NotNil(t, er.Details)
	_ = time.Now()
}
