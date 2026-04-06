package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/middleware"
	"patrn.ink/internal/models"
	"patrn.ink/internal/shortcode"
	"patrn.ink/internal/storage"
)

// ShortenHandler creates a new short URL
// @Summary      Create short URL
// @Description  Creates a new shortened URL with optional custom code, expiration, scheduling, tags, and password protection
// @Tags         Links
// @Accept       json
// @Produce      json
// @Param        request  body      models.CreateLinkRequest  true  "Link creation request"
// @Success      201      {object}  models.CreateLinkResponse
// @Failure      400      {object}  map[string]string  "Invalid request"
// @Failure      401      {object}  map[string]string  "Unauthorized"
// @Failure      409      {object}  map[string]string  "Custom code already in use"
// @Failure      500      {object}  map[string]string  "Server error"
// @Security     BearerAuth
// @Router       /api/shorten [post]
func ShortenHandler(c *gin.Context) {
	var req models.CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := c.GetString("user_id")

	var shortCode string
	var customAlias bool

	// Handle custom code
	if req.CustomCode != "" {
		// Validate custom code (alphanumeric, 3-20 chars)
		if !shortcode.IsValidCustomCode(req.CustomCode) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Custom code must be 3-20 alphanumeric characters",
			})
			return
		}

		// Check if custom code already exists
		existing, _ := storage.GetLink(req.CustomCode)
		if existing != nil && existing.IsActive {
			c.JSON(http.StatusConflict, gin.H{"error": "Custom code already in use"})
			return
		}

		shortCode = req.CustomCode
		customAlias = true
	} else {
		// Generate random short code
		var err error
		shortCode, err = storage.GenerateUniqueShortCode(7, 3)
		if err != nil {
			logger.Logger.Error("Failed to generate short code", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate short code"})
			return
		}
		customAlias = false
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &expiry
	}

	// Parse scheduled activation time
	var scheduledAt *time.Time
	if req.ScheduledAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scheduled_at format. Use ISO 8601 (RFC3339)"})
			return
		}
		if parsed.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scheduled_at must be in the future"})
			return
		}
		scheduledAt = &parsed
	}

	// Hash password if provided
	var passwordHash string
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			logger.Logger.Error("Failed to hash password", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
			return
		}
		passwordHash = string(hash)
	}

	rotationTargets, err := validateRotationTargets(req.RotationTargets)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ensurePrimaryDestinationIsUnique(req.LongURL, rotationTargets); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create link
	link := &models.Link{
		ShortCode:       shortCode,
		LongURL:         req.LongURL,
		UserID:          userID,
		CustomAlias:     customAlias,
		Clicks:          0,
		CreatedAt:       time.Now(),
		ExpiresAt:       expiresAt,
		ScheduledAt:     scheduledAt,
		IsActive:        true,
		IsArchived:      false,
		Tags:            req.Tags,
		Password:        passwordHash,
		Title:           req.Title,
		Description:     req.Description,
		AgeVerification: models.AgeVerification(req.AgeVerification),
		RotationTargets: rotationTargets,
	}

	if err := storage.SaveLink(link); err != nil {
		logger.Logger.Error("Failed to save link", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create short URL"})
		return
	}

	if err := refreshLinkHealth(link); err != nil {
		logger.Logger.Warn("Failed to refresh link health after creation",
			zap.String("code", shortCode),
			zap.Error(err),
		)
	}

	// Build response
	response := models.CreateLinkResponse{
		ShortURL:    config.AppConfig.BaseURL + "/" + shortCode,
		ShortCode:   shortCode,
		LongURL:     req.LongURL,
		QRCodeURL:   config.AppConfig.BaseURL + "/" + shortCode + "/qr",
		ExpiresAt:   expiresAt,
		ScheduledAt: scheduledAt,
		Tags:        req.Tags,
	}

	logger.Logger.Info("Short URL created",
		zap.String("code", shortCode),
		zap.String("user", userID),
		zap.Bool("custom", customAlias),
		zap.Bool("has_password", passwordHash != ""),
		zap.Bool("scheduled", scheduledAt != nil),
	)

	c.JSON(http.StatusCreated, response)
}

