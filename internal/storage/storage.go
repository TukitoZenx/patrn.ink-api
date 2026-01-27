package storage

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"patrn.ink/internal/config"
	"patrn.ink/internal/logger"
	"patrn.ink/internal/models"
)

var rdb *redis.Client
var ddb *dynamodb.Client
var ctx = context.Background()

// InitRedis initializes Redis connection for caching
func InitRedis() error {
	rdb = redis.NewClient(&redis.Options{
		Addr:     config.AppConfig.RedisAddr,
		Password: config.AppConfig.RedisPassword,
		DB:       0,
	})

	// Test connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Logger.Info("Redis connected successfully")
	return nil
}

// InitDynamo initializes DynamoDB connection and creates tables
func InitDynamo() error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.AppConfig.AWSRegion))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Use custom endpoint for local DynamoDB
	if config.AppConfig.DynamoDBEndpoint != "" {
		ddb = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(config.AppConfig.DynamoDBEndpoint)
		})
	} else {
		ddb = dynamodb.NewFromConfig(cfg)
	}

	// Create tables if they don't exist
	if err := createTablesIfNotExist(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	logger.Logger.Info("DynamoDB initialized successfully")
	return nil
}

// PingRedis checks Redis health
func PingRedis() error {
	return rdb.Ping(ctx).Err()
}

// PingDynamo checks DynamoDB health
func PingDynamo() error {
	_, err := ddb.ListTables(ctx, &dynamodb.ListTablesInput{})
	return err
}

// GetCacheBytes returns cached bytes for a key
func GetCacheBytes(key string) ([]byte, error) {
	return rdb.Get(ctx, key).Bytes()
}

// SetCacheBytes stores bytes in cache
func SetCacheBytes(key string, value []byte, ttl time.Duration) error {
	return rdb.Set(ctx, key, value, ttl).Err()
}

