package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"patrn.ink/internal/logger"
	"patrn.ink/internal/models"
	"patrn.ink/internal/storage"
)

const (
	destinationStatusUnknown = "unknown"
	destinationStatusHealthy = "healthy"
	destinationStatusBroken  = "broken"
	linkHealthHealthy        = "healthy"
	linkHealthDegraded       = "degraded"
	linkHealthFailing        = "failing"
	linkHealthUnknown        = "unknown"
	destinationCheckTimeout  = 4 * time.Second
	destinationHealthMaxAge  = 30 * time.Minute
)

var destinationCheckClient = &http.Client{
	Timeout: destinationCheckTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func validateRotationTargets(inputs []models.RotationTargetInput) ([]models.RotationTarget, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(inputs))
	targets := make([]models.RotationTarget, 0, len(inputs))

	for _, input := range inputs {
		rawURL := strings.TrimSpace(input.URL)
		if !isValidHTTPURL(rawURL) {
			return nil, fmt.Errorf("rotation targets must use http or https URLs")
		}

		key := strings.ToLower(rawURL)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("rotation targets must not contain duplicate URLs")
		}
		seen[key] = struct{}{}

		isActive := true
		if input.IsActive != nil {
			isActive = *input.IsActive
		}

		targets = append(targets, models.RotationTarget{
			URL:      rawURL,
			Label:    strings.TrimSpace(input.Label),
			IsActive: isActive,
		})
	}

	return targets, nil
}

func ensurePrimaryDestinationIsUnique(primaryURL string, targets []models.RotationTarget) error {
	primaryKey := strings.ToLower(strings.TrimSpace(primaryURL))
	for _, target := range targets {
		if strings.ToLower(strings.TrimSpace(target.URL)) == primaryKey {
			return fmt.Errorf("rotation targets must not duplicate the primary destination")
		}
	}
	return nil
}

func isValidHTTPURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func refreshLinkHealth(link *models.Link) error {
	if link == nil {
		return nil
	}

	now := time.Now()
	healthyDestinations := 0
	failingDestinations := 0
	totalDestinations := 1

	link.PrimaryHealth = checkDestination(link.LongURL, now)
	if link.PrimaryHealth.Status == destinationStatusHealthy {
		healthyDestinations++
	} else if link.PrimaryHealth.Status == destinationStatusBroken {
		failingDestinations++
	}

	refreshedTargets := make([]models.RotationTarget, 0, len(link.RotationTargets))
	for _, target := range link.RotationTargets {
		target.Status = destinationStatusUnknown
		target.StatusCode = 0
		target.LastError = ""
		target.LastCheckedAt = nil

		check := checkDestination(target.URL, now)
		target.Status = check.Status
		target.StatusCode = check.StatusCode
		target.LastError = check.LastError
		target.LastCheckedAt = check.LastCheckedAt

		refreshedTargets = append(refreshedTargets, target)

		if !target.IsActive {
			continue
		}

		totalDestinations++
		if target.Status == destinationStatusHealthy {
			healthyDestinations++
		} else if target.Status == destinationStatusBroken {
			failingDestinations++
		}
	}
	link.RotationTargets = refreshedTargets

	link.HealthStatus = models.LinkHealthStatus{
		Status:              determineLinkHealthStatus(healthyDestinations, failingDestinations, totalDestinations),
		LastCheckedAt:       &now,
		HealthyDestinations: healthyDestinations,
		FailingDestinations: failingDestinations,
		TotalDestinations:   totalDestinations,
		NeedsAttention:      failingDestinations > 0,
	}

	return storage.SaveLink(link)
}

func checkDestination(destinationURL string, checkedAt time.Time) models.DestinationCheck {
	status, statusCode, lastError := probeDestination(destinationURL)

	return models.DestinationCheck{
		Status:        status,
		StatusCode:    statusCode,
		LastCheckedAt: &checkedAt,
		LastError:     lastError,
	}
}

