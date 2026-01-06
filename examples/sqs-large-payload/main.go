package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/lift/pkg/lift"
	sqslargepayload "github.com/pay-theory/lift/pkg/services/sqs_large_payload"
)

type job struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type memoryStore struct {
	objects map[sqslargepayload.Ref][]byte
}

func newMemoryStore() *memoryStore {
	return &memoryStore{objects: make(map[sqslargepayload.Ref][]byte)}
}

func (s *memoryStore) Put(_ context.Context, ref sqslargepayload.Ref, payload []byte) error {
	if ref.Bucket == "" || ref.Key == "" {
		return errors.New("memory store ref must include bucket and key")
	}
	if payload == nil {
		payload = []byte{}
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	s.objects[ref] = cp
	return nil
}

func (s *memoryStore) Get(_ context.Context, ref sqslargepayload.Ref) ([]byte, error) {
	payload, ok := s.objects[ref]
	if !ok {
		return nil, errors.New("memory store object not found")
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	return cp, nil
}

func (s *memoryStore) Delete(_ context.Context, ref sqslargepayload.Ref) error {
	// Mirror S3 semantics: deleting a missing object is not an error.
	delete(s.objects, ref)
	return nil
}

func (s *memoryStore) keys() []string {
	out := make([]string, 0, len(s.objects))
	for ref := range s.objects {
		out = append(out, fmt.Sprintf("%s/%s", ref.Bucket, ref.Key))
	}
	sort.Strings(out)
	return out
}

func produce(ctx context.Context, store sqslargepayload.ObjectStore, bucket, prefix, messageID string, payload []byte) (events.SQSMessage, error) {
	key := fmt.Sprintf("%s%s.json", prefix, messageID)

	body, envelope, err := sqslargepayload.OffloadIfNeeded(
		ctx,
		store,
		sqslargepayload.Ref{Bucket: bucket, Key: key},
		payload,
		sqslargepayload.DefaultOptions(),
	)
	if err != nil {
		return events.SQSMessage{}, err
	}

	if envelope != nil {
		fmt.Printf("producer: offloaded messageId=%s bytes=%d -> s3://%s/%s\n", messageID, len(payload), envelope.Bucket, envelope.Key)
		fmt.Printf("producer: sqs body is envelope type=%s\n", envelope.Type)
	} else {
		fmt.Printf("producer: inline messageId=%s bytes=%d\n", messageID, len(payload))
	}

	return events.SQSMessage{
		MessageId: messageID,
		Body:      string(body),
	}, nil
}

func main() {
	ctx := context.Background()

	const (
		bucket = "example-bucket"
		prefix = "sqs/example-queue/messages/"
	)

	store := newMemoryStore()

	largeOKPayload, _ := json.Marshal(job{
		ID:   "large-ok",
		Data: strings.Repeat("x", sqslargepayload.SQSMaxMessageBytes+1024),
	})
	largeFailPayload, _ := json.Marshal(job{
		ID:   "large-fail",
		Data: strings.Repeat("y", sqslargepayload.SQSMaxMessageBytes+2048),
	})
	smallOKPayload, _ := json.Marshal(job{
		ID:   "small-ok",
		Data: "hello",
	})

	largeOK, err := produce(ctx, store, bucket, prefix, "msg-1", largeOKPayload)
	if err != nil {
		panic(err)
	}
	largeFail, err := produce(ctx, store, bucket, prefix, "msg-2", largeFailPayload)
	if err != nil {
		panic(err)
	}
	smallOK, err := produce(ctx, store, bucket, prefix, "msg-3", smallOKPayload)
	if err != nil {
		panic(err)
	}

	fmt.Printf("store: objects after produce: %v\n", store.keys())

	app := lift.New()

	processor := sqslargepayload.BatchProcessor{
		Store:   store,
		Options: sqslargepayload.DefaultOptions(),
		Hooks: sqslargepayload.BatchHooks{
			OnFailure: func(_ context.Context, failure sqslargepayload.Failure) {
				fmt.Printf("consumer: failure kind=%s messageId=%s err=%v\n", failure.Kind, failure.Message.Record.MessageId, failure.Err)
			},
		},
	}

	if err := app.SQS("*", func(ctx *lift.Context) (any, error) {
		records, err := ctx.SQSRecords()
		if err != nil {
			return events.SQSEventResponse{}, err
		}

		return processor.Process(ctx, records, func(_ context.Context, msg sqslargepayload.Message) error {
			var j job
			if err := json.Unmarshal(msg.Payload, &j); err != nil {
				return err
			}

			fmt.Printf(
				"consumer: handle id=%s messageId=%s offloaded=%t payloadBytes=%d\n",
				j.ID,
				msg.Record.MessageId,
				msg.Envelope != nil,
				len(msg.Payload),
			)

			if j.ID == "large-fail" {
				return errors.New("simulated handler error")
			}
			return nil
		})
	}); err != nil {
		panic(err)
	}

	event := map[string]any{
		"Records": []any{
			sqsRecord(largeOK, "rh-1"),
			sqsRecord(largeFail, "rh-2"),
			sqsRecord(smallOK, "rh-3"),
		},
	}

	result, err := app.HandleRequest(ctx, event)
	if err != nil {
		panic(err)
	}

	resp, ok := result.(events.SQSEventResponse)
	if !ok {
		panic(fmt.Errorf("expected events.SQSEventResponse, got %T", result))
	}

	fmt.Printf("consumer: batch item failures: %+v\n", resp.BatchItemFailures)
	fmt.Printf("store: objects after consume: %v\n", store.keys())
}

func sqsRecord(msg events.SQSMessage, receiptHandle string) map[string]any {
	return map[string]any{
		"messageId":      msg.MessageId,
		"body":           msg.Body,
		"receiptHandle":  receiptHandle,
		"eventSource":    "aws:sqs",
		"eventSourceARN": "arn:aws:sqs:us-east-1:123456789012:example-queue",
		"awsRegion":      "us-east-1",
		"attributes": map[string]any{
			"SentTimestamp": "0",
		},
	}
}