// RedirectHandler handles short URL redirects
// @Summary      Redirect to long URL
// @Description  Redirects to the original URL. Returns 403 if password or age verification is required.
// @Tags         Redirect
// @Produce      json
// @Param        code  path      string  true  "Short code"
// @Success      301   {string}  string  "Redirect to long URL"
// @Failure      403   {object}  map[string]interface{}  "Password or age verification required"
// @Failure      404   {object}  map[string]string  "URL not found"
// @Failure      410   {object}  map[string]string  "Link expired, archived, or deactivated"
// @Router       /{code} [get]
func RedirectHandler(c *gin.Context) {
	code := c.Param("code")
	htmlRequest := prefersHTML(c)

	// Fallback to database
	link, err := storage.GetLink(code)
	if err != nil {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusNotFound,
				code,
				"Link not found",
				"This short link does not exist or may have been removed.",
				"Check the link for typos or ask the sender to share it again.",
				nil,
			)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	// Check if link is active
	if !link.IsActive {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusGone,
				code,
				"This link has been deactivated",
				"The destination behind this short link is no longer available.",
				"Reach out to the link owner if you expected this destination to still be active.",
				link,
			)
			return
		}
		c.JSON(http.StatusGone, gin.H{"error": "This link has been deactivated"})
		return
	}

	// Check if archived
	if link.IsArchived {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusGone,
				code,
				"This link has been archived",
				"This short link has been retired and is no longer open to visitors.",
				"Archived links stay organized for teams, but they are no longer available for public access.",
				link,
			)
			return
		}
		c.JSON(http.StatusGone, gin.H{"error": "This link has been archived"})
		return
	}

	// Check if expired
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusGone,
				code,
				"This link has expired",
				"This destination was available for a limited time and is no longer active.",
				"Expiration helps teams run time-bound campaigns without leaving old links open indefinitely.",
				link,
			)
			return
		}
		c.JSON(http.StatusGone, gin.H{"error": "This link has expired"})
		return
	}

	// Check if scheduled (not yet active)
	if link.ScheduledAt != nil && link.ScheduledAt.After(time.Now()) {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusNotFound,
				code,
				"This link is not live yet",
				"This destination is scheduled to go live later, so access is not available yet.",
				"Scheduled launch: "+formatPublicTime(link.ScheduledAt),
				link,
			)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "This link is not yet active"})
		return
	}

	ageVerified := hasValidAgeProof(c, code, link.AgeVerification)

	if link.AgeVerification > 0 && !ageVerified {
		ageLabel := ageLabelForLevel(link.AgeVerification)
		ageBody := "This destination is age-gated. Confirm your age first."
		if link.Password != "" {
			ageBody = "This destination is age-gated and password protected. Confirm your age first, then you will unlock it with the password."
		}
		if htmlRequest {
			renderPublicPage(c, http.StatusForbidden, buildPublicPageData(code, link, publicPageData{
				PageTitle:    "Age confirmation required · patrn.ink",
				Eyebrow:      "Protected link",
				Heading:      "Confirm your age before continuing",
				Body:         ageBody,
				Detail:       "Required age: " + ageLabel,
				ActionURL:    config.AppConfig.BaseURL + "/" + code + "/verify-age",
				ActionLabel:  "I confirm I am " + ageLabel + " or older",
				ShowAgeForm:  true,
				AgeLabel:     ageLabel,
				AgeLevel:     int(link.AgeVerification),
				SecondaryURL: config.AppConfig.FrontendURL,
			}))
			return
		}
		response := gin.H{
			"error":         "Age verification required",
			"age_required":  ageLabel,
			"age_level":     int(link.AgeVerification),
			"verify_url":    config.AppConfig.BaseURL + "/" + code + "/verify-age",
			"title":         link.Title,
			"description":   link.Description,
			"gate_sequence": []string{"age"},
		}
		if link.Password != "" {
			response["password_required"] = true
			response["gate_sequence"] = []string{"age", "password"}
		}
		c.JSON(http.StatusForbidden, response)
		return
	}

	if link.Password != "" {
		if htmlRequest {
			renderPasswordGate(c, code, link, "", http.StatusForbidden)
			return
		}
		// Return a special response indicating password is required
		c.JSON(http.StatusForbidden, gin.H{
			"error":             "Password required",
			"password_required": true,
			"verify_url":        config.AppConfig.BaseURL + "/" + code + "/verify",
		})
		return
	}

	// Record analytics and increment counter
	RecordAnalytics(c, code)
	_ = storage.IncrementClicks(code)
	middleware.IncRedirects()

	// Refresh cache
	redirectURL := selectRedirectDestination(link)
	_ = storage.SaveToCache(code, redirectURL)

	c.Redirect(http.StatusMovedPermanently, redirectURL)
}