// createTablesIfNotExist creates the required DynamoDB tables
func createTablesIfNotExist() error {
	tables := []struct {
		name       string
		keySchema  []types.KeySchemaElement
		attributes []types.AttributeDefinition
	}{
		{
			name: "Users",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("ID"), KeyType: types.KeyTypeHash},
			},
			attributes: []types.AttributeDefinition{
				{AttributeName: aws.String("ID"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			name: "Links",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("ShortCode"), KeyType: types.KeyTypeHash},
			},
			attributes: []types.AttributeDefinition{
				{AttributeName: aws.String("ShortCode"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			name: "Analytics",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("ShortCode"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("Timestamp"), KeyType: types.KeyTypeRange},
			},
			attributes: []types.AttributeDefinition{
				{AttributeName: aws.String("ShortCode"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("Timestamp"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		{
			name: "APITokens",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("ID"), KeyType: types.KeyTypeHash},
			},
			attributes: []types.AttributeDefinition{
				{AttributeName: aws.String("ID"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
	}

	for _, table := range tables {
		_, err := ddb.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(table.name),
		})

		if err != nil {
			// Table doesn't exist, create it
			_, err = ddb.CreateTable(ctx, &dynamodb.CreateTableInput{
				TableName:            aws.String(table.name),
				KeySchema:            table.keySchema,
				AttributeDefinitions: table.attributes,
				BillingMode:          types.BillingModePayPerRequest,
			})

			var alreadyExists *types.ResourceInUseException
			if err != nil && errors.As(err, &alreadyExists) {
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to create table %s: %w", table.name, err)
			}

			logger.Logger.Info("Created DynamoDB table", zap.String("table", table.name))
		}
	}

	return nil
}

// base62Chars for encoding IDs
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateShortCode generates a cryptographically secure random short code
func GenerateShortCode(length int) (string, error) {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(base62Chars))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random code: %w", err)
		}
		result[i] = base62Chars[num.Int64()]
	}
	return string(result), nil
}

// SaveToCache stores URL in Redis cache
func SaveToCache(code string, url string) error {
	err := rdb.Set(ctx, "url:"+code, url, 24*time.Hour).Err()
	if err != nil {
		logger.Logger.Error("Failed to save to cache", zap.Error(err), zap.String("code", code))
		return err
	}
	return nil
}

// GetFromCache retrieves URL from Redis cache
func GetFromCache(code string) (string, error) {
	url, err := rdb.Get(ctx, "url:"+code).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("not found in cache")
	}
	return url, err
}

// SaveUser saves user to DynamoDB
func SaveUser(user *models.User) error {
	_, err := ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("Users"),
		Item: map[string]types.AttributeValue{
			"ID":        &types.AttributeValueMemberS{Value: user.ID},
			"Email":     &types.AttributeValueMemberS{Value: user.Email},
			"Name":      &types.AttributeValueMemberS{Value: user.Name},
			"Picture":   &types.AttributeValueMemberS{Value: user.Picture},
			"Provider":  &types.AttributeValueMemberS{Value: user.Provider},
			"CreatedAt": &types.AttributeValueMemberS{Value: user.CreatedAt.Format(time.RFC3339)},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}
	return nil
}

// GetUser retrieves user from DynamoDB
func GetUser(id string) (*models.User, error) {
	result, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("Users"),
		Key: map[string]types.AttributeValue{
			"ID": &types.AttributeValueMemberS{Value: id},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("user not found")
	}

	createdAt, _ := time.Parse(time.RFC3339, result.Item["CreatedAt"].(*types.AttributeValueMemberS).Value)

	user := &models.User{
		ID:        result.Item["ID"].(*types.AttributeValueMemberS).Value,
		Email:     result.Item["Email"].(*types.AttributeValueMemberS).Value,
		Name:      result.Item["Name"].(*types.AttributeValueMemberS).Value,
		Picture:   result.Item["Picture"].(*types.AttributeValueMemberS).Value,
		CreatedAt: createdAt,
	}

	if provider, ok := result.Item["Provider"]; ok {
		user.Provider = provider.(*types.AttributeValueMemberS).Value
	}

	return user, nil
}

// SaveLink saves a link to DynamoDB
func SaveLink(link *models.Link) error {
	item := map[string]types.AttributeValue{
		"ShortCode":   &types.AttributeValueMemberS{Value: link.ShortCode},
		"LongURL":     &types.AttributeValueMemberS{Value: link.LongURL},
		"UserID":      &types.AttributeValueMemberS{Value: link.UserID},
		"CustomAlias": &types.AttributeValueMemberBOOL{Value: link.CustomAlias},
		"Clicks":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", link.Clicks)},
		"CreatedAt":   &types.AttributeValueMemberS{Value: link.CreatedAt.Format(time.RFC3339)},
		"IsActive":    &types.AttributeValueMemberBOOL{Value: link.IsActive},
		"IsArchived":  &types.AttributeValueMemberBOOL{Value: link.IsArchived},
	}

	if link.ExpiresAt != nil {
		item["ExpiresAt"] = &types.AttributeValueMemberS{Value: link.ExpiresAt.Format(time.RFC3339)}
	}

	if link.ScheduledAt != nil {
		item["ScheduledAt"] = &types.AttributeValueMemberS{Value: link.ScheduledAt.Format(time.RFC3339)}
	}

	if len(link.Tags) > 0 {
		tagsList := make([]types.AttributeValue, len(link.Tags))
		for i, tag := range link.Tags {
			tagsList[i] = &types.AttributeValueMemberS{Value: tag}
		}
		item["Tags"] = &types.AttributeValueMemberL{Value: tagsList}
	}

	if link.Password != "" {
		item["Password"] = &types.AttributeValueMemberS{Value: link.Password}
	}

	if link.Title != "" {
		item["Title"] = &types.AttributeValueMemberS{Value: link.Title}
	}

	if link.Description != "" {
		item["Description"] = &types.AttributeValueMemberS{Value: link.Description}
	}

	if link.AgeVerification > 0 {
		item["AgeVerification"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", link.AgeVerification)}
	}

	_, err := ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("Links"),
		Item:      item,
	})

	if err != nil {
		return fmt.Errorf("failed to save link: %w", err)
	}

	// Also cache it
	_ = SaveToCache(link.ShortCode, link.LongURL)

	return nil
}

// GetLink retrieves a link from DynamoDB
func GetLink(shortCode string) (*models.Link, error) {
	result, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("Links"),
		Key: map[string]types.AttributeValue{
			"ShortCode": &types.AttributeValueMemberS{Value: shortCode},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get link: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("link not found")
	}

	link := &models.Link{
		ShortCode:   result.Item["ShortCode"].(*types.AttributeValueMemberS).Value,
		LongURL:     result.Item["LongURL"].(*types.AttributeValueMemberS).Value,
		UserID:      result.Item["UserID"].(*types.AttributeValueMemberS).Value,
		CustomAlias: result.Item["CustomAlias"].(*types.AttributeValueMemberBOOL).Value,
		IsActive:    result.Item["IsActive"].(*types.AttributeValueMemberBOOL).Value,
	}

	clicks, _ := result.Item["Clicks"].(*types.AttributeValueMemberN)
	fmt.Sscanf(clicks.Value, "%d", &link.Clicks)

	createdAt, _ := time.Parse(time.RFC3339, result.Item["CreatedAt"].(*types.AttributeValueMemberS).Value)
	link.CreatedAt = createdAt

	if expiresAtAttr, ok := result.Item["ExpiresAt"]; ok {
		expiresAt, _ := time.Parse(time.RFC3339, expiresAtAttr.(*types.AttributeValueMemberS).Value)
		link.ExpiresAt = &expiresAt
	}

	if scheduledAtAttr, ok := result.Item["ScheduledAt"]; ok {
		scheduledAt, _ := time.Parse(time.RFC3339, scheduledAtAttr.(*types.AttributeValueMemberS).Value)
		link.ScheduledAt = &scheduledAt
	}

	if isArchivedAttr, ok := result.Item["IsArchived"]; ok {
		link.IsArchived = isArchivedAttr.(*types.AttributeValueMemberBOOL).Value
	}

	if tagsAttr, ok := result.Item["Tags"]; ok {
		tagsList := tagsAttr.(*types.AttributeValueMemberL).Value
		link.Tags = make([]string, len(tagsList))
		for i, tag := range tagsList {
			link.Tags[i] = tag.(*types.AttributeValueMemberS).Value
		}
	}

	if passwordAttr, ok := result.Item["Password"]; ok {
		link.Password = passwordAttr.(*types.AttributeValueMemberS).Value
	}

	if titleAttr, ok := result.Item["Title"]; ok {
		link.Title = titleAttr.(*types.AttributeValueMemberS).Value
	}

	if descAttr, ok := result.Item["Description"]; ok {
		link.Description = descAttr.(*types.AttributeValueMemberS).Value
	}

	if ageVerifyAttr, ok := result.Item["AgeVerification"]; ok {
		var ageVal int
		fmt.Sscanf(ageVerifyAttr.(*types.AttributeValueMemberN).Value, "%d", &ageVal)
		link.AgeVerification = models.AgeVerification(ageVal)
	}

	return link, nil
}

// IncrementClicks increments the click count for a link
func IncrementClicks(shortCode string) error {
	_, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("Links"),
		Key: map[string]types.AttributeValue{
			"ShortCode": &types.AttributeValueMemberS{Value: shortCode},
		},
		UpdateExpression: aws.String("SET Clicks = Clicks + :inc"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
		},
	})

	return err
}

