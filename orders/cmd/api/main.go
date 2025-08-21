package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	reqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests."},
		[]string{"method", "path", "status"},
	)
	reqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "Request duration."},
		[]string{"method", "path", "status"},
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
		start := time.Now()
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		reqCounter.WithLabelValues(c.Request.Method, path, status).Inc()
		reqDuration.WithLabelValues(c.Request.Method, path, status).
			Observe(time.Since(start).Seconds())
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
