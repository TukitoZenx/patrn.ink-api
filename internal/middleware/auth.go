package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"patrn.ink/internal/config"
)

// AuthMiddleware validates JWT token
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Authorization header required"})
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

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
		} else {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token claims"})
			return
		}

		c.Next()
	}
}
