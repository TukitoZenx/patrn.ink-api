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
		userAgent := c.Request.UserAgent()
		event := &models.AnalyticsEvent{
			ShortCode:  shortCode,
			Timestamp:  time.Now(),
			Referrer:   c.Request.Referer(),
			UserAgent:  userAgent,
			IPAddress:  c.ClientIP(),
			Country:    extractCountryFromIP(c.ClientIP()),
			DeviceType: detectDeviceType(userAgent),
			Browser:    detectBrowser(userAgent),
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

// GetAnalyticsHandler returns analytics for a specific link with date range support
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

	// Parse date range from query params
	var query models.AnalyticsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(400, gin.H{"error": "Invalid query parameters"})
		return
	}

	// Default to last 30 days if not specified
	if query.StartDate == "" {
		query.StartDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if query.EndDate == "" {
		query.EndDate = time.Now().Format("2006-01-02")
	}

	// Validate date format
	startDate, err := time.Parse("2006-01-02", query.StartDate)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
		return
	}
	endDate, err := time.Parse("2006-01-02", query.EndDate)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
		return
	}

	// Get analytics events for the date range
	events, err := storage.GetAnalyticsEvents(code, query.StartDate, query.EndDate)
	if err != nil {
		logger.Logger.Error("Failed to get analytics events", zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to retrieve analytics"})
		return
	}

	// Aggregate the data
	summary := aggregateAnalytics(events, link.Clicks, startDate, endDate)

	c.JSON(200, summary)
}

// aggregateAnalytics aggregates analytics events into a summary
func aggregateAnalytics(events []*models.AnalyticsEvent, totalClicks int64, startDate, endDate time.Time) *models.AnalyticsSummary {
	summary := &models.AnalyticsSummary{
		TotalClicks:  totalClicks,
		UniqueClicks: 0,
		TopReferrers: make(map[string]int64),
		ClicksByDate: make(map[string]int64),
		ClicksByHour: make(map[string]int64),
		DeviceTypes:  make(map[string]int64),
		BrowserTypes: make(map[string]int64),
		Countries:    make(map[string]int64),
		Timeline:     make([]models.TimelinePoint, 0),
	}

	uniqueIPs := make(map[string]bool)

	for _, event := range events {
		// Count unique clicks by IP
		if !uniqueIPs[event.IPAddress] {
			uniqueIPs[event.IPAddress] = true
			summary.UniqueClicks++
		}

		// Aggregate by date
		dateKey := event.Timestamp.Format("2006-01-02")
		summary.ClicksByDate[dateKey]++

		// Aggregate by hour
		hourKey := event.Timestamp.Format("15")
		summary.ClicksByHour[hourKey]++

		// Aggregate referrers
		referrer := event.Referrer
		if referrer == "" {
			referrer = "Direct"
		} else {
			// Extract domain from referrer
			referrer = extractDomain(referrer)
		}
		summary.TopReferrers[referrer]++

		// Aggregate device types
		deviceType := event.DeviceType
		if deviceType == "" {
			deviceType = detectDeviceType(event.UserAgent)
		}
		summary.DeviceTypes[deviceType]++

		// Aggregate browsers
		browser := event.Browser
		if browser == "" {
			browser = detectBrowser(event.UserAgent)
		}
		summary.BrowserTypes[browser]++

		// Aggregate countries
		country := event.Country
		if country == "" {
			country = "Unknown"
		}
		summary.Countries[country]++
	}

	// Build timeline (fill in missing dates with 0)
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		clicks := summary.ClicksByDate[dateKey]
		summary.Timeline = append(summary.Timeline, models.TimelinePoint{
			Date:   dateKey,
			Clicks: clicks,
		})
	}

	return summary
}

// extractDomain extracts the domain from a URL
func extractDomain(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	// Get domain part
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return url
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
	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		return "Chrome"
	} else if strings.Contains(ua, "firefox") {
		return "Firefox"
	} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		return "Safari"
	} else if strings.Contains(ua, "edg") {
		return "Edge"
	} else if strings.Contains(ua, "opera") || strings.Contains(ua, "opr") {
		return "Opera"
	}
	return "Other"
}
