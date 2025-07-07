package test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSQSEvent(t *testing.T) {
	// Given
	helpers := NewEventHelpers()
	messages := []SQSMessage{
		{
			ID:        "msg-1",
			Body:      `{"test": "data"}`,
			SourceARN: "arn:aws:sqs:us-east-1:123456789012:test-queue",
			Timestamp: "1234567890",
			Attributes: map[string]events.SQSMessageAttribute{
				"test-attr": {
					StringValue: strPtr("test-value"),
					DataType:    "String",
				},
			},
		},
	}
	
	// When
	event := helpers.GenerateSQSEvent(messages)
	
	// Then
	assert.Len(t, event.Records, 1)
	assert.Equal(t, "msg-1", event.Records[0].MessageId)
	assert.Equal(t, `{"test": "data"}`, event.Records[0].Body)
	assert.Equal(t, "arn:aws:sqs:us-east-1:123456789012:test-queue", event.Records[0].EventSourceARN)
	assert.NotEmpty(t, event.Records[0].ReceiptHandle)
	assert.Equal(t, "test-value", *event.Records[0].MessageAttributes["test-attr"].StringValue)
}

func TestGenerateEventBridgeEvent(t *testing.T) {
	// Given
	helpers := NewEventHelpers()
	detail := map[string]interface{}{
		"orderId": "12345",
		"status":  "processing",
	}
	
	// When
	event := helpers.GenerateEventBridgeEvent("order.service", "Order Updated", detail)
	
	// Then
	assert.Equal(t, "order.service", event.Source)
	assert.Equal(t, "Order Updated", event.DetailType)
	assert.Equal(t, "us-east-1", event.Region)
	
	var parsedDetail map[string]interface{}
	err := json.Unmarshal(event.Detail, &parsedDetail)
	require.NoError(t, err)
	assert.Equal(t, "12345", parsedDetail["orderId"])
	assert.Equal(t, "processing", parsedDetail["status"])
}

func TestGenerateS3Event(t *testing.T) {
	// Given
	helpers := NewEventHelpers()
	
	// When
	event := helpers.GenerateS3Event("test-bucket", "path/to/file.txt", "ObjectCreated:Put")
	
	// Then
	assert.Len(t, event.Records, 1)
	assert.Equal(t, "ObjectCreated:Put", event.Records[0].EventName)
	assert.Equal(t, "test-bucket", event.Records[0].S3.Bucket.Name)
	assert.Equal(t, "path/to/file.txt", event.Records[0].S3.Object.Key)
	assert.Equal(t, int64(1024), event.Records[0].S3.Object.Size)
	assert.Equal(t, "us-east-1", event.Records[0].AWSRegion)
}

func TestGenerateDynamoDBStreamEvent(t *testing.T) {
	// Given
	helpers := NewEventHelpers()
	records := []DynamoDBRecord{
		{
			ID:        "1",
			EventName: "INSERT",
			Keys: map[string]events.DynamoDBAttributeValue{
				"id": events.NewStringAttribute("123"),
			},
			NewImage: map[string]events.DynamoDBAttributeValue{
				"id":   events.NewStringAttribute("123"),
				"name": events.NewStringAttribute("Test Item"),
			},
			StreamViewType: "NEW_AND_OLD_IMAGES",
		},
	}
	
	// When
	event := helpers.GenerateDynamoDBStreamEvent("test-table", records)
	
	// Then
	assert.Len(t, event.Records, 1)
	assert.Equal(t, "INSERT", event.Records[0].EventName)
	assert.Equal(t, "aws:dynamodb", event.Records[0].EventSource)
	assert.Contains(t, event.Records[0].EventSourceArn, "test-table")
	assert.Equal(t, "123", event.Records[0].Change.Keys["id"].String())
	assert.Equal(t, "Test Item", event.Records[0].Change.NewImage["name"].String())
}

func TestGenerateSNSEvent(t *testing.T) {
	// Given
	helpers := NewEventHelpers()
	
	// When
	event := helpers.GenerateSNSEvent(
		"arn:aws:sns:us-east-1:123456789012:test-topic",
		"Test Subject",
		"Test message body",
	)
	
	// Then
	assert.Len(t, event.Records, 1)
	assert.Equal(t, "aws:sns", event.Records[0].EventSource)
	assert.Equal(t, "arn:aws:sns:us-east-1:123456789012:test-topic", event.Records[0].SNS.TopicArn)
	assert.Equal(t, "Test Subject", event.Records[0].SNS.Subject)
	assert.Equal(t, "Test message body", event.Records[0].SNS.Message)
}

func TestGenerateKinesisEvent(t *testing.T) {
	// Given
	helpers := NewEventHelpers()
	records := []KinesisRecord{
		{
			Data:           "test-data-1",
			SequenceNumber: "1234567890",
			PartitionKey:   "partition-1",
		},
		{
			Data:           "test-data-2",
			SequenceNumber: "1234567891",
			PartitionKey:   "partition-2",
		},
	}
	
	// When
	event := helpers.GenerateKinesisEvent("arn:aws:kinesis:us-east-1:123456789012:stream/test-stream", records)
	
	// Then
	assert.Len(t, event.Records, 2)
	assert.Equal(t, "aws:kinesis", event.Records[0].EventSource)
	assert.Equal(t, "test-data-1", string(event.Records[0].Kinesis.Data))
	assert.Equal(t, "1234567890", event.Records[0].Kinesis.SequenceNumber)
	assert.Equal(t, "partition-1", event.Records[0].Kinesis.PartitionKey)
}

