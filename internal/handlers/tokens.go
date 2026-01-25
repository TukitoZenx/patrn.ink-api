package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"patrn.ink/internal/logger"
	"patrn.ink/internal/models"
	"patrn.ink/internal/storage"
)

// ValidScopes defines all valid API token scopes
var ValidScopes = map[string]bool{
	"links:read":     true,
	"links:write":    true,
	"analytics:read": true,
	"bulk:read":      true,
	"bulk:write":     true,
}

// generateAPIToken generates a secure random API token
func generateAPIToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ptk_" + hex.EncodeToString(bytes), nil
}

// hashToken creates a SHA-256 hash of the token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// CreateAPITokenHandler creates a new personal API token
func CreateAPITokenHandler(c *gin.Context) {
	var req models.CreateAPITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := c.GetString("user_id")

	// Validate scopes
	for _, scope := range req.Scopes {
		if !ValidScopes[scope] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scope: " + scope})
			return
		}
	}

	// Generate token
	rawToken, err := generateAPIToken()
	if err != nil {
		logger.Logger.Error("Failed to generate API token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Generate token ID
	tokenID, err := storage.GenerateShortCode(12)
	if err != nil {
		logger.Logger.Error("Failed to generate token ID", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &expiry
	}

	// Create token record
	apiToken := &models.APIToken{
		ID:          tokenID,
		UserID:      userID,
		Name:        req.Name,
		TokenHash:   hashToken(rawToken),
		TokenPrefix: rawToken[:12], // "ptk_" + first 8 chars
		Scopes:      req.Scopes,
		RateLimit:   100, // Default rate limit
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
		IsActive:    true,
	}

	if err := storage.SaveAPIToken(apiToken); err != nil {
		logger.Logger.Error("Failed to save API token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		return
	}

	logger.Logger.Info("API token created",
		zap.String("token_id", tokenID),
		zap.String("user", userID),
		zap.Strings("scopes", req.Scopes),
	)

	c.JSON(http.StatusCreated, models.CreateAPITokenResponse{
		Token:    rawToken,
		APIToken: apiToken,
	})
}

// ListAPITokensHandler returns all API tokens for the authenticated user
func ListAPITokensHandler(c *gin.Context) {
	userID := c.GetString("user_id")

	tokens, err := storage.GetUserAPITokens(userID)
	if err != nil {
		logger.Logger.Error("Failed to get API tokens", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

// RevokeAPITokenHandler revokes an API token
func RevokeAPITokenHandler(c *gin.Context) {
	tokenID := c.Param("id")
	userID := c.GetString("user_id")

	if err := storage.RevokeAPIToken(tokenID, userID); err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		if err.Error() == "token not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
			return
		}
		logger.Logger.Error("Failed to revoke API token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke token"})
		return
	}

	logger.Logger.Info("API token revoked", zap.String("token_id", tokenID), zap.String("user", userID))
	c.JSON(http.StatusOK, gin.H{"message": "Token revoked successfully"})
}

// UpdateAPITokenRateLimitHandler updates the rate limit for an API token
func UpdateAPITokenRateLimitHandler(c *gin.Context) {
	tokenID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		RateLimit int `json:"rate_limit" binding:"required,min=1,max=1000"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := storage.UpdateAPITokenRateLimit(tokenID, userID, req.RateLimit); err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		logger.Logger.Error("Failed to update token rate limit", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rate limit updated"})
}

// ValidateAPIToken validates an API token and returns the token record
func ValidateAPIToken(rawToken string) (*models.APIToken, error) {
	tokenHash := hashToken(rawToken)
	return storage.GetAPITokenByHash(tokenHash)
}
