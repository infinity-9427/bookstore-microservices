package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

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
	
	// Try to parse as new multi-item order format first
	var req models.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err == nil && len(req.Items) > 0 {
		// New multi-item order format
		order, err := h.service.CreateOrder(ctx, &req)
		if err != nil {
			h.handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusCreated, order)
		return
	}
	
	// Fall back to legacy single-item format for backward compatibility
	var legacyReq models.CreateLegacyOrderRequest
	if err := c.ShouldBindJSON(&legacyReq); err != nil {
		h.respondWithError(c, http.StatusUnprocessableEntity, "Invalid request body", "Request must be either new multi-item format with 'items' array or legacy format with 'book_id' and 'quantity'")
		return
	}
	
	order, err := h.service.CreateLegacyOrder(ctx, &legacyReq)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}
	
	c.JSON(http.StatusCreated, order)
}

// CreateMultiItemOrder handles the new multi-item order format explicitly
func (h *OrdersHandler) CreateMultiItemOrder(c *gin.Context) {
	ctx := c.Request.Context()
	
	var req models.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusUnprocessableEntity, "Invalid request body", err.Error())
		return
	}
	
	order, err := h.service.CreateOrder(ctx, &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}
	
	c.JSON(http.StatusCreated, order)
}

func (h *OrdersHandler) GetOrder(c *gin.Context) {
	ctx := c.Request.Context()
	
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		h.respondWithError(c, http.StatusUnprocessableEntity, "Invalid order ID", "Order ID must be a number")
		return
	}
	
	order, err := h.service.GetOrderByID(ctx, id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}
	
	c.JSON(http.StatusOK, order)
}

func (h *OrdersHandler) ListOrders(c *gin.Context) {
	ctx := c.Request.Context()
	
	var query models.ListOrdersQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		h.respondWithError(c, http.StatusUnprocessableEntity, "Invalid query parameters", err.Error())
		return
	}
	
	orders, err := h.service.ListOrders(ctx, &query)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}
	
	c.JSON(http.StatusOK, orders)
}

func (h *OrdersHandler) handleServiceError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *service.ValidationError:
		h.respondWithError(c, http.StatusUnprocessableEntity, "Validation failed", e.Error())
	case *service.BookNotFoundError:
		h.respondWithError(c, http.StatusNotFound, "Book not found", e.Error())
	case *service.BookNotActiveError:
		h.respondWithError(c, http.StatusConflict, "Book not available", e.Error())
	case *service.OrderNotFoundError:
		h.respondWithError(c, http.StatusNotFound, "Order not found", e.Error())
	case *service.ServiceUnavailableError:
		h.respondWithError(c, http.StatusServiceUnavailable, "Service unavailable", e.Error())
	case *service.InternalError:
		h.respondWithError(c, http.StatusInternalServerError, "Internal server error", e.Error())
	default:
		h.respondWithError(c, http.StatusInternalServerError, "Internal server error", "An unexpected error occurred")
	}
}

func (h *OrdersHandler) respondWithError(c *gin.Context, status int, message, details string) {
	response := models.ErrorResponse{
		Error:   message,
		Details: details,
	}
	c.JSON(status, response)
}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		
		ctx := context.WithValue(c.Request.Context(), "request_id", requestID)
		c.Request = c.Request.WithContext(ctx)
		
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}