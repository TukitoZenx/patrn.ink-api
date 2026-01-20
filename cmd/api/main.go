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
	"go.uber.org/zap"

	"patrn.ink/internal/config"
	"patrn.ink/internal/handlers"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/middleware"
	"patrn.ink/internal/storage"
)

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

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Authentication routes
	auth := r.Group("/auth")
	{
		auth.GET("/google/login", handlers.GoogleLoginHandler)
		auth.GET("/google/callback", handlers.GoogleCallbackHandler)
	}

	// Protected API routes (require JWT) - rate limiting handled by Cloudflare CDN
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())
	{
		api.POST("/shorten", handlers.ShortenHandler)
		api.GET("/links", handlers.GetLinksHandler)
		api.GET("/links/:code", handlers.GetLinkDetailsHandler)
		api.PUT("/links/:code", handlers.UpdateLinkHandler)
		api.DELETE("/links/:code", handlers.DeleteLinkHandler)
		api.GET("/analytics/:code", handlers.GetAnalyticsHandler)
	}

	// Public routes (no auth required)
	r.GET("/:code", handlers.RedirectHandler)
	r.GET("/:code/qr", handlers.QRCodeHandler)

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
