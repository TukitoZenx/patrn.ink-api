package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

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
		shortCode, err = storage.GenerateUniqueShortCode(7, 3)
		if err != nil {
			logger.Logger.Error("Failed to generate short code", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate short code"})
			return
		}
		customAlias = false
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &expiry
	}

	// Parse scheduled activation time
	var scheduledAt *time.Time
	if req.ScheduledAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scheduled_at format. Use ISO 8601 (RFC3339)"})
			return
		}
		if parsed.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scheduled_at must be in the future"})
			return
		}
		scheduledAt = &parsed
	}

	// Hash password if provided
	var passwordHash string
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			logger.Logger.Error("Failed to hash password", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
			return
		}
		passwordHash = string(hash)
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
		ScheduledAt: scheduledAt,
		IsActive:    true,
		IsArchived:  false,
		Tags:        req.Tags,
		Password:    passwordHash,
		Title:       req.Title,
		Description: req.Description,
	}

	if err := storage.SaveLink(link); err != nil {
		logger.Logger.Error("Failed to save link", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create short URL"})
		return
	}

	// Build response
	response := models.CreateLinkResponse{
		ShortURL:    config.AppConfig.BaseURL + "/" + shortCode,
		ShortCode:   shortCode,
		LongURL:     req.LongURL,
		QRCodeURL:   config.AppConfig.BaseURL + "/" + shortCode + "/qr",
		ExpiresAt:   expiresAt,
		ScheduledAt: scheduledAt,
		Tags:        req.Tags,
	}

	logger.Logger.Info("Short URL created",
		zap.String("code", shortCode),
		zap.String("user", userID),
		zap.Bool("custom", customAlias),
		zap.Bool("has_password", passwordHash != ""),
		zap.Bool("scheduled", scheduledAt != nil),
	)

	c.JSON(http.StatusCreated, response)
}

// RedirectHandler handles short URL redirects
func RedirectHandler(c *gin.Context) {
	code := c.Param("code")

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

	// Check if archived
	if link.IsArchived {
		c.JSON(http.StatusGone, gin.H{"error": "This link has been archived"})
		return
	}

	// Check if expired
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "This link has expired"})
		return
	}

	// Check if scheduled (not yet active)
	if link.ScheduledAt != nil && link.ScheduledAt.After(time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "This link is not yet active"})
		return
	}

	// Check if password protected
	if link.Password != "" {
		// Return a special response indicating password is required
		c.JSON(http.StatusForbidden, gin.H{
			"error":             "Password required",
			"password_required": true,
			"verify_url":        config.AppConfig.BaseURL + "/" + code + "/verify",
		})
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

// VerifyPasswordHandler verifies password and redirects
func VerifyPasswordHandler(c *gin.Context) {
	code := c.Param("code")

	var req models.PasswordVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is required"})
		return
	}

	link, err := storage.GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	if link.Password == "" {
		// No password required, just redirect
		c.JSON(http.StatusOK, gin.H{"redirect_url": link.LongURL})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(link.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// Password correct - record analytics and return redirect URL
	RecordAnalytics(c, code)
	_ = storage.IncrementClicks(code)
	middleware.IncRedirects()

	c.JSON(http.StatusOK, gin.H{"redirect_url": link.LongURL})
}

// GetLinksHandler returns all links for the authenticated user with search and pagination
func GetLinksHandler(c *gin.Context) {
	userID := c.GetString("user_id")

	var query models.LinksQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	// Set defaults
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	if query.SortBy == "" {
		query.SortBy = "created_at"
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}

	result, err := storage.GetUserLinksWithQuery(userID, &query)
	if err != nil {
		logger.Logger.Error("Failed to get user links", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve links"})
		return
	}

	c.JSON(http.StatusOK, result)
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

// UpdateLinkHandler updates a link's properties
func UpdateLinkHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	var req models.UpdateLinkRequest
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
		// Invalidate cache
		_ = storage.DeleteFromCache(code)
	}

	if req.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		link.ExpiresAt = &expiry
	}

	if req.ScheduledAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scheduled_at format"})
			return
		}
		link.ScheduledAt = &parsed
	}

	if req.Tags != nil {
		link.Tags = req.Tags
	}

	if req.Title != "" {
		link.Title = req.Title
	}

	if req.Description != "" {
		link.Description = req.Description
	}

	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
			return
		}
		link.Password = string(hash)
	}

	if req.IsArchived != nil {
		link.IsArchived = *req.IsArchived
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
