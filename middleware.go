package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
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

// LoggingMiddleware logs all HTTP requests
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		Logger.Info("HTTP Request",
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

// RateLimitMiddleware implements token bucket rate limiting per user/IP
func RateLimitMiddleware() gin.HandlerFunc {
	type bucket struct {
		tokens     int
		lastRefill time.Time
		mu         sync.Mutex
	}

	buckets := make(map[string]*bucket)
	bucketsLock := sync.RWMutex{}

	return func(c *gin.Context) {
		// Use user_id if authenticated, otherwise IP address
		key := c.ClientIP()
		if userID, exists := c.Get("user_id"); exists {
			key = userID.(string)
		}

		bucketsLock.Lock()
		b, exists := buckets[key]
		if !exists {
			b = &bucket{
				tokens:     AppConfig.RateLimitRequests,
				lastRefill: time.Now(),
			}
			buckets[key] = b
		}
		bucketsLock.Unlock()

		b.mu.Lock()
		defer b.mu.Unlock()

		// Refill tokens based on elapsed time
		now := time.Now()
		elapsed := now.Sub(b.lastRefill)
		if elapsed >= AppConfig.RateLimitWindow {
			b.tokens = AppConfig.RateLimitRequests
			b.lastRefill = now
		}

		// Check if request is allowed
		if b.tokens <= 0 {
			c.AbortWithStatusJSON(429, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": AppConfig.RateLimitWindow.Seconds(),
			})
			return
		}

		b.tokens--
		c.Next()
	}
}

// CORSMiddleware handles CORS with configured origins
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range AppConfig.AllowedOrigins {
			if origin == allowedOrigin || allowedOrigin == "*" {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
