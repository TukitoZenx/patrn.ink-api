package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/models"
	"patrn.ink/internal/storage"
)

var googleOAuthConfig *oauth2.Config

// InitOAuth initializes Google OAuth configuration
func InitOAuth() {
	googleOAuthConfig = &oauth2.Config{
		ClientID:     config.AppConfig.GoogleClientID,
		ClientSecret: config.AppConfig.GoogleClientSecret,
		RedirectURL:  config.AppConfig.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

// GoogleLoginHandler redirects user to Google OAuth consent page
func GoogleLoginHandler(c *gin.Context) {
	// Generate state token for CSRF protection
	state := generateStateToken()

	// Store state in session (using cookie for simplicity)
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)

	url := googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallbackHandler handles OAuth callback from Google
func GoogleCallbackHandler(c *gin.Context) {
	// Verify state token
	state := c.Query("state")
	cookieState, err := c.Cookie("oauth_state")
	if err != nil || state != cookieState {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state token"})
		return
	}

	// Exchange authorization code for token
	code := c.Query("code")
	token, err := googleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Logger.Error("Failed to exchange code for token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authenticate"})
		return
	}

	// Get user info from Google
	userInfo, err := getUserInfo(token.AccessToken)
	if err != nil {
		logger.Logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Create or update user in database
	user := &models.User{
		ID:        userInfo["id"].(string),
		Email:     userInfo["email"].(string),
		Name:      userInfo["name"].(string),
		Picture:   userInfo["picture"].(string),
		CreatedAt: time.Now(),
	}

	if err := storage.SaveUser(user); err != nil {
		logger.Logger.Error("Failed to save user", zap.Error(err))
	}

	// Generate JWT token
	jwtToken, err := generateJWT(user.ID, user.Email)
	if err != nil {
		logger.Logger.Error("Failed to generate JWT", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Return JWT token and user info
	c.JSON(http.StatusOK, gin.H{
		"token": jwtToken,
		"user":  user,
	})
}

// getUserInfo fetches user information from Google
func getUserInfo(accessToken string) (map[string]interface{}, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return userInfo, nil
}

// generateJWT creates a JWT token for authenticated user
func generateJWT(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(config.AppConfig.JWTExpiration).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}

// generateStateToken generates a random state token for OAuth
func generateStateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
