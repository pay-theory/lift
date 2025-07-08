package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/dynamorm/v2"
)

// TestUser model with dynamorm tags
type TestUser struct {
	// Keys - MUST have both tags
	PK string `dynamorm:"pk" `
	SK string `dynamorm:"sk" `
	
	// Business fields
	UserID    string    `json:"user_id" `
	TenantID  string    `json:"tenant_id" `
	Email     string    `json:"email" `
	Name      string    `json:"name" `
	CreatedAt time.Time `json:"created_at" `
}

func main() {
	// Get table name from env or use default
	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		tableName = "test-dynamorm-table"
	}

	// Create AWS config
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create DynamoDB client
	svc := dynamodb.NewFromConfig(cfg)

	// Initialize DynamORM v2
	db, err := dynamorm.NewClient(dynamorm.Config{
		DynamoDB: svc,
		Debug:    true, // Enable debug logging
	})
	if err != nil {
		log.Fatalf("Failed to create DynamORM client: %v", err)
	}

	// Create test user
	user := &TestUser{
		PK:        fmt.Sprintf("tenant#%s", "test-tenant"),
		SK:        fmt.Sprintf("user#%s", "test-user-123"),
		UserID:    "test-user-123",
		TenantID:  "test-tenant",
		Email:     "test@example.com",
		Name:      "Test User",
		CreatedAt: time.Now(),
	}

	// Log what we're about to write
	fmt.Printf("Writing user with:\n")
	fmt.Printf("  PK: %s\n", user.PK)
	fmt.Printf("  SK: %s\n", user.SK)
	fmt.Printf("  UserID: %s\n", user.UserID)
	fmt.Printf("  Email: %s\n", user.Email)

	// Test 1: Write the user
	fmt.Println("\n=== TEST 1: Writing user ===")
	ctx := context.Background()
	
	err = db.Table(tableName).Put(user).Run(ctx)
	if err != nil {
		log.Fatalf("Failed to write user: %v", err)
	}
	fmt.Println("✅ User written successfully")

	// Test 2: Read the user back
	fmt.Println("\n=== TEST 2: Reading user back ===")
	readUser := &TestUser{}
	
	err = db.Table(tableName).
		Get("pk", user.PK).
		Range("sk", user.SK).
		One(ctx, readUser)
	if err != nil {
		log.Fatalf("Failed to read user: %v", err)
	}
	fmt.Printf("✅ User read successfully: %+v\n", readUser)

	// Test 3: Query by tenant
	fmt.Println("\n=== TEST 3: Query by tenant ===")
	var users []TestUser
	
	err = db.Table(tableName).
		Get("pk", fmt.Sprintf("tenant#%s", "test-tenant")).
		BeginsWith("sk", "user#").
		All(ctx, &users)
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}
	fmt.Printf("✅ Found %d users\n", len(users))

	fmt.Println("\n🎉 All tests passed!")
}