module github.com/pay-theory/lift/examples/rate-limiting-limited

go 1.25

require (
	github.com/pay-theory/dynamorm v1.0.39
	github.com/pay-theory/lift v0.0.0-20250000000000-000000000000
	github.com/pay-theory/limited v1.0.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/aws/aws-lambda-go v1.51.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.4 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi v1.29.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.53.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.39.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssm v1.66.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.4 // indirect
	github.com/aws/smithy-go v1.24.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	go.uber.org/multierr v1.11.0 // indirect
)

replace github.com/pay-theory/lift => ../..
