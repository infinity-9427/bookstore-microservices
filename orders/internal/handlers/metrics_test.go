package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/metrics"
)

// minimal router for metrics test
func setupMetricsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Install metrics middleware so counters/histograms get created when we hit a route
	r.Use(metrics.Middleware())

	// Instrumented sample route (mimics a real API endpoint)
	r.GET("/v1/orders", func(c *gin.Context) {
		c.JSON(http.StatusOK, []any{})
	})

	// Expose metrics using the same registry the middleware uses
	r.GET("/metrics", metrics.Handler())
	return r
}

func TestMetricsEndpoint(t *testing.T) {
	r := setupMetricsRouter()

	// Warm up by hitting an instrumented route so metric vectors create a child
	wRoute := httptest.NewRecorder()
	reqRoute, _ := http.NewRequest(http.MethodGet, "/v1/orders?limit=1&offset=0", nil)
	r.ServeHTTP(wRoute, reqRoute)
	if wRoute.Code != http.StatusOK {
		t.Fatalf("warm-up route expected 200 got %d", wRoute.Code)
	}

	// Now scrape /metrics
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	body := w.Body.String()
	// Assert presence of metric families (names only)
	if !strings.Contains(body, "orders_http_requests_total") {
		t.Fatalf("expected orders_http_requests_total metric; body=\n%s", body)
	}
	if !strings.Contains(body, "orders_http_request_duration_seconds") { // histogram family
		t.Fatalf("expected orders_http_request_duration_seconds metric; body=\n%s", body)
	}
}
