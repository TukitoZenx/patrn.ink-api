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
)

func main() {
	// Load configuration
	if err := LoadConfig(); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := InitLogger(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer Logger.Sync()

	Logger.Info("Starting patrn.ink URL Shortener",
		zap.String("environment", AppConfig.Environment),
		zap.String("port", AppConfig.Port),
	)

	// Initialize connections
	if err := InitRedis(); err != nil {
		Logger.Fatal("Failed to initialize Redis", zap.Error(err))
	}

	if err := InitDynamo(); err != nil {
		Logger.Fatal("Failed to initialize DynamoDB", zap.Error(err))
	}

	// Initialize OAuth
	InitOAuth()

	// Set Gin mode
	if AppConfig.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(RequestIDMiddleware())
	r.Use(LoggingMiddleware())
	r.Use(MetricsMiddleware())
	r.Use(CORSMiddleware())

	// Health check
	r.GET("/health", HealthCheckHandler)

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Authentication routes
	auth := r.Group("/auth")
	{
		auth.GET("/google/login", GoogleLoginHandler)
		auth.GET("/google/callback", GoogleCallbackHandler)
	}

	// Protected API routes (require JWT)
	api := r.Group("/api")
	api.Use(AuthMiddleware())
	api.Use(RedisRateLimitMiddleware())
	{
		api.POST("/shorten", ShortenHandler)
		api.GET("/links", GetLinksHandler)
		api.GET("/links/:code", GetLinkDetailsHandler)
		api.PUT("/links/:code", UpdateLinkHandler)
		api.DELETE("/links/:code", DeleteLinkHandler)
		api.GET("/analytics/:code", GetAnalyticsHandler)
	}

	// Public routes (no auth required)
	r.GET("/:code", RedirectHandler)
	r.GET("/:code/qr", QRCodeHandler)

	// Create server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + AppConfig.Port,
		Handler: r,
	}

	// Start server in goroutine
	go func() {
		Logger.Info("Server started", zap.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	Logger.Info("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		Logger.Error("Server forced to shutdown", zap.Error(err))
	}

	Logger.Info("Server exited")
}
