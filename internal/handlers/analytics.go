package handlers

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"patrn.ink/internal/logger"
	"patrn.ink/internal/models"
	"patrn.ink/internal/storage"
)

// RecordAnalytics records click analytics asynchronously
func RecordAnalytics(c *gin.Context, shortCode string) {
	// Run asynchronously to not block redirect
	go func() {
		event := &models.AnalyticsEvent{
			ShortCode: shortCode,
			Timestamp: time.Now(),
			Referrer:  c.Request.Referer(),
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
			Country:   extractCountryFromIP(c.ClientIP()),
		}

		if err := storage.SaveAnalyticsEvent(event); err != nil {
			logger.Logger.Error("Failed to save analytics", zap.Error(err))
		}
	}()
}

// extractCountryFromIP extracts country from IP (placeholder - would use GeoIP in production)
func extractCountryFromIP(ip string) string {
	// In production, use MaxMind GeoIP or similar service
	// For now, return "Unknown"
	return "Unknown"
}

// GetAnalyticsHandler returns analytics for a specific link
func GetAnalyticsHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	// Verify ownership
	link, err := storage.GetLink(code)
	if err != nil {
		c.JSON(404, gin.H{"error": "Link not found"})
		return
	}

	if link.UserID != userID {
		c.JSON(403, gin.H{"error": "Unauthorized"})
		return
	}

	// In a real implementation, you'd query the Analytics table
	// and aggregate the data. For now, return basic info
	summary := &models.AnalyticsSummary{
		TotalClicks:  link.Clicks,
		UniqueClicks: link.Clicks, // Would need distinct count from Analytics table
		TopReferrers: make(map[string]int64),
		ClicksByDate: make(map[string]int64),
		DeviceTypes:  make(map[string]int64),
		BrowserTypes: make(map[string]int64),
	}

	c.JSON(200, summary)
}

// detectDeviceType detects device type from user agent
func detectDeviceType(userAgent string) string {
	ua := strings.ToLower(userAgent)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") {
		return "Mobile"
	} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		return "Tablet"
	}
	return "Desktop"
}

// detectBrowser detects browser from user agent
func detectBrowser(userAgent string) string {
	ua := strings.ToLower(userAgent)
	if strings.Contains(ua, "chrome") {
		return "Chrome"
	} else if strings.Contains(ua, "firefox") {
		return "Firefox"
	} else if strings.Contains(ua, "safari") {
		return "Safari"
	} else if strings.Contains(ua, "edge") {
		return "Edge"
	}
	return "Other"
}
