package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RedisRateLimitMiddleware implements distributed rate limiting using Redis
func RedisRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use user_id if authenticated, otherwise IP address
		key := c.ClientIP()
		if userID, exists := c.Get("user_id"); exists {
			key = userID.(string)
		}

		// Redis key for this user/IP
		rateLimitKey := fmt.Sprintf("ratelimit:%s", key)
		// windowKey := fmt.Sprintf("ratelimit:window:%s", key)

		// Get current count
		count, err := rdb.Get(ctx, rateLimitKey).Int64()
		if err != nil && err.Error() != "redis: nil" {
			Logger.Error("Rate limit check failed", zap.Error(err))
			// On error, allow the request (fail open)
			c.Next()
			return
		}

		// Check if limit exceeded
		if count >= int64(AppConfig.RateLimitRequests) {
			// Get TTL to inform user when they can retry
			ttl, _ := rdb.TTL(ctx, rateLimitKey).Result()
			c.AbortWithStatusJSON(429, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(ttl.Seconds()),
			})
			return
		}

		// Increment counter
		pipe := rdb.Pipeline()
		incr := pipe.Incr(ctx, rateLimitKey)

		// Set expiration only if this is the first request in the window
		if count == 0 {
			pipe.Expire(ctx, rateLimitKey, AppConfig.RateLimitWindow)
		}

		_, err = pipe.Exec(ctx)
		if err != nil {
			Logger.Error("Rate limit increment failed", zap.Error(err))
			// On error, allow the request (fail open)
			c.Next()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", AppConfig.RateLimitRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", AppConfig.RateLimitRequests-int(incr.Val())))

		c.Next()
	}
}
