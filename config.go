package main

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

	// JWT
	JWTSecret     string
	JWTExpiration time.Duration

	// AWS
	AWSRegion        string
	DynamoDBEndpoint string

	// Redis
	RedisAddr     string
	RedisPassword string

	// CORS
	AllowedOrigins []string

	// Rate Limiting
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

var AppConfig *Config

func LoadConfig() error {
	// Load .env file if it exists (ignore error in production)
	_ = godotenv.Load()

	jwtExpHours, err := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "168"))
	if err != nil {
		return fmt.Errorf("invalid JWT_EXPIRATION_HOURS: %w", err)
	}

	rateLimitReqs, err := strconv.Atoi(getEnv("RATE_LIMIT_REQUESTS", "100"))
	if err != nil {
		return fmt.Errorf("invalid RATE_LIMIT_REQUESTS: %w", err)
	}

	rateLimitWindow, err := strconv.Atoi(getEnv("RATE_LIMIT_WINDOW_SECONDS", "60"))
	if err != nil {
		return fmt.Errorf("invalid RATE_LIMIT_WINDOW_SECONDS: %w", err)
	}

	AppConfig = &Config{
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),

		JWTSecret:     getEnv("JWT_SECRET", "dev-secret-key"),
		JWTExpiration: time.Duration(jwtExpHours) * time.Hour,

		AWSRegion:        getEnv("AWS_REGION", "us-east-1"),
		DynamoDBEndpoint: getEnv("DYNAMODB_ENDPOINT", ""),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"), ","),

		RateLimitRequests: rateLimitReqs,
		RateLimitWindow:   time.Duration(rateLimitWindow) * time.Second,
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
