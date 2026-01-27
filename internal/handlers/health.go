package handlers

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"patrn.ink/internal/storage"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status" example:"healthy"`
	Redis     bool   `json:"redis" example:"true"`
	DynamoDB  bool   `json:"dynamodb" example:"true"`
	Version   string `json:"version" example:"1.0.0"`
	Hostname  string `json:"hostname" example:"api-server-1"`
	Timestamp string `json:"timestamp" example:"2026-01-27T10:00:00Z"`
}

// HealthCheckHandler returns service health status
// @Summary      Health check
// @Description  Returns the health status of the API and its dependencies
// @Tags         System
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Failure      503  {object}  HealthResponse
// @Router       /health [get]
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
