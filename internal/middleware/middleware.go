package middleware

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	redirectsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "redirects_total",
			Help: "Total number of redirects",
		},
	)
)

// IncRedirects increments the redirect counter
func IncRedirects() {
	redirectsTotal.Inc()
}

// LoggingMiddleware logs all HTTP requests
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		logger.Logger.Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("ip", c.ClientIP()),
		)
	}
}

// MetricsMiddleware collects Prometheus metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// CORSMiddleware handles CORS with configured origins
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range config.AppConfig.AllowedOrigins {
			if originMatches(origin, allowedOrigin) {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func originMatches(origin, allowedOrigin string) bool {
	if origin == allowedOrigin || allowedOrigin == "*" {
		return true
	}

	requestURL, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return false
	}

	allowedURL, err := url.Parse(strings.TrimSpace(allowedOrigin))
	if err != nil {
		return false
	}

	if !strings.EqualFold(requestURL.Scheme, allowedURL.Scheme) {
		return false
	}

	if normalizeOriginHost(requestURL.Hostname()) != normalizeOriginHost(allowedURL.Hostname()) {
		return false
	}

	return normalizedOriginPort(requestURL) == normalizedOriginPort(allowedURL)
}

func normalizeOriginHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.TrimPrefix(host, "www.")
}

func normalizedOriginPort(parsed *url.URL) string {
	if parsed.Port() != "" {
		return parsed.Port()
	}

	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}
