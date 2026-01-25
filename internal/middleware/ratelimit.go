package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
)

// RateLimiter stores rate limit information for each key
type RateLimiter struct {
	requests map[string]*rateLimitEntry
	mu       sync.RWMutex
}

type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

var limiter = &RateLimiter{
	requests: make(map[string]*rateLimitEntry),
}

// cleanup periodically removes expired entries
func init() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			limiter.cleanup()
		}
	}()
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.requests {
		if now.After(entry.resetTime) {
			delete(rl.requests, key)
		}
	}
}

// checkRateLimit checks if the request should be rate limited
func (rl *RateLimiter) checkRateLimit(key string, limit int) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[key]

	if !exists || now.After(entry.resetTime) {
		// Create new entry
		rl.requests[key] = &rateLimitEntry{
			count:     1,
			resetTime: now.Add(time.Minute),
		}
		return true, limit - 1, now.Add(time.Minute)
	}

	if entry.count >= limit {
		return false, 0, entry.resetTime
	}

	entry.count++
	return true, limit - entry.count, entry.resetTime
}

// RateLimitMiddleware creates a rate limiting middleware with the specified limit
func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get rate limit key (user ID for authenticated, IP for anonymous)
		key := c.ClientIP()
		if userID := c.GetString("user_id"); userID != "" {
			key = "user:" + userID
		}

		allowed, remaining, resetTime := limiter.checkRateLimit(key, requestsPerMinute)

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()))

		if !allowed {
			logger.Logger.Warn("Rate limit exceeded",
				zap.String("key", key),
				zap.Int("limit", requestsPerMinute),
			)

			c.Header("Retry-After", fmt.Sprintf("%d", int(time.Until(resetTime).Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(time.Until(resetTime).Seconds()),
			})
			return
		}

		c.Next()
	}
}

// APITokenRateLimitMiddleware applies rate limiting based on API token settings
func APITokenRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this is an API token request
		tokenRateLimit := c.GetInt("token_rate_limit")
		if tokenRateLimit <= 0 {
			// Use default rate limit for JWT auth
			tokenRateLimit = config.AppConfig.DefaultRateLimit
		}

		tokenID := c.GetString("token_id")
		key := "token:" + tokenID
		if tokenID == "" {
			// Fall back to user-based rate limiting
			key = "user:" + c.GetString("user_id")
		}

		allowed, remaining, resetTime := limiter.checkRateLimit(key, tokenRateLimit)

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", tokenRateLimit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()))

		if !allowed {
			logger.Logger.Warn("API token rate limit exceeded",
				zap.String("key", key),
				zap.Int("limit", tokenRateLimit),
			)

			c.Header("Retry-After", fmt.Sprintf("%d", int(time.Until(resetTime).Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(time.Until(resetTime).Seconds()),
			})
			return
		}

		c.Next()
	}
}
