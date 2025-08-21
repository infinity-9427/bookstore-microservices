package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourname/bookstore-microservices/orders/internal/clients"
	"github.com/yourname/bookstore-microservices/orders/internal/database"
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
	
	dbErr := database.HealthCheck(ctx, h.dbPool)
	if dbErr != nil {
		h.logger.ErrorContext(ctx, "Database health check failed", slog.String("error", dbErr.Error()))
		response.Status = "unhealthy"
		response.Services["database"] = "unhealthy"
	} else {
		response.Services["database"] = "healthy"
	}
	
	booksErr := h.booksClient.HealthCheck(ctx)
	if booksErr != nil {
		h.logger.WarnContext(ctx, "Books service health check failed", slog.String("error", booksErr.Error()))
		response.Services["books"] = "unhealthy"
	} else {
		response.Services["books"] = "healthy"
	}
	
	if response.Status == "unhealthy" {
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}
	
	c.JSON(http.StatusOK, response)
}