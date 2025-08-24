package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
	svc "github.com/infinity-9427/bookstore-microservices/orders/internal/service"
	"github.com/stretchr/testify/assert"
)

// fakeService implements svc.OrdersService for handler tests
type fakeService struct {
	createFn        func(ctx context.Context, req *models.CreateOrderRequest, key string) (*models.Order, error)
	getFn           func(ctx context.Context, id int64) (*models.Order, error)
	listPaginatedFn func(ctx context.Context, p *models.PaginationRequest) (*models.PaginatedResponse[*models.Order], error)
}

func (f *fakeService) CreateOrder(ctx context.Context, req *models.CreateOrderRequest, key string) (*models.Order, error) {
	return f.createFn(ctx, req, key)
}
func (f *fakeService) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	return f.getFn(ctx, id)
}
func (f *fakeService) ListOrders(ctx context.Context) ([]*models.Order, error) {
	return []*models.Order{}, nil
}
func (f *fakeService) ListOrdersPaginated(ctx context.Context, p *models.PaginationRequest) (*models.PaginatedResponse[*models.Order], error) {
	return f.listPaginatedFn(ctx, p)
}

func newTestRouter(svcImpl svc.OrdersService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOrdersHandler(svcImpl, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	v1 := r.Group("/v1")
	v1.POST("/orders", h.CreateOrder)
	v1.GET("/orders/:id", h.GetOrder)
	v1.GET("/orders", h.ListOrders)
	return r
}

func TestCreateOrder_ValidationError(t *testing.T) {
	fs := &fakeService{createFn: func(ctx context.Context, req *models.CreateOrderRequest, key string) (*models.Order, error) {
		t.Fatalf("service should not be called on validation error")
		return nil, nil
	}}
	router := newTestRouter(fs)
	w := httptest.NewRecorder()
	reqBody := bytes.NewBufferString(`{"items":[]}`) // invalid: empty items
	req, _ := http.NewRequest("POST", "/v1/orders", reqBody)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var er models.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &er)
	assert.Equal(t, "VALIDATION_ERROR", er.Error)
}

func TestCreateOrder_Success(t *testing.T) {
	fs := &fakeService{createFn: func(ctx context.Context, req *models.CreateOrderRequest, key string) (*models.Order, error) {
		return &models.Order{ID: 123, TotalPrice: "19.99"}, nil
	}}
	router := newTestRouter(fs)
	w := httptest.NewRecorder()
	reqBody := bytes.NewBufferString(`{"items":[{"book_id":1,"quantity":1}]}`)
	req, _ := http.NewRequest("POST", "/v1/orders", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "abc")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "/v1/orders/123", w.Header().Get("Location"))
}

func TestGetOrder_NotFound(t *testing.T) {
	fs := &fakeService{getFn: func(ctx context.Context, id int64) (*models.Order, error) {
		return nil, &svc.OrderNotFoundError{ID: id}
	}, createFn: func(context.Context, *models.CreateOrderRequest, string) (*models.Order, error) { return nil, nil }, listPaginatedFn: func(ctx context.Context, p *models.PaginationRequest) (*models.PaginatedResponse[*models.Order], error) {
		return &models.PaginatedResponse[*models.Order]{Data: []*models.Order{}, Total: 0, Limit: p.Limit, Offset: p.Offset}, nil
	}}
	router := newTestRouter(fs)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/orders/999", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListOrders_PaginationHeaders(t *testing.T) {
	fs := &fakeService{listPaginatedFn: func(ctx context.Context, p *models.PaginationRequest) (*models.PaginatedResponse[*models.Order], error) {
		return &models.PaginatedResponse[*models.Order]{Data: []*models.Order{}, Total: 120, Limit: p.Limit, Offset: p.Offset}, nil
	}, createFn: func(context.Context, *models.CreateOrderRequest, string) (*models.Order, error) { return nil, nil }, getFn: func(context.Context, int64) (*models.Order, error) { return nil, &svc.OrderNotFoundError{ID: 1} }}
	router := newTestRouter(fs)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/orders?limit=50&offset=50", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	// Expect next and prev links
	link := w.Header().Get("Link")
	assert.Contains(t, link, "rel=\"next\"")
	assert.Contains(t, link, "rel=\"prev\"")
	assert.Equal(t, "120", w.Header().Get("X-Total-Count"))
}
