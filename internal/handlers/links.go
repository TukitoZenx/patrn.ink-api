package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/middleware"
	"patrn.ink/internal/models"
	"patrn.ink/internal/shortcode"
	"patrn.ink/internal/storage"
)

// ShortenHandler creates a new short URL
func ShortenHandler(c *gin.Context) {
	var req models.CreateLinkRequest
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
		if !shortcode.IsValidCustomCode(req.CustomCode) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Custom code must be 3-20 alphanumeric characters",
			})
			return
		}

		// Check if custom code already exists
		existing, _ := storage.GetLink(req.CustomCode)
		if existing != nil && existing.IsActive {
			c.JSON(http.StatusConflict, gin.H{"error": "Custom code already in use"})
			return
		}

		shortCode = req.CustomCode
		customAlias = true
	} else {
		// Generate random short code
		var err error
		shortCode, err = storage.GenerateShortCode(7)
		if err != nil {
			logger.Logger.Error("Failed to generate short code", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate short code"})
			return
		}

		// Ensure uniqueness (retry if collision)
		for attempts := 0; attempts < 3; attempts++ {
			existing, _ := storage.GetLink(shortCode)
			if existing == nil || !existing.IsActive {
				break
			}
			shortCode, err = storage.GenerateShortCode(7)
			if err != nil {
				logger.Logger.Error("Failed to generate short code", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate short code"})
				return
			}
		}
		customAlias = false
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &expiry
	}

	// Create link
	link := &models.Link{
		ShortCode:   shortCode,
		LongURL:     req.LongURL,
		UserID:      userID,
		CustomAlias: customAlias,
		Clicks:      0,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
		IsActive:    true,
	}

	if err := storage.SaveLink(link); err != nil {
		logger.Logger.Error("Failed to save link", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create short URL"})
		return
	}

	// Build response
	response := models.CreateLinkResponse{
		ShortURL:  config.AppConfig.BaseURL + "/" + shortCode,
		ShortCode: shortCode,
		LongURL:   req.LongURL,
		QRCodeURL: config.AppConfig.BaseURL + "/" + shortCode + "/qr",
		ExpiresAt: expiresAt,
	}

	logger.Logger.Info("Short URL created",
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
	longURL, err := storage.GetFromCache(code)
	if err == nil {
		// Still need to check if link is active and not expired
		link, _ := storage.GetLink(code)
		if link != nil && link.IsActive {
			if link.ExpiresAt == nil || link.ExpiresAt.After(time.Now()) {
				// Record analytics and increment counter
				RecordAnalytics(c, code)
				_ = storage.IncrementClicks(code)
				middleware.IncRedirects()

				c.Redirect(http.StatusMovedPermanently, longURL)
				return
			}
		}
	}

	// Fallback to database
	link, err := storage.GetLink(code)
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
	_ = storage.IncrementClicks(code)
	middleware.IncRedirects()

	// Refresh cache
	_ = storage.SaveToCache(code, link.LongURL)

	c.Redirect(http.StatusMovedPermanently, link.LongURL)
}

// GetLinksHandler returns all links for the authenticated user
func GetLinksHandler(c *gin.Context) {
	userID := c.GetString("user_id")

	links, err := storage.GetUserLinks(userID)
	if err != nil {
		logger.Logger.Error("Failed to get user links", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve links"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"links": links})
}

// GetLinkDetailsHandler returns details for a specific link
func GetLinkDetailsHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	link, err := storage.GetLink(code)
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

	if err := storage.DeleteLink(code, userID); err != nil {
		if err.Error() == "unauthorized: link belongs to different user" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete link"})
		return
	}

	logger.Logger.Info("Link deleted", zap.String("code", code), zap.String("user", userID))
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
	link, err := storage.GetLink(code)
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
	if err := storage.SaveLink(link); err != nil {
		logger.Logger.Error("Failed to update link", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update link"})
		return
	}

	logger.Logger.Info("Link updated", zap.String("code", code), zap.String("user", userID))
	c.JSON(http.StatusOK, link)
}
