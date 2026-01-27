package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"patrn.ink/internal/config"
	"patrn.ink/internal/handlers"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/middleware"
	"patrn.ink/internal/storage"

	_ "patrn.ink/docs" // Import generated docs
)

// @title           Patrn.ink URL Shortener API
// @version         1.0
// @description     A powerful URL shortening service with analytics, QR codes, and link management.
// @termsOfService  https://patrn.ink/terms

// @contact.name   API Support
// @contact.url    https://patrn.ink/support
// @contact.email  support@patrn.ink

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token or API token (prefix with "Bearer ")

// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name X-API-Key
// @description API token for programmatic access

func main() {
	// Load configuration
	if err := config.LoadConfig(); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.InitLogger(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Logger.Sync()

	logger.Logger.Info("Starting patrn.ink URL Shortener",
		zap.String("environment", config.AppConfig.Environment),
		zap.String("port", config.AppConfig.Port),
	)

	// Initialize Redis for caching
	if err := storage.InitRedis(); err != nil {
		logger.Logger.Fatal("Failed to initialize Redis", zap.Error(err))
	}

	// Initialize DynamoDB
	if err := storage.InitDynamo(); err != nil {
		logger.Logger.Fatal("Failed to initialize DynamoDB", zap.Error(err))
	}

	// Initialize OAuth
	handlers.InitOAuth()

	// Set Gin mode
	if config.AppConfig.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.LoggingMiddleware())
	r.Use(middleware.MetricsMiddleware())
	r.Use(middleware.CORSMiddleware())

	// Health check
	r.GET("/health", handlers.HealthCheckHandler)

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Authentication routes
	auth := r.Group("/auth")
	{
		// Google OAuth
		auth.GET("/google/login", handlers.GoogleLoginHandler)
		auth.GET("/google/callback", handlers.GoogleCallbackHandler)

		// GitHub OAuth
		auth.GET("/github/login", handlers.GitHubLoginHandler)
		auth.GET("/github/callback", handlers.GitHubCallbackHandler)
	}

	// Protected API routes (require JWT or API token)
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	api.Use(middleware.APITokenRateLimitMiddleware())
	{
		// User info
		api.GET("/me", handlers.GetCurrentUserHandler)

		// Link management
		api.POST("/shorten", middleware.ScopeMiddleware("links:write"), handlers.ShortenHandler)
		api.GET("/links", middleware.ScopeMiddleware("links:read"), handlers.GetLinksHandler)
		api.GET("/links/:code", middleware.ScopeMiddleware("links:read"), handlers.GetLinkDetailsHandler)
		api.PUT("/links/:code", middleware.ScopeMiddleware("links:write"), handlers.UpdateLinkHandler)
		api.DELETE("/links/:code", middleware.ScopeMiddleware("links:write"), handlers.DeleteLinkHandler)

		// Analytics
		api.GET("/analytics/:code", middleware.ScopeMiddleware("analytics:read"), handlers.GetAnalyticsHandler)
		api.GET("/analytics/:code/export", middleware.ScopeMiddleware("analytics:read"), handlers.ExportAnalyticsHandler)

		// Bulk operations
		api.POST("/bulk/delete", middleware.ScopeMiddleware("bulk:write"), handlers.BulkDeleteHandler)
		api.POST("/bulk/import", middleware.ScopeMiddleware("bulk:write"), handlers.BulkImportHandler)
		api.GET("/export/links", middleware.ScopeMiddleware("bulk:read"), handlers.ExportLinksHandler)

		// API Token management (JWT only - no API token scope check)
		tokens := api.Group("/tokens")
		{
			tokens.POST("", handlers.CreateAPITokenHandler)
			tokens.GET("", handlers.ListAPITokensHandler)
			tokens.DELETE("/:id", handlers.RevokeAPITokenHandler)
			tokens.PUT("/:id/rate-limit", handlers.UpdateAPITokenRateLimitHandler)
		}
	}

	// Public routes (no auth required)
	// Link preview (can be used without auth for URL metadata)
	r.GET("/api/preview", handlers.LinkPreviewHandler)

	// Password verification for protected links
	r.POST("/:code/verify", handlers.VerifyPasswordHandler)

	// Age verification for adult content
	r.POST("/:code/verify-age", handlers.VerifyAgeHandler)

	// Link preview by code
	r.GET("/:code/preview", handlers.GetLinkPreviewByCodeHandler)

	// QR code
	r.GET("/:code/qr", handlers.QRCodeHandler)

	// Redirect (must be last to avoid conflicts)
	r.GET("/:code", handlers.RedirectHandler)

	// Create server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + config.AppConfig.Port,
		Handler: r,
	}

	// Start server in goroutine
	go func() {
		logger.Logger.Info("Server started", zap.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Logger.Info("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Logger.Info("Server exited")
}