// VerifyPasswordHandler verifies password and redirects
// @Summary      Verify link password
// @Description  Verifies the password for a password-protected link and returns the redirect URL
// @Tags         Redirect
// @Accept       json
// @Produce      json
// @Param        code     path      string                      true  "Short code"
// @Param        request  body      models.PasswordVerifyRequest  true  "Password verification request"
// @Success      200      {object}  map[string]string  "redirect_url"
// @Failure      400      {object}  map[string]string  "Invalid request"
// @Failure      401      {object}  map[string]string  "Invalid password"
// @Failure      404      {object}  map[string]string  "URL not found"
// @Router       /{code}/verify [post]
func VerifyPasswordHandler(c *gin.Context) {
	code := c.Param("code")
	htmlRequest := prefersHTML(c) || isHTMLFormPost(c)

	var req models.PasswordVerifyRequest
	if isHTMLFormPost(c) {
		req.Password = c.PostForm("password")
	}
	if req.Password == "" {
		if err := c.ShouldBindJSON(&req); err != nil && !isHTMLFormPost(c) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password is required"})
			return
		}
	}
	if req.Password == "" {
		if htmlRequest {
			link, _ := storage.GetLink(code)
			renderPasswordGate(c, code, link, "Enter the password to continue.", http.StatusBadRequest)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password is required"})
		return
	}

	link, err := storage.GetLink(code)
	if err != nil {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusNotFound,
				code,
				"Link not found",
				"This short link does not exist or may have been removed.",
				"Check the link and try again.",
				nil,
			)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	if link.Password == "" {
		// No password required, just redirect
		redirectURL := selectRedirectDestination(link)
		if htmlRequest {
			c.Redirect(http.StatusFound, redirectURL)
			return
		}
		c.JSON(http.StatusOK, gin.H{"redirect_url": redirectURL})
		return
	}

	if link.AgeVerification > 0 && !hasValidAgeProof(c, code, link.AgeVerification) {
		ageLabel := ageLabelForLevel(link.AgeVerification)
		if htmlRequest {
			renderPublicPage(c, http.StatusForbidden, buildPublicPageData(code, link, publicPageData{
				PageTitle:    "Age confirmation required · patrn.ink",
				Eyebrow:      "Protected link",
				Heading:      "Confirm your age before entering the password",
				Body:         "This destination requires age confirmation before the password step can unlock it.",
				Detail:       "Required age: " + ageLabel,
				ActionURL:    config.AppConfig.BaseURL + "/" + code + "/verify-age",
				ActionLabel:  "I confirm I am " + ageLabel + " or older",
				ShowAgeForm:  true,
				AgeLabel:     ageLabel,
				AgeLevel:     int(link.AgeVerification),
				SecondaryURL: config.AppConfig.FrontendURL,
			}))
			return
		}
		c.JSON(http.StatusForbidden, gin.H{
			"error":             "Age verification required",
			"password_required": true,
			"age_required":      ageLabel,
			"age_level":         int(link.AgeVerification),
			"verify_url":        config.AppConfig.BaseURL + "/" + code + "/verify-age",
			"gate_sequence":     []string{"age", "password"},
			"title":             link.Title,
			"description":       link.Description,
		})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(link.Password), []byte(req.Password)); err != nil {
		if htmlRequest {
			renderPasswordGate(c, code, link, "That password did not match. Try again.", http.StatusUnauthorized)
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// Password correct - record analytics and return redirect URL
	RecordAnalytics(c, code)
	_ = storage.IncrementClicks(code)
	middleware.IncRedirects()
	redirectURL := selectRedirectDestination(link)

	if htmlRequest {
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	c.JSON(http.StatusOK, gin.H{"redirect_url": redirectURL})
}

// VerifyAgeHandler verifies age confirmation and redirects
// @Summary      Verify age for age-gated link
// @Description  Confirms the user's age and returns the redirect URL for age-restricted content
// @Tags         Redirect
// @Accept       json
// @Produce      json
// @Param        code     path      string  true  "Short code"
// @Param        request  body      object{confirmed=bool,age_level=int}  true  "Age verification request"
// @Success      200      {object}  map[string]string  "redirect_url"
// @Failure      400      {object}  map[string]string  "Invalid request"
// @Failure      403      {object}  map[string]string  "Age requirement not met"
// @Failure      404      {object}  map[string]string  "URL not found"
// @Router       /{code}/verify-age [post]
func VerifyAgeHandler(c *gin.Context) {
	code := c.Param("code")
	htmlRequest := prefersHTML(c) || isHTMLFormPost(c)

	var req struct {
		Confirmed bool `json:"confirmed" binding:"required"`
		AgeLevel  int  `json:"age_level" binding:"required"`
	}

	if isHTMLFormPost(c) {
		req.Confirmed = c.PostForm("confirmed") == "true" || c.PostForm("confirmed") == "on"
		_, _ = fmt.Sscanf(c.PostForm("age_level"), "%d", &req.AgeLevel)
	}
	if req.AgeLevel == 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !isHTMLFormPost(c) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Age confirmation is required"})
			return
		}
	}

	link, err := storage.GetLink(code)
	if err != nil {
		if htmlRequest {
			renderUnavailablePage(
				c,
				http.StatusNotFound,
				code,
				"Link not found",
				"This short link does not exist or may have been removed.",
				"Check the link and try again.",
				nil,
			)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	ageLabel := ageLabelForLevel(link.AgeVerification)

	if req.AgeLevel == 0 {
		if htmlRequest {
			renderAgeGate(c, code, link, ageLabel, "Confirm your age to continue.", http.StatusBadRequest)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Age confirmation is required"})
		return
	}

	if !req.Confirmed {
		if htmlRequest {
			renderAgeGate(c, code, link, ageLabel, "You need to confirm your age before continuing.", http.StatusForbidden)
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Age confirmation not provided"})
		return
	}

	// Verify the age level matches what's required
	if req.AgeLevel < int(link.AgeVerification) {
		if htmlRequest {
			renderAgeGate(c, code, link, ageLabel, "This destination requires a higher age confirmation.", http.StatusForbidden)
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "Age requirement not met"})
		return
	}

	issueAgeProof(c, code, link.AgeVerification)

	if link.Password != "" {
		if htmlRequest {
			renderPasswordGate(c, code, link, "", http.StatusForbidden)
			return
		}
		c.JSON(http.StatusForbidden, gin.H{
			"error":             "Password required",
			"password_required": true,
			"verify_url":        config.AppConfig.BaseURL + "/" + code + "/verify",
			"title":             link.Title,
			"description":       link.Description,
		})
		return
	}

	// Age confirmed - record analytics and return redirect URL
	RecordAnalytics(c, code)
	_ = storage.IncrementClicks(code)
	middleware.IncRedirects()
	redirectURL := selectRedirectDestination(link)

	if htmlRequest {
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	c.JSON(http.StatusOK, gin.H{"redirect_url": redirectURL})
}

// GetLinksHandler returns all links for the authenticated user with search and pagination
// @Summary      List user links
// @Description  Returns all links for the authenticated user with optional search, filtering, and pagination
// @Tags         Links
// @Produce      json
// @Param        search      query     string   false  "Search in URL, code, or title"
// @Param        tags        query     []string false  "Filter by tags"
// @Param        page        query     int      false  "Page number (default: 1)"
// @Param        limit       query     int      false  "Items per page (default: 20, max: 100)"
// @Param        sort_by     query     string   false  "Sort by: clicks, created_at, expires_at"
// @Param        sort_order  query     string   false  "Sort order: asc, desc"
// @Param        archived    query     bool     false  "Filter by archived status"
// @Success      200         {object}  models.PaginatedLinks
// @Failure      400         {object}  map[string]string  "Invalid query parameters"
// @Failure      401         {object}  map[string]string  "Unauthorized"
// @Failure      500         {object}  map[string]string  "Server error"
// @Security     BearerAuth
// @Router       /api/links [get]
func GetLinksHandler(c *gin.Context) {
	userID := c.GetString("user_id")

	var query models.LinksQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	// Set defaults
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	if query.SortBy == "" {
		query.SortBy = "created_at"
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}

	result, err := storage.GetUserLinksWithQuery(userID, &query)
	if err != nil {
		logger.Logger.Error("Failed to get user links", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve links"})
		return
	}

	for _, link := range result.Links {
		if shouldRefreshHealth(link) {
			refreshLinkHealthAsync(link.ShortCode)
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetLinkDetailsHandler returns details for a specific link
// @Summary      Get link details
// @Description  Returns detailed information about a specific link owned by the user
// @Tags         Links
// @Produce      json
// @Param        code  path      string  true  "Short code"
// @Success      200   {object}  models.Link
// @Failure      401   {object}  map[string]string  "Unauthorized"
// @Failure      403   {object}  map[string]string  "Forbidden - not link owner"
// @Failure      404   {object}  map[string]string  "Link not found"
// @Security     BearerAuth
// @Router       /api/links/{code} [get]
func GetLinkDetailsHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	link, err := storage.GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
		return
	}

	// Verify ownership
	if link.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	if shouldRefreshHealth(link) {
		refreshLinkHealthAsync(link.ShortCode)
	}

	c.JSON(http.StatusOK, link)
}

// DeleteLinkHandler deletes a link
// @Summary      Delete link
// @Description  Permanently deletes a link owned by the user
// @Tags         Links
// @Produce      json
// @Param        code  path      string  true  "Short code"
// @Success      200   {object}  map[string]string  "Success message"
// @Failure      401   {object}  map[string]string  "Unauthorized"
// @Failure      403   {object}  map[string]string  "Forbidden - not link owner"
// @Failure      500   {object}  map[string]string  "Server error"
// @Security     BearerAuth
// @Router       /api/links/{code} [delete]
func DeleteLinkHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	if err := storage.DeleteLink(code, userID); err != nil {
		if err.Error() == "unauthorized: link belongs to different user" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete link"})
		return
	}

	logger.Logger.Info("Link deleted", zap.String("code", code), zap.String("user", userID))
	c.JSON(http.StatusOK, gin.H{"message": "Link deleted successfully"})
}

// UpdateLinkHandler updates a link's properties
// @Summary      Update link
// @Description  Updates properties of a link owned by the user
// @Tags         Links
// @Accept       json
// @Produce      json
// @Param        code     path      string                    true  "Short code"
// @Param        request  body      models.UpdateLinkRequest  true  "Update request"
// @Success      200      {object}  models.Link
// @Failure      400      {object}  map[string]string  "Invalid request"
// @Failure      401      {object}  map[string]string  "Unauthorized"
// @Failure      403      {object}  map[string]string  "Forbidden - not link owner"
// @Failure      404      {object}  map[string]string  "Link not found"
// @Failure      500      {object}  map[string]string  "Server error"
// @Security     BearerAuth
// @Router       /api/links/{code} [put]
func UpdateLinkHandler(c *gin.Context) {
	code := c.Param("code")
	userID := c.GetString("user_id")

	var req models.UpdateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get existing link
	link, err := storage.GetLink(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
		return
	}

	// Verify ownership
	if link.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	// Update fields
	if req.LongURL != "" {
		link.LongURL = req.LongURL
		// Invalidate cache
		_ = storage.DeleteFromCache(code)
	}

	if req.ExpiresIn > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		link.ExpiresAt = &expiry
	}

	if req.ScheduledAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scheduled_at format"})
			return
		}
		link.ScheduledAt = &parsed
	}

	if req.Tags != nil {
		link.Tags = req.Tags
	}

	if req.Title != "" {
		link.Title = req.Title
	}

	if req.Description != "" {
		link.Description = req.Description
	}

	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
			return
		}
		link.Password = string(hash)
	}

	if req.IsArchived != nil {
		link.IsArchived = *req.IsArchived
	}

	if req.RotationTargets != nil {
		rotationTargets, err := validateRotationTargets(req.RotationTargets)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		primaryURL := link.LongURL
		if req.LongURL != "" {
			primaryURL = req.LongURL
		}
		if err := ensurePrimaryDestinationIsUnique(primaryURL, rotationTargets); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		link.RotationTargets = rotationTargets
		link.RotationCursor = 0
	}

	// Save updated link
	if err := storage.SaveLink(link); err != nil {
		logger.Logger.Error("Failed to update link", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update link"})
		return
	}

	if err := refreshLinkHealth(link); err != nil {
		logger.Logger.Warn("Failed to refresh link health after update",
			zap.String("code", code),
			zap.Error(err),
		)
	}

	logger.Logger.Info("Link updated", zap.String("code", code), zap.String("user", userID))
	c.JSON(http.StatusOK, link)
}
