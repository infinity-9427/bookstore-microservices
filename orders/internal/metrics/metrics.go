package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registry = prometheus.NewRegistry()

	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Namespace: "orders", Name: "http_requests_total", Help: "Total HTTP requests."},
		[]string{"method", "route", "status"},
	)
	httpLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Namespace: "orders", Name: "http_request_duration_seconds", Help: "Latency in seconds.", Buckets: prometheus.DefBuckets},
		[]string{"route"},
	)

	BooksCircuitOpens = promauto.With(registry).NewCounter(prometheus.CounterOpts{Namespace: "orders", Name: "books_circuit_open_total", Help: "Times the Books HTTP circuit opened"})
)

func init() {
	registry.MustRegister(httpRequests, httpLatency)
}

// Handler exposes /metrics endpoint
func Handler() gin.HandlerFunc {
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// Middleware instruments requests using route templates (gin full path) and excludes /health & /metrics.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		if path == "/health" || path == "/metrics" {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" { // Unmatched route (404) or no template
			route = "<unmatched>"
		}
		status := c.Writer.Status()
		httpRequests.WithLabelValues(c.Request.Method, route, strconv.Itoa(status)).Inc()
		httpLatency.WithLabelValues(route).Observe(time.Since(start).Seconds())
	}
}

// Exposed for tests
func Registry() *prometheus.Registry { return registry }
