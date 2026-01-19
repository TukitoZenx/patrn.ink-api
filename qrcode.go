package main

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"
)

// QRCodeHandler generates QR code for a short URL
func QRCodeHandler(c *gin.Context) {
	code := c.Param("code")

	// Check if link exists
	link, err := GetLink(code)
	if err != nil || !link.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	// Check if expired
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "Link has expired"})
		return
	}

	// Try to get from cache first
	cacheKey := "qr:" + code
	cachedQR, err := rdb.Get(ctx, cacheKey).Bytes()
	if err == nil {
		c.Data(http.StatusOK, "image/png", cachedQR)
		return
	}

	// Generate QR code
	shortURL := AppConfig.BaseURL + "/" + code
	qr, err := qrcode.Encode(shortURL, qrcode.Medium, 256)
	if err != nil {
		Logger.Error("Failed to generate QR code", zap.Error(err), zap.String("code", code))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
		return
	}

	// Cache QR code for 24 hours
	_ = rdb.Set(ctx, cacheKey, qr, 24*time.Hour).Err()

	c.Data(http.StatusOK, "image/png", qr)
}

// GenerateQRCodeBytes generates QR code as bytes (helper function)
func GenerateQRCodeBytes(shortURL string) ([]byte, error) {
	var buf bytes.Buffer
	qr, err := qrcode.Encode(shortURL, qrcode.Medium, 256)
	if err != nil {
		return nil, err
	}
	buf.Write(qr)
	return buf.Bytes(), nil
}
