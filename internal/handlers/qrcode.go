package handlers

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/storage"
)

// QRCodeHandler generates QR code for a short URL
// @Summary      Generate QR code
// @Description  Generates a QR code PNG image for the short URL
// @Tags         QR Code
// @Produce      image/png
// @Param        code  path      string  true  "Short code"
// @Success      200   {file}    file    "PNG image"
// @Failure      404   {object}  map[string]string  "URL not found"
// @Failure      410   {object}  map[string]string  "Link has expired"
// @Failure      500   {object}  map[string]string  "Server error"
// @Router       /{code}/qr [get]
func QRCodeHandler(c *gin.Context) {
	code := c.Param("code")
	htmlRequest := prefersHTML(c)

	// Check if link exists
	link, err := storage.GetLink(code)
	if err != nil || !link.IsActive {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusNotFound,
				code,
				"QR code unavailable",
				"This QR code cannot be generated because the short link is not available.",
				"Check the link code or ask the link owner to confirm it is still active.",
				nil,
			)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	// Check if expired
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusGone,
				code,
				"QR code unavailable",
				"This link has expired, so its QR code is no longer active.",
				"Time-limited links stop accepting visits and QR scans after expiration.",
				link,
			)
			return
		}
		c.JSON(http.StatusGone, gin.H{"error": "Link has expired"})
		return
	}

	// Try to get from cache first
	cacheKey := "qr:" + code
	cachedQR, err := storage.GetCacheBytes(cacheKey)
	if err == nil {
		c.Data(http.StatusOK, "image/png", cachedQR)
		return
	}

	// Generate QR code
	shortURL := config.AppConfig.BaseURL + "/" + code
	qr, err := qrcode.Encode(shortURL, qrcode.Medium, 256)
	if err != nil {
		logger.Logger.Error("Failed to generate QR code", zap.Error(err), zap.String("code", code))
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusInternalServerError,
				code,
				"QR code unavailable",
				"We could not generate the QR code for this link right now.",
				"Please try again in a moment.",
				link,
			)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
		return
	}

	// Cache QR code for 24 hours
	_ = storage.SetCacheBytes(cacheKey, qr, 24*time.Hour)

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
