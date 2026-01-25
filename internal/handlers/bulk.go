package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/models"
	"patrn.ink/internal/shortcode"
	"patrn.ink/internal/storage"
)

// BulkDeleteHandler deletes or archives multiple links
func BulkDeleteHandler(c *gin.Context) {
	var req models.BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := c.GetString("user_id")

	deleted := make([]string, 0)
	failed := make(map[string]string)

	for _, code := range req.Codes {
		var err error
		if req.Archive {
			err = storage.ArchiveLink(code, userID)
		} else {
			err = storage.DeleteLink(code, userID)
		}

		if err != nil {
			if err.Error() == "unauthorized: link belongs to different user" {
				failed[code] = "unauthorized"
			} else {
				failed[code] = err.Error()
			}
		} else {
			deleted = append(deleted, code)
		}
	}

	action := "deleted"
	if req.Archive {
		action = "archived"
	}

	logger.Logger.Info("Bulk operation completed",
		zap.String("action", action),
		zap.Int("deleted_count", len(deleted)),
		zap.Int("failed_count", len(failed)),
		zap.String("user", userID),
	)

	c.JSON(http.StatusOK, models.BulkDeleteResponse{
		Deleted: deleted,
		Failed:  failed,
	})
}

// BulkImportHandler imports multiple links from JSON
func BulkImportHandler(c *gin.Context) {
	var req models.BulkImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := c.GetString("user_id")

	created := make([]models.CreateLinkResponse, 0)
	failed := make([]models.BulkImportError, 0)

	for i, item := range req.Links {
		var shortCode string
		var customAlias bool

		// Handle custom code
		if item.CustomCode != "" {
			if !shortcode.IsValidCustomCode(item.CustomCode) {
				failed = append(failed, models.BulkImportError{
					Index:  i,
					URL:    item.LongURL,
					Reason: "Invalid custom code (must be 3-20 alphanumeric characters)",
				})
				continue
			}

			existing, _ := storage.GetLink(item.CustomCode)
			if existing != nil && existing.IsActive {
				failed = append(failed, models.BulkImportError{
					Index:  i,
					URL:    item.LongURL,
					Reason: "Custom code already in use",
				})
				continue
			}

			shortCode = item.CustomCode
			customAlias = true
		} else {
			var err error
			shortCode, err = storage.GenerateUniqueShortCode(7, 3)
			if err != nil {
				failed = append(failed, models.BulkImportError{
					Index:  i,
					URL:    item.LongURL,
					Reason: "Failed to generate short code",
				})
				continue
			}
			customAlias = false
		}

		// Create link
		link := &models.Link{
			ShortCode:   shortCode,
			LongURL:     item.LongURL,
			UserID:      userID,
			CustomAlias: customAlias,
			Tags:        item.Tags,
			Title:       item.Title,
			Clicks:      0,
			CreatedAt:   time.Now(),
			IsActive:    true,
			IsArchived:  false,
		}

		if err := storage.SaveLink(link); err != nil {
			failed = append(failed, models.BulkImportError{
				Index:  i,
				URL:    item.LongURL,
				Reason: "Failed to save link",
			})
			continue
		}

		created = append(created, models.CreateLinkResponse{
			ShortURL:  config.AppConfig.BaseURL + "/" + shortCode,
			ShortCode: shortCode,
			LongURL:   item.LongURL,
			QRCodeURL: config.AppConfig.BaseURL + "/" + shortCode + "/qr",
			Tags:      item.Tags,
		})
	}

	logger.Logger.Info("Bulk import completed",
		zap.Int("created_count", len(created)),
		zap.Int("failed_count", len(failed)),
		zap.String("user", userID),
	)

	c.JSON(http.StatusOK, models.BulkImportResponse{
		Created: created,
		Failed:  failed,
	})
}

// ExportLinksHandler exports user's links as CSV or JSON
func ExportLinksHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	format := c.DefaultQuery("format", "csv")

	links, err := storage.GetUserLinks(userID)
	if err != nil {
		logger.Logger.Error("Failed to get user links for export", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve links"})
		return
	}

	if format == "json" {
		c.Header("Content-Disposition", "attachment; filename=links_export.json")
		c.JSON(http.StatusOK, gin.H{"links": links, "exported_at": time.Now()})
		return
	}

	// CSV export
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"short_code", "short_url", "long_url", "title", "tags", "clicks", "created_at", "expires_at", "is_active", "is_archived"}
	if err := writer.Write(header); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CSV"})
		return
	}

	// Write data rows
	for _, link := range links {
		expiresAt := ""
		if link.ExpiresAt != nil {
			expiresAt = link.ExpiresAt.Format(time.RFC3339)
		}

		tags := ""
		if len(link.Tags) > 0 {
			for i, tag := range link.Tags {
				if i > 0 {
					tags += ","
				}
				tags += tag
			}
		}

		row := []string{
			link.ShortCode,
			config.AppConfig.BaseURL + "/" + link.ShortCode,
			link.LongURL,
			link.Title,
			tags,
			fmt.Sprintf("%d", link.Clicks),
			link.CreatedAt.Format(time.RFC3339),
			expiresAt,
			fmt.Sprintf("%t", link.IsActive),
			fmt.Sprintf("%t", link.IsArchived),
		}

		if err := writer.Write(row); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CSV"})
			return
		}
	}

	writer.Flush()

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename=links_export.csv")
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}

// ExportAnalyticsHandler exports analytics data as CSV
func ExportAnalyticsHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	// Verify ownership
	link, err := storage.GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
		return
	}

	if link.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse date range
	startDate := c.DefaultQuery("start_date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	endDate := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	events, err := storage.GetAnalyticsEvents(code, startDate, endDate)
	if err != nil {
		logger.Logger.Error("Failed to get analytics for export", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve analytics"})
		return
	}

	format := c.DefaultQuery("format", "csv")

	if format == "json" {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=analytics_%s.json", code))
		c.JSON(http.StatusOK, gin.H{"events": events, "exported_at": time.Now()})
		return
	}

	// CSV export
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"timestamp", "referrer", "user_agent", "ip_address", "country", "device_type", "browser"}
	if err := writer.Write(header); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CSV"})
		return
	}

	// Write data rows
	for _, event := range events {
		row := []string{
			event.Timestamp.Format(time.RFC3339),
			event.Referrer,
			event.UserAgent,
			event.IPAddress,
			event.Country,
			event.DeviceType,
			event.Browser,
		}

		if err := writer.Write(row); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CSV"})
			return
		}
	}

	writer.Flush()

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=analytics_%s.csv", code))
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}
