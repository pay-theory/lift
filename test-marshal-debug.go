package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TestUser with both tags
type TestUser struct {
	PK       string `dynamorm:"pk" `
	SK       string `dynamorm:"sk" `
	UserID   string `json:"user_id" `
	TenantID string `json:"tenant_id" `
	Email    string `json:"email" `
}

// TestUserWrong with only dynamorm tags (WRONG)
type TestUserWrong struct {
	PK       string `dynamorm:"pk"`
	SK       string `dynamorm:"sk"`
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
}

func main() {
	fmt.Println("=== DynamoDB Marshaling Debug Test ===\n")

	// Test 1: Correct struct with both tags
	fmt.Println("TEST 1: Struct with dynamorm tags")
	user1 := TestUser{
		PK:       "tenant#123",
		SK:       "user#456",
		UserID:   "456",
		TenantID: "123",
		Email:    "test@example.com",
	}

	av1, err := attributevalue.MarshalMap(user1)
	if err != nil {
		log.Fatalf("Failed to marshal correct struct: %v", err)
	}
	
	fmt.Println("Marshaled attributes:")
	for k, v := range av1 {
		fmt.Printf("  %s: %s\n", k, attributeValueToString(v))
	}
	
	if _, hasPK := av1["pk"]; hasPK {
		fmt.Println("✅ SUCCESS: 'pk' attribute found")
	} else {
		fmt.Println("❌ FAIL: 'pk' attribute missing")
	}

	// Test 2: Wrong struct with only dynamorm tags
	fmt.Println("\nTEST 2: Struct with only dynamorm tags (WRONG)")
	user2 := TestUserWrong{
		PK:       "tenant#123",
		SK:       "user#456",
		UserID:   "456",
		TenantID: "123",
		Email:    "test@example.com",
	}

	av2, err := attributevalue.MarshalMap(user2)
	if err != nil {
		log.Fatalf("Failed to marshal wrong struct: %v", err)
	}
	
	fmt.Println("Marshaled attributes:")
	for k, v := range av2 {
		fmt.Printf("  %s: %s\n", k, attributeValueToString(v))
	}
	
	if _, hasPK := av2["pk"]; hasPK {
		fmt.Println("✅ SUCCESS: 'pk' attribute found")
	} else {
		fmt.Println("❌ FAIL: 'pk' attribute missing - This causes 'Missing the key pk in the item' error!")
	}

	fmt.Println("\n🔍 Analysis:")
	fmt.Println("- dynamorm tags tell DynamORM which fields are keys/indexes")
	fmt.Println("- DynamORM uses field names as attribute names")
	fmt.Println("- Field names (PK, SK) are used as DynamoDB attribute names")
}

func attributeValueToString(av types.AttributeValue) string {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return fmt.Sprintf("S:%s", v.Value)
	case *types.AttributeValueMemberN:
		return fmt.Sprintf("N:%s", v.Value)
	case *types.AttributeValueMemberBOOL:
		return fmt.Sprintf("BOOL:%t", v.Value)
	default:
		return fmt.Sprintf("%T", av)
	}
}