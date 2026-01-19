package main

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Encode converts a numeric ID to base62 string
func Encode(id uint64) string {
	if id == 0 {
		return string(alphabet[0])
	}

	var sb strings.Builder
	for id > 0 {
		sb.WriteByte(alphabet[id%62])
		id = id / 62
	}

	// Reverse the string
	runes := []rune(sb.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

// ShortenHandler creates a new short URL
func ShortenHandler(c *gin.Context) {
	var req CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := c.GetString("user_id")

	var shortCode string
	var customAlias bool

	// Handle custom code
	if req.CustomCode != "" {
		// Validate custom code (alphanumeric, 3-20 chars)
		if !isValidCustomCode(req.CustomCode) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Custom code must be 3-20 alphanumeric characters",
			})
			return
		}

		// Check if custom code already exists
		existing, _ := GetLink(req.CustomCode)
		if existing != nil && existing.IsActive {
			c.JSON(http.StatusConflict, gin.H{"error": "Custom code already in use"})
			return
		}

		shortCode = req.CustomCode
		customAlias = true
	} else {
		// Generate unique ID
		id, err := GetNextID()
		if err != nil {
			Logger.Error("Failed to generate ID", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate short code"})
			return
		}
		shortCode = Encode(id)
		customAlias = false
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &expiry
	}

	// Create link
	link := &Link{
		ShortCode:   shortCode,
		LongURL:     req.LongURL,
		UserID:      userID,
		CustomAlias: customAlias,
		Clicks:      0,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
		IsActive:    true,
	}

	if err := SaveLink(link); err != nil {
		Logger.Error("Failed to save link", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create short URL"})
		return
	}

	// Build response
	response := CreateLinkResponse{
		ShortURL:  AppConfig.BaseURL + "/" + shortCode,
		ShortCode: shortCode,
		LongURL:   req.LongURL,
		QRCodeURL: AppConfig.BaseURL + "/" + shortCode + "/qr",
		ExpiresAt: expiresAt,
	}

	Logger.Info("Short URL created",
		zap.String("code", shortCode),
		zap.String("user", userID),
		zap.Bool("custom", customAlias),
	)

	c.JSON(http.StatusCreated, response)
}

// RedirectHandler handles short URL redirects
func RedirectHandler(c *gin.Context) {
	code := c.Param("code")

	// Try cache first
	longURL, err := GetFromCache(code)
	if err == nil {
		// Still need to check if link is active and not expired
		link, _ := GetLink(code)
		if link != nil && link.IsActive {
			if link.ExpiresAt == nil || link.ExpiresAt.After(time.Now()) {
				// Record analytics and increment counter
				RecordAnalytics(c, code)
				_ = IncrementClicks(code)
				redirectsTotal.Inc()

				c.Redirect(http.StatusMovedPermanently, longURL)
				return
			}
		}
	}

	// Fallback to database
	link, err := GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	// Check if link is active
	if !link.IsActive {
		c.JSON(http.StatusGone, gin.H{"error": "This link has been deactivated"})
		return
	}

	// Check if expired
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "This link has expired"})
		return
	}

	// Record analytics and increment counter
	RecordAnalytics(c, code)
	_ = IncrementClicks(code)
	redirectsTotal.Inc()

	// Refresh cache
	_ = SaveToCache(code, link.LongURL)

	c.Redirect(http.StatusMovedPermanently, link.LongURL)
}

// GetLinksHandler returns all links for the authenticated user
func GetLinksHandler(c *gin.Context) {
	userID := c.GetString("user_id")

	links, err := GetUserLinks(userID)
	if err != nil {
		Logger.Error("Failed to get user links", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve links"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"links": links})
}

// GetLinkDetailsHandler returns details for a specific link
func GetLinkDetailsHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	link, err := GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
		return
	}

	// Verify ownership
	if link.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	c.JSON(http.StatusOK, link)
}

// DeleteLinkHandler deletes a link
func DeleteLinkHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	if err := DeleteLink(code, userID); err != nil {
		if err.Error() == "unauthorized: link belongs to different user" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete link"})
		return
	}

	Logger.Info("Link deleted", zap.String("code", code), zap.String("user", userID))
	c.JSON(http.StatusOK, gin.H{"message": "Link deleted successfully"})
}

// UpdateLinkHandler updates a link's long URL or expiration
func UpdateLinkHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	var req struct {
		LongURL   string `json:"long_url,omitempty"`
		ExpiresIn int    `json:"expires_in,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get existing link
	link, err := GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
		return
	}

	// Verify ownership
	if link.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	// Update fields
	if req.LongURL != "" {
		link.LongURL = req.LongURL
	}

	if req.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		link.ExpiresAt = &expiry
	}

	// Save updated link
	if err := SaveLink(link); err != nil {
		Logger.Error("Failed to update link", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update link"})
		return
	}

	Logger.Info("Link updated", zap.String("code", code), zap.String("user", userID))
	c.JSON(http.StatusOK, link)
}

// isValidCustomCode validates custom short codes
func isValidCustomCode(code string) bool {
	if len(code) < 3 || len(code) > 20 {
		return false
	}
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", code)
	return matched
}

// HealthCheckHandler returns service health status
func HealthCheckHandler(c *gin.Context) {
	// Check Redis connection
	redisOK := rdb.Ping(ctx).Err() == nil

	// Check DynamoDB connection
	dynamoOK := true
	_, err := ddb.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		dynamoOK = false
	}

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
