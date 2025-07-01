package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBIdempotencyStore implements IdempotencyStore using DynamoDB
type DynamoDBIdempotencyStore struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBIdempotencyStore creates a new DynamoDB-backed idempotency store
func NewDynamoDBIdempotencyStore(client *dynamodb.Client, tableName string) *DynamoDBIdempotencyStore {
	return &DynamoDBIdempotencyStore{
		client:    client,
		tableName: tableName,
	}
}

// DynamoDBRecord represents the DynamoDB item structure
type DynamoDBRecord struct {
	PK             string    `dynamodbav:"pk"`
	Status         string    `dynamodbav:"status"`
	Response       string    `dynamodbav:"response,omitempty"`
	StatusCode     int       `dynamodbav:"status_code,omitempty"`
	Error          string    `dynamodbav:"error,omitempty"`
	CreatedAt      time.Time `dynamodbav:"created_at"`
	TTL            int64     `dynamodbav:"ttl"`
	RequestHash    string    `dynamodbav:"request_hash,omitempty"`
}

// Get retrieves a stored response by key
func (d *DynamoDBIdempotencyStore) Get(ctx context.Context, key string) (*IdempotencyRecord, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	result, err := d.client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, nil
	}

	var dbRecord DynamoDBRecord
	if err := attributevalue.UnmarshalMap(result.Item, &dbRecord); err != nil {
		return nil, err
	}

	// Convert from DynamoDB record to IdempotencyRecord
	record := &IdempotencyRecord{
		Key:         key,
		Status:      dbRecord.Status,
		StatusCode:  dbRecord.StatusCode,
		Error:       dbRecord.Error,
		CreatedAt:   dbRecord.CreatedAt,
		ExpiresAt:   time.Unix(dbRecord.TTL, 0),
		RequestHash: dbRecord.RequestHash,
	}

	// Unmarshal response if present
	if dbRecord.Response != "" {
		if err := json.Unmarshal([]byte(dbRecord.Response), &record.Response); err != nil {
			// If unmarshal fails, store as string
			record.Response = dbRecord.Response
		}
	}

	return record, nil
}

// Set stores a response with the given key
func (d *DynamoDBIdempotencyStore) Set(ctx context.Context, key string, record *IdempotencyRecord) error {
	// Marshal response to JSON
	var responseJSON string
	if record.Response != nil {
		data, err := json.Marshal(record.Response)
		if err != nil {
			return err
		}
		responseJSON = string(data)
	}

	dbRecord := DynamoDBRecord{
		PK:          key,
		Status:      record.Status,
		Response:    responseJSON,
		StatusCode:  record.StatusCode,
		Error:       record.Error,
		CreatedAt:   record.CreatedAt,
		TTL:         record.ExpiresAt.Unix(),
		RequestHash: record.RequestHash,
	}

	av, err := attributevalue.MarshalMap(dbRecord)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(d.tableName),
		Item:      av,
	}

	_, err = d.client.PutItem(ctx, input)
	return err
}

// SetProcessing marks a key as being processed
func (d *DynamoDBIdempotencyStore) SetProcessing(ctx context.Context, key string, expiresAt time.Time) error {
	dbRecord := DynamoDBRecord{
		PK:        key,
		Status:    "processing",
		CreatedAt: time.Now(),
		TTL:       expiresAt.Unix(),
	}

	av, err := attributevalue.MarshalMap(dbRecord)
	if err != nil {
		return err
	}

	// Use conditional put to prevent overwriting existing records
	input := &dynamodb.PutItemInput{
		TableName: aws.String(d.tableName),
		Item:      av,
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	}

	_, err = d.client.PutItem(ctx, input)
	if err != nil {
		// Check if it's a conditional check failure
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			// Key already exists, which is fine
			return nil
		}
		return err
	}

	return nil
}

// Delete removes a key from the store
func (d *DynamoDBIdempotencyStore) Delete(ctx context.Context, key string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: key},
		},
	}

	_, err := d.client.DeleteItem(ctx, input)
	return err
}

// CreateIdempotencyTable creates the DynamoDB table for idempotency storage
// This is a helper function for initial setup
func CreateIdempotencyTable(ctx context.Context, client *dynamodb.Client, tableName string) error {
	// Create the table
	createInput := &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	}

	_, err := client.CreateTable(ctx, createInput)
	if err != nil {
		return err
	}

	// Enable TTL on the table
	ttlInput := &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(tableName),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			Enabled:       aws.Bool(true),
			AttributeName: aws.String("ttl"),
		},
	}

	_, err = client.UpdateTimeToLive(ctx, ttlInput)
	return err
}