func TestValidateEventPattern(t *testing.T) {
	// Given
	helpers := NewEventHelpers()
	pattern := map[string]interface{}{
		"source":      []string{"order.service", "payment.service"},
		"detail-type": []string{"Order Created", "Payment Processed"},
	}
	
	// When/Then - matching event
	matchingEvent := helpers.GenerateEventBridgeEvent("order.service", "Order Created", nil)
	assert.True(t, helpers.ValidateEventPattern(pattern, matchingEvent))
	
	// When/Then - non-matching source
	nonMatchingSource := helpers.GenerateEventBridgeEvent("inventory.service", "Order Created", nil)
	assert.False(t, helpers.ValidateEventPattern(pattern, nonMatchingSource))
	
	// When/Then - non-matching detail type
	nonMatchingDetail := helpers.GenerateEventBridgeEvent("order.service", "Order Updated", nil)
	assert.False(t, helpers.ValidateEventPattern(pattern, nonMatchingDetail))
}

func TestMockEventSource(t *testing.T) {
	// Given
	mock := NewMockEventSource()
	mock.AddEvent("event1")
	mock.AddEvent("event2")
	mock.AddEvent("event3")
	mock.SetDelay(10 * time.Millisecond)
	
	// When
	events := mock.Emit()
	
	// Then
	received := make([]interface{}, 0)
	for event := range events {
		received = append(received, event)
	}
	
	assert.Len(t, received, 3)
	assert.Equal(t, "event1", received[0])
	assert.Equal(t, "event2", received[1])
	assert.Equal(t, "event3", received[2])
}

func TestEventRecorder(t *testing.T) {
	// Given
	recorder := NewEventRecorder()
	
	// When
	recorder.Record("event1")
	recorder.Record("event2")
	recorder.RecordError(assert.AnError)
	
	// Then
	assert.Equal(t, 2, recorder.GetEventCount())
	assert.Equal(t, 1, recorder.GetErrorCount())
	assert.Equal(t, "event1", recorder.RecordedEvents[0])
	assert.Equal(t, "event2", recorder.RecordedEvents[1])
	assert.Equal(t, assert.AnError, recorder.Errors[0])
	
	// When
	recorder.Clear()
	
	// Then
	assert.Equal(t, 0, recorder.GetEventCount())
	assert.Equal(t, 0, recorder.GetErrorCount())
}

func TestEventReplay(t *testing.T) {
	// Given
	events := []interface{}{"event1", "event2", "event3"}
	replay := NewEventReplay(events)
	processedEvents := make([]interface{}, 0)
	
	handler := func(event interface{}) error {
		processedEvents = append(processedEvents, event)
		return nil
	}
	
	// When
	err := replay.Replay(handler)
	
	// Then
	require.NoError(t, err)
	assert.Equal(t, events, processedEvents)
}

func TestEventReplayWithDelay(t *testing.T) {
	// Given
	events := []interface{}{"event1", "event2"}
	replay := NewEventReplay(events)
	startTime := time.Now()
	
	handler := func(event interface{}) error {
		return nil
	}
	
	// When
	err := replay.ReplayWithDelay(handler, 50*time.Millisecond)
	
	// Then
	require.NoError(t, err)
	elapsed := time.Since(startTime)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

func TestEventValidator(t *testing.T) {
	// Given
	validator := NewEventValidator()
	
	// Test SQS validation
	t.Run("ValidateSQSMessage", func(t *testing.T) {
		// Valid message
		validMsg := events.SQSMessage{
			MessageId: "123",
			Body:      "test",
		}
		assert.NoError(t, validator.ValidateSQSMessage(validMsg))
		
		// Invalid - missing ID
		invalidMsg1 := events.SQSMessage{Body: "test"}
		assert.Error(t, validator.ValidateSQSMessage(invalidMsg1))
		
		// Invalid - missing body
		invalidMsg2 := events.SQSMessage{MessageId: "123"}
		assert.Error(t, validator.ValidateSQSMessage(invalidMsg2))
	})
	
	// Test EventBridge validation
	t.Run("ValidateEventBridgeEvent", func(t *testing.T) {
		// Valid event
		validEvent := events.CloudWatchEvent{
			Source:     "test.source",
			DetailType: "Test Event",
		}
		assert.NoError(t, validator.ValidateEventBridgeEvent(validEvent))
		
		// Invalid - missing source
		invalidEvent1 := events.CloudWatchEvent{DetailType: "Test Event"}
		assert.Error(t, validator.ValidateEventBridgeEvent(invalidEvent1))
		
		// Invalid - missing detail type
		invalidEvent2 := events.CloudWatchEvent{Source: "test.source"}
		assert.Error(t, validator.ValidateEventBridgeEvent(invalidEvent2))
	})
	
	// Test S3 validation
	t.Run("ValidateS3Event", func(t *testing.T) {
		// Valid event
		validEvent := events.S3Event{
			Records: []events.S3EventRecord{
				{
					S3: events.S3Entity{
						Bucket: events.S3Bucket{Name: "bucket"},
						Object: events.S3Object{Key: "key"},
					},
				},
			},
		}
		assert.NoError(t, validator.ValidateS3Event(validEvent))
		
		// Invalid - no records
		invalidEvent1 := events.S3Event{}
		assert.Error(t, validator.ValidateS3Event(invalidEvent1))
		
		// Invalid - missing bucket name
		invalidEvent2 := events.S3Event{
			Records: []events.S3EventRecord{
				{
					S3: events.S3Entity{
						Object: events.S3Object{Key: "key"},
					},
				},
			},
		}
		assert.Error(t, validator.ValidateS3Event(invalidEvent2))
	})
}

// Helper function
func strPtr(s string) *string {
	return &s
}