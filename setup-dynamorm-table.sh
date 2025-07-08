#!/bin/bash

# Setup script for DynamORM table

TABLE_NAME="Users"  # DynamORM convention: struct name + s
REGION="us-east-1"

echo "=== DynamORM Table Setup ==="
echo ""

# Check if table exists
echo "Checking if table exists..."
aws dynamodb describe-table --table-name $TABLE_NAME --region $REGION >/dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "Table '$TABLE_NAME' already exists."
    echo ""
    echo "Current table structure:"
    aws dynamodb describe-table --table-name $TABLE_NAME --region $REGION --query 'Table.{TableName:TableName,KeySchema:KeySchema,AttributeDefinitions:AttributeDefinitions}' --output json
else
    echo "Table '$TABLE_NAME' does not exist. Creating..."
    
    # Create table to match DynamORM expectations
    # Based on the struct: ID string `dynamorm:"pk"`
    # DynamORM will map the field name "ID" as the attribute name
    aws dynamodb create-table \
        --table-name $TABLE_NAME \
        --attribute-definitions \
            AttributeName=ID,AttributeType=S \
        --key-schema \
            AttributeName=ID,KeyType=HASH \
        --billing-mode PAY_PER_REQUEST \
        --region $REGION

    echo "Waiting for table to be active..."
    aws dynamodb wait table-exists --table-name $TABLE_NAME --region $REGION
    
    echo ""
    echo "✅ Table created successfully!"
fi

echo ""
echo "Now run: go run test-dynamorm-standalone.go"