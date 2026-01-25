package handlers

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/net/html"

	"patrn.ink/internal/logger"
	"patrn.ink/internal/models"
	"patrn.ink/internal/storage"
)

// LinkPreviewHandler fetches metadata for a URL to generate a preview
func LinkPreviewHandler(c *gin.Context) {
	targetURL := c.Query("url")
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL parameter is required"})
		return
	}

	// Validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL"})
		return
	}

	// Check cache first
	cacheKey := "preview:" + targetURL
	cached, err := storage.GetCacheJSON(cacheKey)
	if err == nil && cached != nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	// Fetch the page
	preview, err := fetchLinkPreview(targetURL)
	if err != nil {
		logger.Logger.Error("Failed to fetch link preview", zap.Error(err), zap.String("url", targetURL))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch URL metadata"})
		return
	}

	// Cache for 1 hour
	_ = storage.SetCacheJSON(cacheKey, preview, time.Hour)

	c.JSON(http.StatusOK, preview)
}

// fetchLinkPreview fetches and parses HTML metadata from a URL
func fetchLinkPreview(targetURL string) (*models.LinkPreview, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	// Set a browser-like User-Agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PatrnBot/1.0; +https://patrn.ink)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Limit response body to 1MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}

	// Parse URL for domain
	parsedURL, _ := url.Parse(targetURL)
	domain := parsedURL.Host

	preview := &models.LinkPreview{
		URL:    targetURL,
		Domain: domain,
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		// Return basic preview if parsing fails
		return preview, nil
	}

	// Extract metadata
	extractMetadata(doc, preview)

	// Try to get favicon if not found in meta tags
	if preview.Favicon == "" {
		preview.Favicon = parsedURL.Scheme + "://" + parsedURL.Host + "/favicon.ico"
	}

	return preview, nil
}

// extractMetadata extracts Open Graph and other metadata from HTML
func extractMetadata(n *html.Node, preview *models.LinkPreview) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "title":
			if n.FirstChild != nil && preview.Title == "" {
				preview.Title = strings.TrimSpace(n.FirstChild.Data)
			}
		case "meta":
			var name, property, content string
			for _, attr := range n.Attr {
				switch attr.Key {
				case "name":
					name = attr.Val
				case "property":
					property = attr.Val
				case "content":
					content = attr.Val
				}
			}

			// Open Graph tags
			switch property {
			case "og:title":
				if preview.Title == "" || property == "og:title" {
					preview.Title = content
				}
			case "og:description":
				if preview.Description == "" || property == "og:description" {
					preview.Description = content
				}
			case "og:image":
				if preview.Image == "" {
					preview.Image = content
				}
			}

			// Standard meta tags
			switch name {
			case "description":
				if preview.Description == "" {
					preview.Description = content
				}
			}

		case "link":
			var rel, href string
			for _, attr := range n.Attr {
				switch attr.Key {
				case "rel":
					rel = attr.Val
				case "href":
					href = attr.Val
				}
			}

			// Favicon
			if strings.Contains(rel, "icon") && preview.Favicon == "" {
				preview.Favicon = href
			}
		}
	}

	// Recursively process child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractMetadata(c, preview)
	}
}

// GetLinkPreviewByCodeHandler returns preview for an existing short link
func GetLinkPreviewByCodeHandler(c *gin.Context) {
	code := c.Param("code")

	link, err := storage.GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
		return
	}

	// Check cache first
	cacheKey := "preview:" + link.LongURL
	cached, err := storage.GetCacheJSON(cacheKey)
	if err == nil && cached != nil {
		c.JSON(http.StatusOK, cached)
		return
	}

	// Fetch preview
	preview, err := fetchLinkPreview(link.LongURL)
	if err != nil {
		logger.Logger.Error("Failed to fetch link preview", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch preview"})
		return
	}

	// Cache for 1 hour
	_ = storage.SetCacheJSON(cacheKey, preview, time.Hour)

	c.JSON(http.StatusOK, preview)
}

// extractDomainFromURL extracts the domain from a URL string
func extractDomainFromURL(urlStr string) string {
	re := regexp.MustCompile(`^(?:https?://)?(?:www\.)?([^/]+)`)
	matches := re.FindStringSubmatch(urlStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return urlStr
}
