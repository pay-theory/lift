package cli

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const (
	stringType          = "string"
	mediumSize          = "Medium"
	keysOnlyView        = "KEYS_ONLY"
	newAndOldImagesView = "NEW_AND_OLD_IMAGES"
	allView             = "ALL"
)

type dynamodbClient interface {
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}
