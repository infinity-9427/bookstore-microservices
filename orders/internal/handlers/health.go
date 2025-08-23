package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourname/bookstore-microservices/orders/internal/clients"
)

type HealthHandler struct {
	dbPool      *pgxpool.Pool
	booksClient clients.BooksClient
	logger      *slog.Logger
}

type HealthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

func NewHealthHandler(dbPool *pgxpool.Pool, booksClient clients.BooksClient, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		dbPool:      dbPool,
		booksClient: booksClient,
		logger:      logger,
	}
}

func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status:   "healthy",
		Services: make(map[string]string),
	}

	// Simple database ping
	if err := h.dbPool.Ping(ctx); err != nil {
		h.logger.ErrorContext(ctx, "Database health check failed", slog.String("error", err.Error()))
		response.Status = "unhealthy"
		response.Services["database"] = "unhealthy"
	} else {
		response.Services["database"] = "healthy"
	}

	// Simple books service check by trying to get a non-existent book
	if _, err := h.booksClient.GetBook(ctx, 99999); err != nil {
		// We expect this to fail with "not found", but if it's a connection error, mark as unhealthy
		if err.Error() != "book with ID 99999 not found" {
			h.logger.WarnContext(ctx, "Books service health check failed", slog.String("error", err.Error()))
			response.Services["books"] = "unhealthy"
		} else {
			response.Services["books"] = "healthy"
		}
	} else {
		response.Services["books"] = "healthy"
	}

	if response.Status == "unhealthy" {
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	c.JSON(http.StatusOK, response)
}
