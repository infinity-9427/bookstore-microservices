package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	reqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests."},
		[]string{"method", "path", "status_class"},
	)
	reqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request duration in seconds.",
			Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
		},
		[]string{"method", "path"},
	)
)

func initRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(reqCounter, reqDuration)
	return reg
}

func promMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip metrics for health/metrics endpoints
		path := c.Request.URL.Path
		if path == "/health" || path == "/metrics" {
			c.Next()
			return
		}
		
		start := time.Now()
		c.Next()
		
		status := c.Writer.Status()
		statusClass := fmt.Sprintf("%dxx", status/100)
		routePath := c.FullPath()
		if routePath == "" {
			routePath = "unknown"
		}
		
		reqCounter.WithLabelValues(c.Request.Method, routePath, statusClass).Inc()
		reqDuration.WithLabelValues(c.Request.Method, routePath).Observe(time.Since(start).Seconds())
	}
}

func main() {
	reg := initRegistry()

	r := gin.New()
	r.Use(gin.Recovery(), promMiddleware())

	// health
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// your existing routes...
	// r.POST("/orders", createOrder)
	// r.GET("/orders", listOrders)
	// r.GET("/orders/:id", getOrder)

	// /metrics for Prometheus
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))

	log.Println("Orders API listening on :8082")
	if err := r.Run(":8082"); err != nil {
		log.Fatal(err)
	}
}
