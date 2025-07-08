#!/bin/bash

# Local DynamoDB test setup script

TABLE_NAME="test-dynamorm-table"
REGION="us-east-1"

echo "=== DynamORM Local Test Setup ==="
echo ""

# Check if table exists
echo "Checking if table exists..."
aws dynamodb describe-table --table-name $TABLE_NAME --region $REGION >/dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "Table already exists. Deleting..."
    aws dynamodb delete-table --table-name $TABLE_NAME --region $REGION >/dev/null
    echo "Waiting for table deletion..."
    aws dynamodb wait table-not-exists --table-name $TABLE_NAME --region $REGION
fi

echo "Creating table with pk/sk structure..."
aws dynamodb create-table \
    --table-name $TABLE_NAME \
    --attribute-definitions \
        AttributeName=pk,AttributeType=S \
        AttributeName=sk,AttributeType=S \
    --key-schema \
        AttributeName=pk,KeyType=HASH \
        AttributeName=sk,KeyType=RANGE \
    --billing-mode PAY_PER_REQUEST \
    --region $REGION

echo "Waiting for table to be active..."
aws dynamodb wait table-exists --table-name $TABLE_NAME --region $REGION

echo ""
echo "✅ Table created successfully!"
echo ""
echo "Table details:"
aws dynamodb describe-table --table-name $TABLE_NAME --region $REGION --query 'Table.{TableName:TableName,KeySchema:KeySchema,AttributeDefinitions:AttributeDefinitions}' --output json

echo ""
echo "Now run: go run test-dynamorm-local.go"