func probeDestination(destinationURL string) (string, int, string) {
	statusCode, err := executeDestinationProbe(http.MethodHead, destinationURL)
	if err == nil && statusCode >= 200 && statusCode < 400 {
		return classifyDestinationStatus(statusCode), statusCode, ""
	}

	if statusCode > 0 {
		statusCode, err = executeDestinationProbe(http.MethodGet, destinationURL)
		if err == nil {
			return classifyDestinationStatus(statusCode), statusCode, ""
		}
	}

	lastError := err.Error()
	if statusCode > 0 {
		return classifyDestinationStatus(statusCode), statusCode, lastError
	}

	return destinationStatusBroken, 0, lastError
}

func executeDestinationProbe(method string, destinationURL string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), destinationCheckTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, method, destinationURL, nil)
	if err != nil {
		return 0, err
	}

	request.Header.Set("User-Agent", "patrn.ink health monitor/1.0")
	if method == http.MethodGet {
		request.Header.Set("Range", "bytes=0-0")
	}

	response, err := destinationCheckClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	return response.StatusCode, nil
}

func classifyDestinationStatus(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 400:
		return destinationStatusHealthy
	case statusCode >= 400:
		return destinationStatusBroken
	default:
		return destinationStatusUnknown
	}
}

func determineLinkHealthStatus(healthyDestinations, failingDestinations, totalDestinations int) string {
	switch {
	case totalDestinations == 0:
		return linkHealthUnknown
	case failingDestinations == 0 && healthyDestinations == totalDestinations:
		return linkHealthHealthy
	case failingDestinations > 0 && healthyDestinations == 0:
		return linkHealthFailing
	case failingDestinations > 0:
		return linkHealthDegraded
	default:
		return linkHealthUnknown
	}
}

func shouldRefreshHealth(link *models.Link) bool {
	if link == nil {
		return false
	}

	return link.HealthStatus.LastCheckedAt == nil || time.Since(*link.HealthStatus.LastCheckedAt) > destinationHealthMaxAge
}

func refreshLinkHealthAsync(code string) {
	go func() {
		link, err := storage.GetLink(code)
		if err != nil {
			return
		}
		if !shouldRefreshHealth(link) {
			return
		}
		if err := refreshLinkHealth(link); err != nil {
			logger.Logger.Warn("Failed to refresh link health asynchronously",
				zap.String("code", code),
				zap.Error(err),
			)
		}
	}()
}

type redirectDestination struct {
	URL    string
	Status string
}

func selectRedirectDestination(link *models.Link) string {
	destinations := make([]redirectDestination, 0, 1+len(link.RotationTargets))
	destinations = append(destinations, redirectDestination{
		URL:    link.LongURL,
		Status: link.PrimaryHealth.Status,
	})
	for _, target := range link.RotationTargets {
		if target.IsActive {
			destinations = append(destinations, redirectDestination{
				URL:    target.URL,
				Status: target.Status,
			})
		}
	}

	if len(destinations) == 1 {
		return destinations[0].URL
	}

	viableDestinations := filterBrokenDestinations(link, destinations)
	index := link.RotationCursor % len(viableDestinations)
	nextCursor := (link.RotationCursor + 1) % len(viableDestinations)

	go func(shortCode string, cursor int) {
		if err := storage.UpdateLinkRotationCursor(shortCode, cursor); err != nil {
			logger.Logger.Warn("Failed to update rotation cursor",
				zap.String("code", shortCode),
				zap.Error(err),
			)
		}
	}(link.ShortCode, nextCursor)

	return viableDestinations[index].URL
}

func filterBrokenDestinations(link *models.Link, destinations []redirectDestination) []redirectDestination {
	viable := make([]redirectDestination, 0, len(destinations))

	for _, destination := range destinations {
		if destination.Status != destinationStatusBroken {
			viable = append(viable, destination)
		}
	}

	if len(viable) == 0 {
		return destinations
	}

	return viable
}
