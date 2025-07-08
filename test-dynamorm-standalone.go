package main

import (
	"fmt"
	"log"
	"time"

	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/dynamorm/pkg/session"
)

// User model - using only dynamorm tags as per their docs
type User struct {
	ID        string    `dynamorm:"pk" json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	fmt.Println("=== Testing DynamORM Standalone (No Lift) ===\n")

	// Initialize DynamORM according to their docs
	config := session.Config{
		Region: "us-east-1",
	}
	
	db, err := dynamorm.New(config)
	if err != nil {
		log.Fatalf("Failed to initialize DynamORM: %v", err)
	}

	// Create a user
	user := &User{
		ID:        "test-user-123",
		Email:     "test@example.com",
		Name:      "Test User",
		Active:    true,
		CreatedAt: time.Now(),
	}

	fmt.Printf("Creating user: %+v\n\n", user)

	// Test 1: Create the user
	fmt.Println("TEST 1: Creating user with DynamORM...")
	err = db.Model(user).Create()
	if err != nil {
		fmt.Printf("❌ Failed to create user: %v\n", err)
	} else {
		fmt.Println("✅ User created successfully")
	}

	// Test 2: Read the user back
	fmt.Println("\nTEST 2: Reading user back...")
	var readUser User
	err = db.Model(&User{}).
		Where("ID", "=", "test-user-123").
		First(&readUser)
	if err != nil {
		fmt.Printf("❌ Failed to read user: %v\n", err)
	} else {
		fmt.Printf("✅ User read successfully: %+v\n", readUser)
	}

	// Test 3: Query users
	fmt.Println("\nTEST 3: Querying all users...")
	var users []User
	err = db.Model(&User{}).
		All(&users)
	if err != nil {
		fmt.Printf("❌ Failed to query users: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d users\n", len(users))
	}

	// Test 4: Update user
	fmt.Println("\nTEST 4: Updating user...")
	readUser.Name = "Updated User"
	readUser.Active = false
	err = db.Model(&readUser).Update()
	if err != nil {
		fmt.Printf("❌ Failed to update user: %v\n", err)
	} else {
		fmt.Println("✅ User updated successfully")
	}

	// Test 5: Delete user
	fmt.Println("\nTEST 5: Deleting user...")
	err = db.Model(&User{ID: "test-user-123"}).Delete()
	if err != nil {
		fmt.Printf("❌ Failed to delete user: %v\n", err)
	} else {
		fmt.Println("✅ User deleted successfully")
	}
}