package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
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

	return &models.User{
		ID:        result.Item["ID"].(*types.AttributeValueMemberS).Value,
		Email:     result.Item["Email"].(*types.AttributeValueMemberS).Value,
		Name:      result.Item["Name"].(*types.AttributeValueMemberS).Value,
		Picture:   result.Item["Picture"].(*types.AttributeValueMemberS).Value,
		CreatedAt: createdAt,
	}, nil
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
	}

	if link.ExpiresAt != nil {
		item["ExpiresAt"] = &types.AttributeValueMemberS{Value: link.ExpiresAt.Format(time.RFC3339)}
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

// SaveAnalyticsEvent saves an analytics event to DynamoDB
func SaveAnalyticsEvent(event *models.AnalyticsEvent) error {
	_, err := ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("Analytics"),
		Item: map[string]types.AttributeValue{
			"ShortCode": &types.AttributeValueMemberS{Value: event.ShortCode},
			"Timestamp": &types.AttributeValueMemberS{Value: event.Timestamp.Format(time.RFC3339)},
			"Referrer":  &types.AttributeValueMemberS{Value: event.Referrer},
			"UserAgent": &types.AttributeValueMemberS{Value: event.UserAgent},
			"IPAddress": &types.AttributeValueMemberS{Value: event.IPAddress},
			"Country":   &types.AttributeValueMemberS{Value: event.Country},
		},
	})

	if err != nil {
		logger.Logger.Error("Failed to save analytics", zap.Error(err))
		return err
	}

	return nil
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

		links = append(links, link)
	}

	return links, nil
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
