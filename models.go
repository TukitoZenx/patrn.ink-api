package main

import "time"

// User represents an authenticated user
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
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
	IsActive    bool       `json:"is_active"`
}

// AnalyticsEvent represents a click event
type AnalyticsEvent struct {
	ShortCode string    `json:"short_code"`
	Timestamp time.Time `json:"timestamp"`
	Referrer  string    `json:"referrer"`
	UserAgent string    `json:"user_agent"`
	IPAddress string    `json:"ip_address"`
	Country   string    `json:"country,omitempty"`
}

// AnalyticsSummary represents aggregated analytics
type AnalyticsSummary struct {
	TotalClicks  int64            `json:"total_clicks"`
	UniqueClicks int64            `json:"unique_clicks"`
	TopReferrers map[string]int64 `json:"top_referrers"`
	ClicksByDate map[string]int64 `json:"clicks_by_date"`
	DeviceTypes  map[string]int64 `json:"device_types"`
	BrowserTypes map[string]int64 `json:"browser_types"`
}

// CreateLinkRequest represents the request to create a short URL
type CreateLinkRequest struct {
	LongURL    string `json:"long_url" binding:"required,url"`
	CustomCode string `json:"custom_code,omitempty"`
	ExpiresIn  int    `json:"expires_in,omitempty"` // hours
}

// CreateLinkResponse represents the response after creating a short URL
type CreateLinkResponse struct {
	ShortURL  string     `json:"short_url"`
	ShortCode string     `json:"short_code"`
	LongURL   string     `json:"long_url"`
	QRCodeURL string     `json:"qr_code_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
