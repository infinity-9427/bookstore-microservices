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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	
	"github.com/yourname/bookstore-microservices/orders/internal/clients"
	"github.com/yourname/bookstore-microservices/orders/internal/config"
	"github.com/yourname/bookstore-microservices/orders/internal/database"
	"github.com/yourname/bookstore-microservices/orders/internal/handlers"
	"github.com/yourname/bookstore-microservices/orders/internal/logging"
	"github.com/yourname/bookstore-microservices/orders/internal/repository"
	"github.com/yourname/bookstore-microservices/orders/internal/service"
)

var (
	reqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests."},
		[]string{"method", "handler", "status_class"},
	)
	reqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request duration in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		},
		[]string{"method", "handler"},
	)
)

func initRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(reqCounter, reqDuration)
	return reg
}

var statusClasses = [6]string{"0xx", "1xx", "2xx", "3xx", "4xx", "5xx"}

func promMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/health" || path == "/metrics" {
			c.Next()
			return
		}
		
		start := time.Now()
		c.Next()
		
		status := c.Writer.Status()
		statusClass := "5xx"
		if status >= 100 && status < 600 {
			statusClass = statusClasses[status/100]
		}
		
		handler := c.FullPath()
		if handler == "" {
			handler = "unknown"
		}
		
		reqCounter.WithLabelValues(c.Request.Method, handler, statusClass).Inc()
		reqDuration.WithLabelValues(c.Request.Method, handler).Observe(time.Since(start).Seconds())
	}
}

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
	)
	
	dbPool, err := database.NewPool(cfg.DatabaseURL, cfg.DBTimeout)
	if err != nil {
		logger.Error("Failed to create database pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbPool.Close()
	
	booksClient := clients.NewBooksClient(
		cfg.BooksServiceURL, 
		cfg.HTTPTimeout, 
		uint32(cfg.CircuitThreshold),
	)
	
	ordersRepo := repository.NewOrdersRepository(dbPool)
	ordersService := service.NewOrdersService(ordersRepo, booksClient, logger)
	
	ordersHandler := handlers.NewOrdersHandler(ordersService, logger)
	healthHandler := handlers.NewHealthHandler(dbPool, booksClient, logger)
	
	reg := initRegistry()
	
	r := gin.New()
	r.Use(
		gin.Recovery(),
		handlers.RequestIDMiddleware(),
		logging.LoggingMiddleware(logger),
		promMiddleware(),
	)
	
	v1 := r.Group("/v1")
	{
		v1.POST("/orders", ordersHandler.CreateOrder)           // Supports both legacy and new formats
		v1.POST("/orders/multi", ordersHandler.CreateMultiItemOrder) // Explicit multi-item format
		v1.GET("/orders", ordersHandler.ListOrders)
		v1.GET("/orders/:id", ordersHandler.GetOrder)
	}
	
	r.GET("/health", healthHandler.Health)
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))
	
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}
	
	go func() {
		logger.Info("Orders API listening", slog.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()
	
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	logger.Info("Shutting down server...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}
	
	logger.Info("Server exited")
}
