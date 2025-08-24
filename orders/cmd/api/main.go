package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/clients"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/config"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/handlers"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/logging"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/metrics"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/repository"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/service"
)

func main() {
	logger := logging.NewLogger()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("Starting Orders API",
		slog.String("service", "orders"),
		slog.Int("port", cfg.Port),
		slog.String("books_service_url", cfg.BooksServiceURL),
		slog.Duration("http_timeout", cfg.HTTPTimeout),
		slog.Duration("db_timeout", cfg.DBTimeout),
		slog.Int("circuit_threshold", cfg.CircuitThreshold),
		slog.Bool("idempotency_enabled", cfg.IdempotencyEnabled),
	)

	// Database connection
	dbCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("Failed to parse database configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), dbCfg)
	if err != nil {
		logger.Error("Failed to create database pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	// Books client
	booksClient := clients.NewHTTPBooksClient(cfg.BooksServiceURL, cfg.HTTPTimeout, logger)

	// Repository and service
	ordersRepo := repository.NewOrdersRepository(pool)
	ordersService := service.NewOrdersService(ordersRepo, booksClient, logger, cfg)

	// Handlers
	ordersHandler := handlers.NewOrdersHandler(ordersService, logger)
	healthHandler := handlers.NewHealthHandler(pool, booksClient, logger)

	// Router setup
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(metrics.Middleware())
	r.Use(func(c *gin.Context) {
		// Simple request ID middleware
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		ctx := context.WithValue(c.Request.Context(), "request_id", requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Request-ID", requestID)
		c.Next()
	})

	// Routes
	v1 := r.Group("/v1")
	{
		v1.POST("/orders", ordersHandler.CreateOrder)
		v1.GET("/orders", ordersHandler.ListOrders)
		v1.GET("/orders/:id", ordersHandler.GetOrder)
	}

	r.GET("/health", healthHandler.Health)
	r.GET("/metrics", metrics.Handler())

	// Server setup
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server
	go func() {
		logger.Info("Orders API listening", slog.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("Server exited")
}
