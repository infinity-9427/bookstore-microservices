package logging

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

const serviceName = "orders"

func NewLogger() *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}

func LoggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		if param.Path == "/health" || param.Path == "/metrics" {
			return ""
		}

		requestID := param.Keys["request_id"]
		if requestID == nil {
			requestID = "unknown"
		}

		handler := param.Keys["handler"]
		if handler == nil {
			handler = param.Path
		}

		logger.Info("HTTP request",
			slog.String("service", serviceName),
			slog.String("method", param.Method),
			slog.String("handler", handler.(string)),
			slog.Int("status", param.StatusCode),
			slog.Duration("latency", param.Latency),
			slog.String("request_id", requestID.(string)),
		)

		return ""
	})
}
