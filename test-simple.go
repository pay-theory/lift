package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Test struct with both tags as you discovered is needed
type User struct {
	PK       string `dynamorm:"pk" `
	SK       string `dynamorm:"sk" `
	UserID   string `json:"user_id" `
	Email    string `json:"email" `
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}

	svc := dynamodb.NewFromConfig(cfg)
	
	user := User{
		PK:     "user#123",
		SK:     "user#123",
		UserID: "123",
		Email:  "test@example.com",
	}

	av, err := attributevalue.MarshalMap(user)
	if err != nil {
		panic(err)
	}

	fmt.Println("Marshaled item:")
	for k, v := range av {
		fmt.Printf("  %s: %v\n", k, v)
	}

	_, err = svc.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String("test-dynamorm-table"),
		Item:      av,
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Success!")
	}
}