// GetUserLinks retrieves all links for a specific user
func GetUserLinks(userID string) ([]*models.Link, error) {
	// Note: In production, you'd want to add a GSI on UserID for efficient querying
	// For now, we'll do a scan (not ideal for large datasets)
	result, err := ddb.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String("Links"),
		FilterExpression: aws.String("UserID = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get user links: %w", err)
	}

	links := make([]*models.Link, 0, len(result.Items))
	for _, item := range result.Items {
		link := parseLinkFromItem(item)
		links = append(links, link)
	}

	return links, nil
}

// GetUserLinksWithQuery retrieves links with search, filtering, and pagination
func GetUserLinksWithQuery(userID string, query *models.LinksQuery) (*models.PaginatedLinks, error) {
	// Get all user links first (in production, use GSI with proper pagination)
	allLinks, err := GetUserLinks(userID)
	if err != nil {
		return nil, err
	}

	// Filter links
	filteredLinks := make([]*models.Link, 0)
	for _, link := range allLinks {
		// Filter by archived status
		if query.Archived != nil {
			if *query.Archived != link.IsArchived {
				continue
			}
		} else {
			// By default, don't show archived links
			if link.IsArchived {
				continue
			}
		}

		// Filter by search term
		if query.Search != "" {
			searchLower := strings.ToLower(query.Search)
			if !strings.Contains(strings.ToLower(link.LongURL), searchLower) &&
				!strings.Contains(strings.ToLower(link.ShortCode), searchLower) &&
				!strings.Contains(strings.ToLower(link.Title), searchLower) {
				continue
			}
		}

		// Filter by tags
		if len(query.Tags) > 0 {
			hasTag := false
			for _, queryTag := range query.Tags {
				for _, linkTag := range link.Tags {
					if strings.EqualFold(linkTag, queryTag) {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		filteredLinks = append(filteredLinks, link)
	}

	// Sort links
	sort.Slice(filteredLinks, func(i, j int) bool {
		switch query.SortBy {
		case "clicks":
			if query.SortOrder == "asc" {
				return filteredLinks[i].Clicks < filteredLinks[j].Clicks
			}
			return filteredLinks[i].Clicks > filteredLinks[j].Clicks
		case "expires_at":
			if filteredLinks[i].ExpiresAt == nil {
				return false
			}
			if filteredLinks[j].ExpiresAt == nil {
				return true
			}
			if query.SortOrder == "asc" {
				return filteredLinks[i].ExpiresAt.Before(*filteredLinks[j].ExpiresAt)
			}
			return filteredLinks[i].ExpiresAt.After(*filteredLinks[j].ExpiresAt)
		default: // created_at
			if query.SortOrder == "asc" {
				return filteredLinks[i].CreatedAt.Before(filteredLinks[j].CreatedAt)
			}
			return filteredLinks[i].CreatedAt.After(filteredLinks[j].CreatedAt)
		}
	})

	// Calculate pagination
	total := len(filteredLinks)
	totalPages := (total + query.Limit - 1) / query.Limit
	start := (query.Page - 1) * query.Limit
	end := start + query.Limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &models.PaginatedLinks{
		Links:      filteredLinks[start:end],
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
	}, nil
}

// parseLinkFromItem parses a DynamoDB item into a Link struct
func parseLinkFromItem(item map[string]types.AttributeValue) *models.Link {
	link := &models.Link{
		ShortCode:   item["ShortCode"].(*types.AttributeValueMemberS).Value,
		LongURL:     item["LongURL"].(*types.AttributeValueMemberS).Value,
		UserID:      item["UserID"].(*types.AttributeValueMemberS).Value,
		CustomAlias: item["CustomAlias"].(*types.AttributeValueMemberBOOL).Value,
		IsActive:    item["IsActive"].(*types.AttributeValueMemberBOOL).Value,
	}

	clicks, _ := item["Clicks"].(*types.AttributeValueMemberN)
	fmt.Sscanf(clicks.Value, "%d", &link.Clicks)

	createdAt, _ := time.Parse(time.RFC3339, item["CreatedAt"].(*types.AttributeValueMemberS).Value)
	link.CreatedAt = createdAt

	if expiresAtAttr, ok := item["ExpiresAt"]; ok {
		expiresAt, _ := time.Parse(time.RFC3339, expiresAtAttr.(*types.AttributeValueMemberS).Value)
		link.ExpiresAt = &expiresAt
	}

	if scheduledAtAttr, ok := item["ScheduledAt"]; ok {
		scheduledAt, _ := time.Parse(time.RFC3339, scheduledAtAttr.(*types.AttributeValueMemberS).Value)
		link.ScheduledAt = &scheduledAt
	}

	if isArchivedAttr, ok := item["IsArchived"]; ok {
		link.IsArchived = isArchivedAttr.(*types.AttributeValueMemberBOOL).Value
	}

	if tagsAttr, ok := item["Tags"]; ok {
		tagsList := tagsAttr.(*types.AttributeValueMemberL).Value
		link.Tags = make([]string, len(tagsList))
		for i, tag := range tagsList {
			link.Tags[i] = tag.(*types.AttributeValueMemberS).Value
		}
	}

	if passwordAttr, ok := item["Password"]; ok {
		link.Password = passwordAttr.(*types.AttributeValueMemberS).Value
	}

	if titleAttr, ok := item["Title"]; ok {
		link.Title = titleAttr.(*types.AttributeValueMemberS).Value
	}

	if descAttr, ok := item["Description"]; ok {
		link.Description = descAttr.(*types.AttributeValueMemberS).Value
	}

	return link
}

// DeleteLink soft-deletes a link
func DeleteLink(shortCode string, userID string) error {
	// First verify ownership
	link, err := GetLink(shortCode)
	if err != nil {
		return err
	}

	if link.UserID != userID {
		return fmt.Errorf("unauthorized: link belongs to different user")
	}

	// Soft delete by setting IsActive to false
	_, err = ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("Links"),
		Key: map[string]types.AttributeValue{
			"ShortCode": &types.AttributeValueMemberS{Value: shortCode},
		},
		UpdateExpression: aws.String("SET IsActive = :inactive"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inactive": &types.AttributeValueMemberBOOL{Value: false},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}

	// Remove from cache
	_ = rdb.Del(ctx, "url:"+shortCode).Err()

	return nil
}

// ArchiveLink archives a link (soft archive, different from delete)
func ArchiveLink(shortCode string, userID string) error {
	link, err := GetLink(shortCode)
	if err != nil {
		return err
	}

	if link.UserID != userID {
		return fmt.Errorf("unauthorized: link belongs to different user")
	}

	_, err = ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("Links"),
		Key: map[string]types.AttributeValue{
			"ShortCode": &types.AttributeValueMemberS{Value: shortCode},
		},
		UpdateExpression: aws.String("SET IsArchived = :archived"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":archived": &types.AttributeValueMemberBOOL{Value: true},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to archive link: %w", err)
	}

	return nil
}

// DeleteFromCache removes a URL from Redis cache
func DeleteFromCache(code string) error {
	return rdb.Del(ctx, "url:"+code).Err()
}

// GenerateUniqueShortCode generates a unique short code with collision checking
func GenerateUniqueShortCode(length int, maxAttempts int) (string, error) {
	for attempts := 0; attempts < maxAttempts; attempts++ {
		shortCode, err := GenerateShortCode(length)
		if err != nil {
			return "", err
		}

		existing, _ := GetLink(shortCode)
		if existing == nil || !existing.IsActive {
			return shortCode, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique short code after %d attempts", maxAttempts)
}

// GetAnalyticsEvents retrieves analytics events for a link within a date range
func GetAnalyticsEvents(shortCode, startDate, endDate string) ([]*models.AnalyticsEvent, error) {
	result, err := ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String("Analytics"),
		KeyConditionExpression: aws.String("ShortCode = :code AND #ts BETWEEN :start AND :end"),
		ExpressionAttributeNames: map[string]string{
			"#ts": "Timestamp",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":code":  &types.AttributeValueMemberS{Value: shortCode},
			":start": &types.AttributeValueMemberS{Value: startDate + "T00:00:00Z"},
			":end":   &types.AttributeValueMemberS{Value: endDate + "T23:59:59Z"},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query analytics: %w", err)
	}

	events := make([]*models.AnalyticsEvent, 0, len(result.Items))
	for _, item := range result.Items {
		event := &models.AnalyticsEvent{
			ShortCode: item["ShortCode"].(*types.AttributeValueMemberS).Value,
		}

		if ts, ok := item["Timestamp"]; ok {
			event.Timestamp, _ = time.Parse(time.RFC3339, ts.(*types.AttributeValueMemberS).Value)
		}
		if ref, ok := item["Referrer"]; ok {
			event.Referrer = ref.(*types.AttributeValueMemberS).Value
		}
		if ua, ok := item["UserAgent"]; ok {
			event.UserAgent = ua.(*types.AttributeValueMemberS).Value
		}
		if ip, ok := item["IPAddress"]; ok {
			event.IPAddress = ip.(*types.AttributeValueMemberS).Value
		}
		if country, ok := item["Country"]; ok {
			event.Country = country.(*types.AttributeValueMemberS).Value
		}
		if dt, ok := item["DeviceType"]; ok {
			event.DeviceType = dt.(*types.AttributeValueMemberS).Value
		}
		if browser, ok := item["Browser"]; ok {
			event.Browser = browser.(*types.AttributeValueMemberS).Value
		}

		events = append(events, event)
	}

	return events, nil
}

// SaveAnalyticsEvent saves an analytics event to DynamoDB (updated with new fields)
func SaveAnalyticsEvent(event *models.AnalyticsEvent) error {
	item := map[string]types.AttributeValue{
		"ShortCode": &types.AttributeValueMemberS{Value: event.ShortCode},
		"Timestamp": &types.AttributeValueMemberS{Value: event.Timestamp.Format(time.RFC3339)},
		"Referrer":  &types.AttributeValueMemberS{Value: event.Referrer},
		"UserAgent": &types.AttributeValueMemberS{Value: event.UserAgent},
		"IPAddress": &types.AttributeValueMemberS{Value: event.IPAddress},
		"Country":   &types.AttributeValueMemberS{Value: event.Country},
	}

	if event.DeviceType != "" {
		item["DeviceType"] = &types.AttributeValueMemberS{Value: event.DeviceType}
	}
	if event.Browser != "" {
		item["Browser"] = &types.AttributeValueMemberS{Value: event.Browser}
	}

	_, err := ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("Analytics"),
		Item:      item,
	})

	if err != nil {
		logger.Logger.Error("Failed to save analytics", zap.Error(err))
		return err
	}

	return nil
}

// ============ API Token Functions ============

// SaveAPIToken saves an API token to DynamoDB
func SaveAPIToken(token *models.APIToken) error {
	scopesList := make([]types.AttributeValue, len(token.Scopes))
	for i, scope := range token.Scopes {
		scopesList[i] = &types.AttributeValueMemberS{Value: scope}
	}

	item := map[string]types.AttributeValue{
		"ID":          &types.AttributeValueMemberS{Value: token.ID},
		"UserID":      &types.AttributeValueMemberS{Value: token.UserID},
		"Name":        &types.AttributeValueMemberS{Value: token.Name},
		"TokenHash":   &types.AttributeValueMemberS{Value: token.TokenHash},
		"TokenPrefix": &types.AttributeValueMemberS{Value: token.TokenPrefix},
		"Scopes":      &types.AttributeValueMemberL{Value: scopesList},
		"RateLimit":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", token.RateLimit)},
		"CreatedAt":   &types.AttributeValueMemberS{Value: token.CreatedAt.Format(time.RFC3339)},
		"IsActive":    &types.AttributeValueMemberBOOL{Value: token.IsActive},
	}

	if token.ExpiresAt != nil {
		item["ExpiresAt"] = &types.AttributeValueMemberS{Value: token.ExpiresAt.Format(time.RFC3339)}
	}

	if token.LastUsedAt != nil {
		item["LastUsedAt"] = &types.AttributeValueMemberS{Value: token.LastUsedAt.Format(time.RFC3339)}
	}

	_, err := ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("APITokens"),
		Item:      item,
	})

	if err != nil {
		return fmt.Errorf("failed to save API token: %w", err)
	}

	// Also store token hash -> ID mapping in Redis for fast lookup
	_ = rdb.Set(ctx, "token:"+token.TokenHash, token.ID, 0).Err()

	return nil
}

// GetUserAPITokens retrieves all API tokens for a user
func GetUserAPITokens(userID string) ([]*models.APIToken, error) {
	result, err := ddb.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String("APITokens"),
		FilterExpression: aws.String("UserID = :uid AND IsActive = :active"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid":    &types.AttributeValueMemberS{Value: userID},
			":active": &types.AttributeValueMemberBOOL{Value: true},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get API tokens: %w", err)
	}

	tokens := make([]*models.APIToken, 0, len(result.Items))
	for _, item := range result.Items {
		token := parseAPITokenFromItem(item)
		tokens = append(tokens, token)
	}

	return tokens, nil
}

// GetAPITokenByHash retrieves an API token by its hash
func GetAPITokenByHash(tokenHash string) (*models.APIToken, error) {
	// Try to get token ID from Redis cache
	tokenID, err := rdb.Get(ctx, "token:"+tokenHash).Result()
	if err == nil && tokenID != "" {
		return GetAPITokenByID(tokenID)
	}

	// Fall back to scanning (not ideal, but works)
	result, err := ddb.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String("APITokens"),
		FilterExpression: aws.String("TokenHash = :hash AND IsActive = :active"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":hash":   &types.AttributeValueMemberS{Value: tokenHash},
			":active": &types.AttributeValueMemberBOOL{Value: true},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("token not found")
	}

	token := parseAPITokenFromItem(result.Items[0])

	// Check if expired
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	// Update last used time
	go updateTokenLastUsed(token.ID)

	return token, nil
}

// GetAPITokenByID retrieves an API token by its ID
func GetAPITokenByID(tokenID string) (*models.APIToken, error) {
	result, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("APITokens"),
		Key: map[string]types.AttributeValue{
			"ID": &types.AttributeValueMemberS{Value: tokenID},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("token not found")
	}

	return parseAPITokenFromItem(result.Item), nil
}

// RevokeAPIToken revokes an API token
func RevokeAPIToken(tokenID, userID string) error {
	token, err := GetAPITokenByID(tokenID)
	if err != nil {
		return fmt.Errorf("token not found")
	}

	if token.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	_, err = ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("APITokens"),
		Key: map[string]types.AttributeValue{
			"ID": &types.AttributeValueMemberS{Value: tokenID},
		},
		UpdateExpression: aws.String("SET IsActive = :inactive"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inactive": &types.AttributeValueMemberBOOL{Value: false},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	// Remove from Redis cache
	_ = rdb.Del(ctx, "token:"+token.TokenHash).Err()

	return nil
}

// UpdateAPITokenRateLimit updates the rate limit for an API token
func UpdateAPITokenRateLimit(tokenID, userID string, rateLimit int) error {
	token, err := GetAPITokenByID(tokenID)
	if err != nil {
		return err
	}

	if token.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	_, err = ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("APITokens"),
		Key: map[string]types.AttributeValue{
			"ID": &types.AttributeValueMemberS{Value: tokenID},
		},
		UpdateExpression: aws.String("SET RateLimit = :limit"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":limit": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", rateLimit)},
		},
	})

	return err
}

// updateTokenLastUsed updates the last used time for a token
func updateTokenLastUsed(tokenID string) {
	_, _ = ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("APITokens"),
		Key: map[string]types.AttributeValue{
			"ID": &types.AttributeValueMemberS{Value: tokenID},
		},
		UpdateExpression: aws.String("SET LastUsedAt = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})
}

// parseAPITokenFromItem parses a DynamoDB item into an APIToken struct
func parseAPITokenFromItem(item map[string]types.AttributeValue) *models.APIToken {
	token := &models.APIToken{
		ID:          item["ID"].(*types.AttributeValueMemberS).Value,
		UserID:      item["UserID"].(*types.AttributeValueMemberS).Value,
		Name:        item["Name"].(*types.AttributeValueMemberS).Value,
		TokenHash:   item["TokenHash"].(*types.AttributeValueMemberS).Value,
		TokenPrefix: item["TokenPrefix"].(*types.AttributeValueMemberS).Value,
		IsActive:    item["IsActive"].(*types.AttributeValueMemberBOOL).Value,
	}

	if rateLimit, ok := item["RateLimit"]; ok {
		fmt.Sscanf(rateLimit.(*types.AttributeValueMemberN).Value, "%d", &token.RateLimit)
	}

	if createdAt, ok := item["CreatedAt"]; ok {
		token.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.(*types.AttributeValueMemberS).Value)
	}

	if expiresAt, ok := item["ExpiresAt"]; ok {
		t, _ := time.Parse(time.RFC3339, expiresAt.(*types.AttributeValueMemberS).Value)
		token.ExpiresAt = &t
	}

	if lastUsedAt, ok := item["LastUsedAt"]; ok {
		t, _ := time.Parse(time.RFC3339, lastUsedAt.(*types.AttributeValueMemberS).Value)
		token.LastUsedAt = &t
	}

	if scopes, ok := item["Scopes"]; ok {
		scopesList := scopes.(*types.AttributeValueMemberL).Value
		token.Scopes = make([]string, len(scopesList))
		for i, s := range scopesList {
			token.Scopes[i] = s.(*types.AttributeValueMemberS).Value
		}
	}

	return token
}

// ============ Cache Helper Functions ============

// GetCacheJSON retrieves and unmarshals JSON from cache
func GetCacheJSON(key string) (map[string]interface{}, error) {
	data, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// SetCacheJSON marshals and stores JSON in cache
func SetCacheJSON(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return rdb.Set(ctx, key, data, ttl).Err()
}
