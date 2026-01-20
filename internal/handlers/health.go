package handlers

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"patrn.ink/internal/storage"
)

// HealthCheckHandler returns service health status
func HealthCheckHandler(c *gin.Context) {
	// Check Redis connection
	redisOK := storage.PingRedis() == nil

	// Check DynamoDB connection
	dynamoOK := storage.PingDynamo() == nil

	// Get hostname for instance identification
	hostname, _ := os.Hostname()

	health := gin.H{
		"status":    "healthy",
		"redis":     redisOK,
		"dynamodb":  dynamoOK,
		"version":   "1.0.0",
		"hostname":  hostname,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if !redisOK || !dynamoOK {
		health["status"] = "degraded"
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}
