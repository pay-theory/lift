package cloudwatch

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/require"
)

func TestMockCloudWatchLogsClient_Behavior(t *testing.T) {
	client := NewMockCloudWatchLogsClient()

	// Create log group
	_, err := client.CreateLogGroup(context.Background(), &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String("group"),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), client.GetCallCount("CreateLogGroup"))

	// Duplicate log group
	_, err = client.CreateLogGroup(context.Background(), &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String("group"),
	})
	require.Error(t, err)
	_, isAlready := err.(*types.ResourceAlreadyExistsException)
	require.True(t, isAlready)

	// Create log stream fails if group is missing
	_, err = client.CreateLogStream(context.Background(), &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String("missing"),
		LogStreamName: aws.String("stream"),
	})
	require.Error(t, err)

	// Create log stream
	_, err = client.CreateLogStream(context.Background(), &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String("group"),
		LogStreamName: aws.String("stream"),
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), client.GetCallCount("CreateLogStream"))

	// Duplicate log stream
	_, err = client.CreateLogStream(context.Background(), &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String("group"),
		LogStreamName: aws.String("stream"),
	})
	require.Error(t, err)

	// PutLogEvents fails when stream missing
	_, err = client.PutLogEvents(context.Background(), &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String("group"),
		LogStreamName: aws.String("missing"),
		LogEvents:     []types.InputLogEvent{{Message: aws.String("m")}},
	})
	require.Error(t, err)

	// PutLogEvents success
	out, err := client.PutLogEvents(context.Background(), &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String("group"),
		LogStreamName: aws.String("stream"),
		LogEvents:     []types.InputLogEvent{{Message: aws.String("m")}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, aws.ToString(out.NextSequenceToken))

	events := client.GetLogEvents()
	require.Len(t, events, 1)
	client.ClearLogEvents()
	require.Empty(t, client.GetLogEvents())

	// Describe calls
	groups, err := client.DescribeLogGroups(context.Background(), &cloudwatchlogs.DescribeLogGroupsInput{})
	require.NoError(t, err)
	require.Len(t, groups.LogGroups, 1)

	streams, err := client.DescribeLogStreams(context.Background(), &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String("group"),
	})
	require.NoError(t, err)
	require.Len(t, streams.LogStreams, 1)

	// Helpers return copies
	groupMap := client.GetLogGroups()
	require.Len(t, groupMap, 1)
	groupMap["new"] = &types.LogGroup{}
	require.Len(t, client.GetLogGroups(), 1)

	streamMap := client.GetLogStreams("group")
	require.Len(t, streamMap, 1)
	streamMap["new"] = &types.LogStream{}
	require.Len(t, client.GetLogStreams("group"), 1)
	require.Nil(t, client.GetLogStreams("missing"))

	// Error injection
	client.SetError("DescribeLogGroups", errors.New("boom"))
	_, err = client.DescribeLogGroups(context.Background(), &cloudwatchlogs.DescribeLogGroupsInput{})
	require.Error(t, err)
	client.ClearErrors()

	client.SetShouldFail("PutLogEvents", true)
	_, err = client.PutLogEvents(context.Background(), &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String("group"),
		LogStreamName: aws.String("stream"),
		LogEvents:     []types.InputLogEvent{{Message: aws.String("m")}},
	})
	require.Error(t, err)
	client.SetShouldFail("PutLogEvents", false)

	// Reset clears all state
	client.Reset()
	require.Empty(t, client.GetLogGroups())
	require.Equal(t, int64(0), client.GetCallCount("CreateLogGroup"))
}
