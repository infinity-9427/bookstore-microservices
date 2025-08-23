package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourname/bookstore-microservices/orders/internal/models"
	"github.com/yourname/bookstore-microservices/orders/internal/service"
)

type OrdersHandler struct {
	service service.OrdersService
	logger  *slog.Logger
}

func NewOrdersHandler(service service.OrdersService, logger *slog.Logger) *OrdersHandler {
	return &OrdersHandler{
		service: service,
		logger:  logger,
	}
}

func (h *OrdersHandler) CreateOrder(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract idempotency key from header
	idempotencyKey := c.GetHeader("Idempotency-Key")

	var req models.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body: "+err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		h.respondWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return
	}

	order, err := h.service.CreateOrder(ctx, &req, idempotencyKey)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/v1/orders/%d", order.ID))
	c.JSON(http.StatusCreated, order)
}

func (h *OrdersHandler) ListOrders(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse and validate pagination parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		h.respondWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid limit parameter")
		return
	}
	if limit > 200 {
		limit = 200 // Cap to maximum
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		h.respondWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid offset parameter")
		return
	}

	pagination := &models.PaginationRequest{
		Limit:  limit,
		Offset: offset,
	}

	// Use pagination
	response, err := h.service.ListOrdersPaginated(ctx, pagination)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	// Set pagination headers
	baseURL := "/v1/orders"
	
	// RFC5988 Link header
	var links []string
	if response.Offset + response.Limit < response.Total {
		nextOffset := response.Offset + response.Limit
		links = append(links, fmt.Sprintf("<%s?limit=%d&offset=%d>; rel=\"next\"", baseURL, response.Limit, nextOffset))
	}
	if response.Offset > 0 {
		prevOffset := response.Offset - response.Limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, fmt.Sprintf("<%s?limit=%d&offset=%d>; rel=\"prev\"", baseURL, response.Limit, prevOffset))
	}
	
	if len(links) > 0 {
		c.Header("Link", strings.Join(links, ", "))
	}
	
	// X-Total-Count header for simple client use
	c.Header("X-Total-Count", fmt.Sprintf("%d", response.Total))

	c.JSON(http.StatusOK, response)
}

func (h *OrdersHandler) GetOrder(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid order ID")
		return
	}

	order, err := h.service.GetOrderByID(ctx, id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, order)
}

func (h *OrdersHandler) handleServiceError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *service.ValidationError:
		h.respondWithError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", e.Error())
	case *service.BookNotFoundError:
		h.respondWithError(c, http.StatusNotFound, "BOOK_NOT_FOUND", e.Error())
	case *service.BookNotOrderableError:
		h.respondWithError(c, http.StatusConflict, "BOOK_NOT_ORDERABLE", e.Error())
	case *service.OrderNotFoundError:
		h.respondWithError(c, http.StatusNotFound, "ORDER_NOT_FOUND", e.Error())
	case *service.ServiceUnavailableError:
		h.respondWithError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", e.Error())
	case *service.IdempotencyConflictError:
		h.respondWithError(c, http.StatusConflict, "IDEMPOTENCY_CONFLICT", e.Error())
	default:
		h.logger.ErrorContext(c.Request.Context(), "Unhandled service error",
			slog.String("error", err.Error()),
			slog.String("type", fmt.Sprintf("%T", err)))
		h.respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
	}
}

func (h *OrdersHandler) respondWithError(c *gin.Context, status int, errorCode, message string) {
	response := models.ErrorResponse{
		Error:   errorCode,
		Message: message,
	}
	c.JSON(status, response)
}
