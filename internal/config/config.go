package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
	BaseURL     string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// GitHub OAuth
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string

	// JWT
	JWTSecret     string
	JWTExpiration time.Duration

	// AWS
	AWSRegion        string
	DynamoDBEndpoint string

	// Redis (for caching)
	RedisAddr     string
	RedisPassword string

	// CORS
	AllowedOrigins []string

	// Rate Limiting
	DefaultRateLimit  int // Requests per minute for JWT auth
	APITokenRateLimit int // Default rate limit for API tokens

	// Frontend URL (for OAuth redirects)
	FrontendURL string
}

func Load() *Config {
	return AppConfig
}

var AppConfig *Config

func LoadConfig() error {
	// Load .env file if it exists (ignore error in production)
	_ = godotenv.Load()

	jwtExpHours, err := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "168"))
	if err != nil {
		return fmt.Errorf("invalid JWT_EXPIRATION_HOURS: %w", err)
	}

	defaultRateLimit, _ := strconv.Atoi(getEnv("DEFAULT_RATE_LIMIT", "60"))
	apiTokenRateLimit, _ := strconv.Atoi(getEnv("API_TOKEN_RATE_LIMIT", "100"))

	AppConfig = &Config{
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),

		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURL:  getEnv("GITHUB_REDIRECT_URL", "http://localhost:8080/auth/github/callback"),

		JWTSecret:     getEnv("JWT_SECRET", "dev-secret-key"),
		JWTExpiration: time.Duration(jwtExpHours) * time.Hour,

		AWSRegion:        getEnv("AWS_REGION", "us-east-1"),
		DynamoDBEndpoint: getEnv("DYNAMODB_ENDPOINT", ""),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"), ","),

		DefaultRateLimit:  defaultRateLimit,
		APITokenRateLimit: apiTokenRateLimit,

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
	}

	// Validate required fields in production
	if AppConfig.Environment == "production" {
		if AppConfig.GoogleClientID == "" || AppConfig.GoogleClientSecret == "" {
			return fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required in production")
		}
		if AppConfig.JWTSecret == "dev-secret-key" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
