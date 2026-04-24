module github.com/pay-theory/lift

go 1.25

require (
	github.com/PaesslerAG/jsonpath v0.1.1
	github.com/aws/aws-cdk-go/awscdk/v2 v2.220.0
	github.com/aws/aws-lambda-go v1.51.0
	github.com/aws/aws-sdk-go-v2 v1.41.0
	github.com/aws/aws-sdk-go-v2/config v1.32.4
	github.com/aws/aws-sdk-go-v2/credentials v1.19.4
	github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign v1.9.11
	github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi v1.29.1
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.32.9
	github.com/aws/aws-sdk-go-v2/service/appconfig v1.42.9
	github.com/aws/aws-sdk-go-v2/service/appmesh v1.34.8
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.68.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.51.4
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.58.5
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.53.4
	github.com/aws/aws-sdk-go-v2/service/lambda v1.79.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.93.1
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.39.9
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.39.12
	github.com/aws/aws-sdk-go-v2/service/ses v1.34.7
	github.com/aws/aws-sdk-go-v2/service/sns v1.39.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.66.2
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.4
	github.com/aws/aws-xray-sdk-go/v2 v2.0.0
	github.com/aws/constructs-go/constructs/v10 v10.4.2
	github.com/aws/jsii-runtime-go v1.117.0
	github.com/aws/smithy-go v1.24.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/oklog/ulid/v2 v2.1.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pay-theory/dynamorm v1.0.39
	github.com/pay-theory/limited v1.0.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/zap v1.27.0
	golang.org/x/text v0.30.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/PaesslerAG/gval v1.2.4 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.12 // indirect
	github.com/cdklabs/awscdk-asset-awscli-go/awscliv1/v2 v2.2.257 // indirect
	github.com/cdklabs/awscdk-asset-node-proxy-agent-go/nodeproxyagentv6/v2 v2.1.0 // indirect
	github.com/cdklabs/cloud-assembly-schema-go/awscdkcloudassemblyschema/v48 v48.16.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.2 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.68.0 // indirect
	github.com/xyproto/randomstring v1.2.0 // indirect
	github.com/yuin/goldmark v1.7.13 // indirect
	go.opentelemetry.io/otel v1.41.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/lint v0.0.0-20241112194109-818c5a804067 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/telemetry v0.0.0-20251022145735-5be28d707443 // indirect
	golang.org/x/tools v0.38.0 // indirect
	golang.org/x/tools/cmd/godoc v0.1.0-deprecated // indirect
	golang.org/x/tools/godoc v0.1.0-deprecated // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/grpc v1.76.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
