package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/dynamorm/pkg/session"
)

// TestUser - According to DynamORM docs, we should ONLY need dynamorm tags
type TestUser struct {
	// Keys - DynamORM docs say only dynamorm tags needed
	ID string `dynamorm:"pk" json:"id"`
	
	// Business fields
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TestUserWithBothTags - What actually works based on your debugging
type TestUserWithBothTags struct {
	// Keys - BOTH tags required
	ID string `dynamorm:"pk" json:"id"`
	
	// Business fields
	Email     string    `json:"email" `
	Name      string    `json:"name" `
	TenantID  string    `json:"tenant_id" `
	CreatedAt time.Time `json:"created_at" `
}

func main() {
	// Get table name from env or use default
	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		tableName = "test-dynamorm-table"
	}

	// Initialize DynamORM according to their docs
	config := session.Config{
		Region: "us-east-1",
	}
	
	db, err := dynamorm.New(config)
	if err != nil {
		log.Fatalf("Failed to initialize DynamORM: %v", err)
	}

	// Test 1: Try with struct as DynamORM docs show (only dynamorm tags)
	fmt.Println("=== TEST 1: Using struct as per DynamORM documentation ===")
	user1 := &TestUser{
		ID:        "test-user-123",
		Email:     "test@example.com",
		Name:      "Test User",
		TenantID:  "test-tenant",
		CreatedAt: time.Now(),
	}

	fmt.Printf("Attempting to create user with ID: %s\n", user1.ID)
	err = db.Model(user1).Create()
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
		fmt.Println("This is likely the 'Missing the key pk in the item' error")
	} else {
		fmt.Println("✅ SUCCESS: User created")
	}

	// Test 2: Try with both tags as your debugging shows is needed
	fmt.Println("\n=== TEST 2: Using struct with BOTH tags ===")
	user2 := &TestUserWithBothTags{
		ID:        "test-user-456",
		Email:     "test2@example.com",
		Name:      "Test User 2",
		TenantID:  "test-tenant",
		CreatedAt: time.Now(),
	}

	fmt.Printf("Attempting to create user with ID: %s\n", user2.ID)
	err = db.Model(user2).Create()
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
	} else {
		fmt.Println("✅ SUCCESS: User created")
	}

	// Also test raw AWS SDK to see table structure
	fmt.Println("\n=== TEST 3: Checking table structure with AWS SDK ===")
	cfg, _ := awsconfig.LoadDefaultConfig(context.Background())
	svc := dynamodb.NewFromConfig(cfg)
	
	result, err := svc.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: &tableName,
	})
	if err != nil {
		fmt.Printf("Failed to describe table: %v\n", err)
	} else {
		fmt.Println("Table key schema:")
		for _, key := range result.Table.KeySchema {
			fmt.Printf("  - %s (%s)\n", *key.AttributeName, key.KeyType)
		}
	}
}