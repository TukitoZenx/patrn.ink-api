package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"patrn.ink/internal/config"
	"patrn.ink/internal/storage"
)

// AuthMiddleware validates JWT token or API token
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Authorization header required"})
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		// Check if it's an API token (starts with ptk_)
		if strings.HasPrefix(tokenString, "ptk_") {
			handleAPITokenAuth(c, tokenString)
			return
		}

		// Otherwise, treat as JWT
		handleJWTAuth(c, tokenString)
	}
}

// handleJWTAuth validates JWT token
func handleJWTAuth(c *gin.Context, tokenString string) {
	// Parse and validate JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.AppConfig.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid or expired token"})
		return
	}

	// Extract claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		c.Set("user_id", claims["user_id"].(string))
		c.Set("email", claims["email"].(string))
		c.Set("auth_type", "jwt")
	} else {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token claims"})
		return
	}

	c.Next()
}

// handleAPITokenAuth validates API token
func handleAPITokenAuth(c *gin.Context, tokenString string) {
	// Hash the token and look it up
	apiToken, err := storage.GetAPITokenByHash(hashAPIToken(tokenString))
	if err != nil {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid or expired API token"})
		return
	}

	// Check if token is active
	if !apiToken.IsActive {
		c.AbortWithStatusJSON(401, gin.H{"error": "API token has been revoked"})
		return
	}

	// Set context values
	c.Set("user_id", apiToken.UserID)
	c.Set("token_id", apiToken.ID)
	c.Set("token_scopes", apiToken.Scopes)
	c.Set("token_rate_limit", apiToken.RateLimit)
	c.Set("auth_type", "api_token")

	c.Next()
}

// hashAPIToken creates a SHA-256 hash of the token
func hashAPIToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ScopeMiddleware checks if the API token has the required scope
func ScopeMiddleware(requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authType := c.GetString("auth_type")

		// JWT tokens have full access
		if authType == "jwt" {
			c.Next()
			return
		}

		// Check API token scopes
		scopes, exists := c.Get("token_scopes")
		if !exists {
			c.AbortWithStatusJSON(403, gin.H{"error": "Insufficient permissions"})
			return
		}

		scopesList, ok := scopes.([]string)
		if !ok {
			c.AbortWithStatusJSON(403, gin.H{"error": "Invalid token scopes"})
			return
		}

		// Check if required scope is present
		hasScope := false
		for _, scope := range scopesList {
			if scope == requiredScope || scope == "*" {
				hasScope = true
				break
			}
			// Check for wildcard scopes (e.g., "links:*" matches "links:read")
			if strings.HasSuffix(scope, ":*") {
				prefix := strings.TrimSuffix(scope, "*")
				if strings.HasPrefix(requiredScope, prefix) {
					hasScope = true
					break
				}
			}
		}

		if !hasScope {
			c.AbortWithStatusJSON(403, gin.H{
				"error":          "Insufficient permissions",
				"required_scope": requiredScope,
			})
			return
		}

		c.Next()
	}
}
