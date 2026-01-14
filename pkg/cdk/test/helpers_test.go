package test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
)

func TestTestStack(t *testing.T) {
	ts := NewTestStack()
	require.NotNil(t, ts)
	require.NotNil(t, ts.Stack())
}

func TestEventHelpers_GenerateSQSEvent(t *testing.T) {
	helpers := NewEventHelpers()

	msgs := []SQSMessage{
		{
			ID:        "m1",
			Body:      "b1",
			SourceARN: "arn:aws:sqs:us-east-1:123456789012:q",
			Timestamp: "123",
			Attributes: map[string]events.SQSMessageAttribute{
				"attr": {StringValue: ptr("v")},
			},
		},
	}

	ev := helpers.GenerateSQSEvent(msgs)
	require.Len(t, ev.Records, 1)
	require.Equal(t, "m1", ev.Records[0].MessageId)
	require.Equal(t, "b1", ev.Records[0].Body)
	require.Equal(t, msgs[0].SourceARN, ev.Records[0].EventSourceARN)
	require.Equal(t, "v", *ev.Records[0].MessageAttributes["attr"].StringValue)
}

func TestEventHelpers_GenerateEventBridgeEventAndPatternValidation(t *testing.T) {
	helpers := NewEventHelpers()

	ev := helpers.GenerateEventBridgeEvent("my.source", "MyType", map[string]any{"a": "b"})
	require.True(t, strings.HasPrefix(ev.ID, "test-event-"))
	require.Equal(t, "my.source", ev.Source)
	require.Equal(t, "MyType", ev.DetailType)

	ok := helpers.ValidateEventPattern(map[string]interface{}{
		"source":      []string{"my.source"},
		"detail-type": []string{"MyType"},
	}, ev)
	require.True(t, ok)

	ok = helpers.ValidateEventPattern(map[string]interface{}{
		"source": []string{"other"},
	}, ev)
	require.False(t, ok)

	// Marshal failure fallback branch.
	ev = helpers.GenerateEventBridgeEvent("my.source", "MyType", make(chan int))
	require.Equal(t, "my.source", ev.Source)
	require.Equal(t, "MyType", ev.DetailType)
}

func TestEventHelpers_GenerateS3Event(t *testing.T) {
	helpers := NewEventHelpers()

	ev := helpers.GenerateS3Event("bucket", "key", "ObjectCreated:Put")
	require.Len(t, ev.Records, 1)
	require.Equal(t, "ObjectCreated:Put", ev.Records[0].EventName)
	require.Equal(t, "bucket", ev.Records[0].S3.Bucket.Name)
	require.Equal(t, "key", ev.Records[0].S3.Object.Key)
}

func TestEventHelpers_GenerateDynamoDBStreamEvent(t *testing.T) {
	helpers := NewEventHelpers()

	ev := helpers.GenerateDynamoDBStreamEvent("tbl", []DynamoDBRecord{
		{
			ID:        "1",
			EventName: "INSERT",
			Keys: map[string]events.DynamoDBAttributeValue{
				"PK": events.NewStringAttribute("pk"),
			},
			NewImage: map[string]events.DynamoDBAttributeValue{
				"PK": events.NewStringAttribute("pk"),
			},
			StreamViewType: "NEW_IMAGE",
		},
	})

	require.Len(t, ev.Records, 1)
	require.Equal(t, "INSERT", ev.Records[0].EventName)
	require.Equal(t, "pk", ev.Records[0].Change.Keys["PK"].String())
}

func TestEventHelpers_GenerateSNSEvent(t *testing.T) {
	helpers := NewEventHelpers()

	ev := helpers.GenerateSNSEvent("arn:aws:sns:us-east-1:1:t", "subj", "msg")
	require.Len(t, ev.Records, 1)
	require.Equal(t, "arn:aws:sns:us-east-1:1:t", ev.Records[0].SNS.TopicArn)
	require.Equal(t, "subj", ev.Records[0].SNS.Subject)
	require.Equal(t, "msg", ev.Records[0].SNS.Message)
}

