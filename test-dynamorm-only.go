package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// User - According to DynamORM docs, only need dynamorm tags
type User struct {
	ID    string `dynamorm:"pk" json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func main() {
	// Test what happens when we marshal with only dynamorm tags
	user := User{
		ID:    "user123",
		Email: "test@example.com",
		Name:  "Test User",
	}

	// Show what AWS SDK marshaling does
	av, err := attributevalue.MarshalMap(user)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("AWS SDK marshaled (with only dynamorm tags):")
	for k, v := range av {
		fmt.Printf("  %s: %v\n", k, v)
	}

	// Try to write to DynamoDB
	cfg, _ := config.LoadDefaultConfig(context.Background())
	svc := dynamodb.NewFromConfig(cfg)

	_, err = svc.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String("test-dynamorm-table"),
		Item:      av,
	})

	if err != nil {
		fmt.Printf("\n❌ Error writing to DynamoDB: %v\n", err)
		fmt.Println("\nThis is the issue - table expects 'pk' but AWS SDK marshaled 'ID'")
	} else {
		fmt.Println("\n✅ Success!")
	}
}