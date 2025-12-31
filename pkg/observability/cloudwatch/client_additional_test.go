package cloudwatch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/stretchr/testify/require"
)

func TestCloudWatchLogsClientImpl_ForwardsCalls(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"__type":"InvalidParameterException","message":"bad request"}`))
	}))
	defer server.Close()

	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  server.Client(),
		Retryer: func() aws.Retryer {
			return aws.NopRetryer{}
		},
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               server.URL,
				SigningRegion:     region,
				HostnameImmutable: true,
			}, nil
		}),
	}

	client := NewCloudWatchLogsClient(cfg)
	impl, ok := client.(*CloudWatchLogsClientImpl)
	require.True(t, ok)
	require.NotNil(t, impl)

	_, err := client.CreateLogGroup(context.Background(), &cloudwatchlogs.CreateLogGroupInput{LogGroupName: aws.String("g")})
	require.Error(t, err)

	_, err = client.CreateLogStream(context.Background(), &cloudwatchlogs.CreateLogStreamInput{LogGroupName: aws.String("g"), LogStreamName: aws.String("s")})
	require.Error(t, err)

	_, err = client.PutLogEvents(context.Background(), &cloudwatchlogs.PutLogEventsInput{LogGroupName: aws.String("g"), LogStreamName: aws.String("s")})
	require.Error(t, err)

	_, err = client.DescribeLogGroups(context.Background(), &cloudwatchlogs.DescribeLogGroupsInput{})
	require.Error(t, err)

	_, err = client.DescribeLogStreams(context.Background(), &cloudwatchlogs.DescribeLogStreamsInput{LogGroupName: aws.String("g")})
	require.Error(t, err)

	require.GreaterOrEqual(t, atomic.LoadInt32(&requests), int32(1))
}
