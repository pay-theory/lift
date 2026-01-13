#!/bin/bash
SRC="pkg/testing/mocks.go"
HEADER_DYNAMORM="package testing\n\nimport (\n\t\"context\"\n\t\"encoding/json\"\n\t\"fmt\"\n\t\"sync\"\n\t\"time\"\n\n\t\"github.com/pay-theory/lift/pkg/dynamorm\"\n)"
HEADER_OTHER="package testing\n\nimport (\n\t\"context\"\n\t\"encoding/json\"\n\t\"fmt\"\n\t\"sync\"\n\t\"time\"\n)"

# DynamORM
echo -e "$HEADER_DYNAMORM" > pkg/testing/mocks_dynamorm.go
sed -n '15,323p' "$SRC" >> pkg/testing/mocks_dynamorm.go

# AWS
echo -e "$HEADER_OTHER" > pkg/testing/mocks_aws.go
sed -n '324,396p' "$SRC" >> pkg/testing/mocks_aws.go

# HTTP
echo -e "$HEADER_OTHER" > pkg/testing/mocks_http.go
sed -n '397,595p' "$SRC" >> pkg/testing/mocks_http.go

# APIGateway
echo -e "$HEADER_OTHER" > pkg/testing/mocks_apigateway.go
sed -n '596,1052p' "$SRC" >> pkg/testing/mocks_apigateway.go

# CloudWatch
echo -e "$HEADER_OTHER" > pkg/testing/mocks_cloudwatch.go
sed -n '1053,1737p' "$SRC" >> pkg/testing/mocks_cloudwatch.go
