package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/models"
	"patrn.ink/internal/storage"
)

var googleOAuthConfig *oauth2.Config
var githubOAuthConfig *oauth2.Config

// InitOAuth initializes OAuth configurations
func InitOAuth() {
	// Google OAuth
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

	// GitHub OAuth
	githubOAuthConfig = &oauth2.Config{
		ClientID:     config.AppConfig.GitHubClientID,
		ClientSecret: config.AppConfig.GitHubClientSecret,
		RedirectURL:  config.AppConfig.GitHubRedirectURL,
		Scopes: []string{
			"read:user",
			"user:email",
		},
		Endpoint: github.Endpoint,
	}
}

// GoogleLoginHandler redirects user to Google OAuth consent page
// @Summary      Google OAuth login
// @Description  Redirects to Google OAuth consent page for authentication
// @Tags         Authentication
// @Success      307  {string}  string  "Redirect to Google OAuth"
// @Router       /auth/google/login [get]
func GoogleLoginHandler(c *gin.Context) {
	// Generate state token for CSRF protection
	state := generateStateToken()

	// Store state in session (using cookie for simplicity)
	setOAuthStateCookie(c, state)

	url := googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallbackHandler handles OAuth callback from Google
// @Summary      Google OAuth callback
// @Description  Handles the OAuth callback from Google, creates/updates user, and returns JWT
// @Tags         Authentication
// @Produce      json
// @Param        state  query     string  true  "OAuth state token"
// @Param        code   query     string  true  "Authorization code"
// @Success      307    {string}  string  "Redirect with JWT token"
// @Failure      400    {object}  map[string]string  "Invalid state token"
// @Failure      500    {object}  map[string]string  "Server error"
// @Router       /auth/google/callback [get]
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
	userInfo, err := getGoogleUserInfo(token.AccessToken)
	if err != nil {
		logger.Logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Create or update user in database
	user := &models.User{
		ID:        "google_" + userInfo["id"].(string),
		Email:     userInfo["email"].(string),
		Name:      userInfo["name"].(string),
		Picture:   userInfo["picture"].(string),
		Provider:  "google",
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

	// Redirect to frontend with token
	redirectURL := config.AppConfig.FrontendURL + "/auth/callback?token=" + jwtToken
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// GitHubLoginHandler redirects user to GitHub OAuth consent page
// @Summary      GitHub OAuth login
// @Description  Redirects to GitHub OAuth consent page for authentication
// @Tags         Authentication
// @Success      307  {string}  string  "Redirect to GitHub OAuth"
// @Failure      501  {object}  map[string]string  "GitHub login not configured"
// @Router       /auth/github/login [get]
func GitHubLoginHandler(c *gin.Context) {
	if config.AppConfig.GitHubClientID == "" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "GitHub login not configured"})
		return
	}

	// Generate state token for CSRF protection
	state := generateStateToken()

	// Store state in session (using cookie for simplicity)
	setOAuthStateCookie(c, state)

	url := githubOAuthConfig.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GitHubCallbackHandler handles OAuth callback from GitHub
// @Summary      GitHub OAuth callback
// @Description  Handles the OAuth callback from GitHub, creates/updates user, and returns JWT
// @Tags         Authentication
// @Produce      json
// @Param        state  query     string  true  "OAuth state token"
// @Param        code   query     string  true  "Authorization code"
// @Success      307    {string}  string  "Redirect with JWT token"
// @Failure      400    {object}  map[string]string  "Invalid state token"
// @Failure      500    {object}  map[string]string  "Server error"
// @Router       /auth/github/callback [get]
func GitHubCallbackHandler(c *gin.Context) {
	// Verify state token
	state := c.Query("state")
	cookieState, err := c.Cookie("oauth_state")
	if err != nil || state != cookieState {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state token"})
		return
	}

	// Exchange authorization code for token
	code := c.Query("code")
	token, err := githubOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Logger.Error("Failed to exchange code for token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authenticate"})
		return
	}

	// Get user info from GitHub
	userInfo, err := getGitHubUserInfo(token.AccessToken)
	if err != nil {
		logger.Logger.Error("Failed to get GitHub user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Get user email (may need separate API call)
	email := ""
	if userInfo["email"] != nil {
		email = userInfo["email"].(string)
	} else {
		// Fetch email from GitHub emails API
		email, _ = getGitHubPrimaryEmail(token.AccessToken)
	}

	// Create or update user in database
	user := &models.User{
		ID:        "github_" + formatGitHubID(userInfo["id"]),
		Email:     email,
		Name:      getStringOrDefault(userInfo["name"], userInfo["login"].(string)),
		Picture:   getStringOrDefault(userInfo["avatar_url"], ""),
		Provider:  "github",
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

	// Redirect to frontend with token
	redirectURL := config.AppConfig.FrontendURL + "/auth/callback?token=" + jwtToken
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// getGoogleUserInfo fetches user information from Google
func getGoogleUserInfo(accessToken string) (map[string]interface{}, error) {
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

// getGitHubUserInfo fetches user information from GitHub
func getGitHubUserInfo(accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
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

// getGitHubPrimaryEmail fetches the primary email from GitHub
func getGitHubPrimaryEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", nil
}

func setOAuthStateCookie(c *gin.Context, state string) {
	secure := config.AppConfig.Environment == "production"
	c.SetCookie("oauth_state", state, 300, "/", "", secure, true)
}

// formatGitHubID formats the GitHub ID (JSON numbers decode as float64)
func formatGitHubID(id interface{}) string {
	switch v := id.(type) {
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprint(v)
	}
}

// getStringOrDefault returns the string value or default if nil/empty
func getStringOrDefault(val interface{}, def string) string {
	if val == nil {
		return def
	}
	if s, ok := val.(string); ok && s != "" {
		return s
	}
	return def
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

// GetCurrentUserHandler returns the current authenticated user
// @Summary      Get current user
// @Description  Returns the currently authenticated user's profile information
// @Tags         User
// @Produce      json
// @Success      200  {object}  models.User
// @Failure      401  {object}  map[string]string  "Unauthorized"
// @Failure      404  {object}  map[string]string  "User not found"
// @Security     BearerAuth
// @Router       /api/me [get]
func GetCurrentUserHandler(c *gin.Context) {
	userID := c.GetString("user_id")

	user, err := storage.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}
