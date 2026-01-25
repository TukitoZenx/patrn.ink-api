package models

import "time"

// User represents an authenticated user
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
	Provider  string    `json:"provider"` // "google" or "github"
	CreatedAt time.Time `json:"created_at"`
}

// Link represents a shortened URL
type Link struct {
	ShortCode   string     `json:"short_code"`
	LongURL     string     `json:"long_url"`
	UserID      string     `json:"user_id"`
	CustomAlias bool       `json:"custom_alias"`
	Clicks      int64      `json:"clicks"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"` // Link goes live at this time
	IsActive    bool       `json:"is_active"`
	Tags        []string   `json:"tags,omitempty"`        // Categories/tags for organization
	Password    string     `json:"password,omitempty"`    // Password hash for protected links
	IsArchived  bool       `json:"is_archived"`           // Soft archive (not deleted)
	Title       string     `json:"title,omitempty"`       // Optional link title
	Description string     `json:"description,omitempty"` // Optional description
}

// AnalyticsEvent represents a click event
type AnalyticsEvent struct {
	ShortCode  string    `json:"short_code"`
	Timestamp  time.Time `json:"timestamp"`
	Referrer   string    `json:"referrer"`
	UserAgent  string    `json:"user_agent"`
	IPAddress  string    `json:"ip_address"`
	Country    string    `json:"country,omitempty"`
	DeviceType string    `json:"device_type,omitempty"`
	Browser    string    `json:"browser,omitempty"`
}

// AnalyticsSummary represents aggregated analytics
type AnalyticsSummary struct {
	TotalClicks  int64            `json:"total_clicks"`
	UniqueClicks int64            `json:"unique_clicks"`
	TopReferrers map[string]int64 `json:"top_referrers"`
	ClicksByDate map[string]int64 `json:"clicks_by_date"`
	ClicksByHour map[string]int64 `json:"clicks_by_hour"`
	DeviceTypes  map[string]int64 `json:"device_types"`
	BrowserTypes map[string]int64 `json:"browser_types"`
	Countries    map[string]int64 `json:"countries"`
	Timeline     []TimelinePoint  `json:"timeline"`
}

// TimelinePoint represents a single point in the click timeline
type TimelinePoint struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

// CreateLinkRequest represents the request to create a short URL
type CreateLinkRequest struct {
	LongURL     string   `json:"long_url" binding:"required,url"`
	CustomCode  string   `json:"custom_code,omitempty"`
	ExpiresIn   int      `json:"expires_in,omitempty"`   // hours
	ScheduledAt string   `json:"scheduled_at,omitempty"` // ISO 8601 datetime
	Tags        []string `json:"tags,omitempty"`
	Password    string   `json:"password,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
}

// CreateLinkResponse represents the response after creating a short URL
type CreateLinkResponse struct {
	ShortURL    string     `json:"short_url"`
	ShortCode   string     `json:"short_code"`
	LongURL     string     `json:"long_url"`
	QRCodeURL   string     `json:"qr_code_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

// UpdateLinkRequest represents the request to update a link
type UpdateLinkRequest struct {
	LongURL     string   `json:"long_url,omitempty"`
	ExpiresIn   int      `json:"expires_in,omitempty"`
	ScheduledAt string   `json:"scheduled_at,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Password    string   `json:"password,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	IsArchived  *bool    `json:"is_archived,omitempty"`
}

// LinksQuery represents query parameters for listing links
type LinksQuery struct {
	Search    string   `form:"search"`     // Search in URL, code, title
	Tags      []string `form:"tags"`       // Filter by tags
	Page      int      `form:"page"`       // Page number (1-based)
	Limit     int      `form:"limit"`      // Items per page (default 20, max 100)
	SortBy    string   `form:"sort_by"`    // clicks, created_at, expires_at
	SortOrder string   `form:"sort_order"` // asc, desc
	Archived  *bool    `form:"archived"`   // Filter by archived status
}

// PaginatedLinks represents paginated link results
type PaginatedLinks struct {
	Links      []*Link `json:"links"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	Limit      int     `json:"limit"`
	TotalPages int     `json:"total_pages"`
}

// AnalyticsQuery represents query parameters for analytics
type AnalyticsQuery struct {
	StartDate string `form:"start_date"` // ISO 8601 date
	EndDate   string `form:"end_date"`   // ISO 8601 date
}

// BulkDeleteRequest represents a request to delete multiple links
type BulkDeleteRequest struct {
	Codes   []string `json:"codes" binding:"required,min=1"`
	Archive bool     `json:"archive"` // If true, archive instead of delete
}

// BulkDeleteResponse represents the result of bulk delete
type BulkDeleteResponse struct {
	Deleted []string          `json:"deleted"`
	Failed  map[string]string `json:"failed,omitempty"`
}

// BulkImportRequest represents a request to import links from CSV
type BulkImportItem struct {
	LongURL    string   `json:"long_url" binding:"required,url"`
	CustomCode string   `json:"custom_code,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Title      string   `json:"title,omitempty"`
}

type BulkImportRequest struct {
	Links []BulkImportItem `json:"links" binding:"required,min=1,max=100"`
}

// BulkImportResponse represents the result of bulk import
type BulkImportResponse struct {
	Created []CreateLinkResponse `json:"created"`
	Failed  []BulkImportError    `json:"failed,omitempty"`
}

type BulkImportError struct {
	Index  int    `json:"index"`
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// ExportFormat represents supported export formats
type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatJSON ExportFormat = "json"
)

// APIToken represents a personal API token
type APIToken struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	TokenHash   string     `json:"-"`            // Never expose hash
	TokenPrefix string     `json:"token_prefix"` // First 8 chars for identification
	Scopes      []string   `json:"scopes"`       // e.g., ["links:read", "links:write", "analytics:read"]
	RateLimit   int        `json:"rate_limit"`   // Requests per minute
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	IsActive    bool       `json:"is_active"`
}

// CreateAPITokenRequest represents a request to create an API token
type CreateAPITokenRequest struct {
	Name      string   `json:"name" binding:"required,min=1,max=50"`
	Scopes    []string `json:"scopes" binding:"required,min=1"`
	ExpiresIn int      `json:"expires_in,omitempty"` // Days until expiration (0 = never)
}

// CreateAPITokenResponse includes the raw token (only shown once)
type CreateAPITokenResponse struct {
	Token    string    `json:"token"` // Raw token - only shown once!
	APIToken *APIToken `json:"api_token"`
}

// LinkPreview represents metadata for a URL preview
type LinkPreview struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image,omitempty"`
	Favicon     string `json:"favicon,omitempty"`
	URL         string `json:"url"`
	Domain      string `json:"domain"`
}

// PasswordVerifyRequest represents a request to verify link password
type PasswordVerifyRequest struct {
	Password string `json:"password" binding:"required"`
}