func TestEventHelpers_GenerateKinesisEvent(t *testing.T) {
	helpers := NewEventHelpers()

	ev := helpers.GenerateKinesisEvent("arn:aws:kinesis:us-east-1:1:stream/s", []KinesisRecord{
		{Data: "d", SequenceNumber: "1", PartitionKey: "p"},
	})
	require.Len(t, ev.Records, 1)
	require.Equal(t, "arn:aws:kinesis:us-east-1:1:stream/s", ev.Records[0].EventSourceArn)
	require.Equal(t, "1", ev.Records[0].Kinesis.SequenceNumber)
}

func TestMockEventSource_Emit(t *testing.T) {
	source := NewMockEventSource()
	source.AddEvent("a")
	source.AddEvent("b")

	ch := source.Emit()
	require.Equal(t, "a", <-ch)
	require.Equal(t, "b", <-ch)
	_, ok := <-ch
	require.False(t, ok)

	source = NewMockEventSource()
	source.AddEvent("a")
	source.SetDelay(1 * time.Nanosecond)
	ch = source.Emit()
	require.Equal(t, "a", <-ch)
	_, ok = <-ch
	require.False(t, ok)
}

func TestEventRecorder(t *testing.T) {
	rec := NewEventRecorder()
	require.Equal(t, 0, rec.GetEventCount())
	require.Equal(t, 0, rec.GetErrorCount())

	sentinel := errors.New("sentinel")
	rec.Record("e1")
	rec.RecordError(sentinel)
	require.Equal(t, 1, rec.GetEventCount())
	require.Equal(t, 1, rec.GetErrorCount())

	rec.Clear()
	require.Equal(t, 0, rec.GetEventCount())
	require.Equal(t, 0, rec.GetErrorCount())
}

func TestEventReplay(t *testing.T) {
	replay := NewEventReplay([]interface{}{"a", "b"})

	var seen []string
	require.NoError(t, replay.Replay(func(event interface{}) error {
		seen = append(seen, event.(string))
		return nil
	}))
	require.Equal(t, []string{"a", "b"}, seen)

	sentinel := errors.New("sentinel")
	require.ErrorIs(t, replay.Replay(func(_ interface{}) error { return sentinel }), sentinel)

	seen = nil
	require.NoError(t, replay.ReplayWithDelay(func(event interface{}) error {
		seen = append(seen, event.(string))
		return nil
	}, 1*time.Nanosecond))
	require.Equal(t, []string{"a", "b"}, seen)
}

func TestEventValidator(t *testing.T) {
	v := NewEventValidator()

	require.Error(t, v.ValidateSQSMessage(events.SQSMessage{}))
	require.Error(t, v.ValidateSQSMessage(events.SQSMessage{MessageId: "id"}))
	require.NoError(t, v.ValidateSQSMessage(events.SQSMessage{MessageId: "id", Body: "body"}))

	require.Error(t, v.ValidateEventBridgeEvent(events.CloudWatchEvent{}))
	require.Error(t, v.ValidateEventBridgeEvent(events.CloudWatchEvent{Source: "s"}))
	require.NoError(t, v.ValidateEventBridgeEvent(events.CloudWatchEvent{Source: "s", DetailType: "dt"}))

	require.Error(t, v.ValidateS3Event(events.S3Event{}))
	require.Error(t, v.ValidateS3Event(events.S3Event{Records: []events.S3EventRecord{{}}}))
	require.NoError(t, v.ValidateS3Event(events.S3Event{Records: []events.S3EventRecord{
		{S3: events.S3Entity{Bucket: events.S3Bucket{Name: "b"}, Object: events.S3Object{Key: "k"}}},
	}}))
}

func ptr[T any](v T) *T { return &v }
