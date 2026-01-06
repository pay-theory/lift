package sqslargepayload

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestParseSQSExtendedClientPointer_ParsesKnownFormat(t *testing.T) {
	body := mustMarshalJSON(t, []any{
		SQSExtendedClientPointerClass,
		map[string]string{"s3BucketName": "bucket", "s3Key": "key"},
	})

	pointer, ok, err := ParseSQSExtendedClientPointer(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if pointer.Class != SQSExtendedClientPointerClass {
		t.Fatalf("expected class %q, got %q", SQSExtendedClientPointerClass, pointer.Class)
	}
	if pointer.Bucket != "bucket" || pointer.Key != "key" {
		t.Fatalf("unexpected pointer fields: %#v", pointer)
	}
}

func TestParseSQSExtendedClientPointer_ReturnsFalseForNonPointerBodies(t *testing.T) {
	cases := [][]byte{
		[]byte("not json"),
		mustMarshalJSON(t, map[string]any{"type": "lift:something"}),
		mustMarshalJSON(t, []any{"not-a-pointer-class", map[string]string{"s3BucketName": "b", "s3Key": "k"}}),
	}

	for i, body := range cases {
		_, ok, err := ParseSQSExtendedClientPointer(body)
		if err != nil {
			t.Fatalf("case %d: unexpected error: %v", i, err)
		}
		if ok {
			t.Fatalf("case %d: expected ok=false", i)
		}
	}
}

func TestParseSQSExtendedClientPointer_ErrorsOnMalformedPointer(t *testing.T) {
	body := mustMarshalJSON(t, []any{
		SQSExtendedClientPointerClass,
		map[string]string{"s3BucketName": "bucket"},
	})

	_, ok, err := ParseSQSExtendedClientPointer(body)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestBatchProcessor_HydratesAndDeletesSQSExtendedClientPointers(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()

	ref := Ref{Bucket: "bucket", Key: "key"}
	payload := []byte("payload")
	requireStorePut(t, store, ref, payload)

	pointerBody := mustMarshalJSON(t, []any{
		SQSExtendedClientPointerClass,
		map[string]string{"s3BucketName": ref.Bucket, "s3Key": ref.Key},
	})

	processor := BatchProcessor{Store: store}

	resp, err := processor.Process(ctx, []events.SQSMessage{
		{
			MessageId: "msg-1",
			Body:      string(pointerBody),
			MessageAttributes: map[string]events.SQSMessageAttribute{
				SQSExtendedClientReservedAttributeName: {DataType: "Number", StringValue: strPtr("7")},
			},
		},
	}, func(_ context.Context, msg Message) error {
		if msg.Envelope != nil {
			t.Fatalf("expected Envelope to be nil for extended-client pointer messages")
		}
		if string(msg.Payload) != string(payload) {
			t.Fatalf("expected hydrated payload %q, got %q", payload, msg.Payload)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected no failures, got %#v", resp.BatchItemFailures)
	}
	if store.deletes != 1 {
		t.Fatalf("expected exactly 1 delete, got %d", store.deletes)
	}
	if _, err := store.Get(ctx, ref); err == nil {
		t.Fatalf("expected payload object to be deleted")
	}
}

func TestBatchProcessor_DoesNotDeleteFailedSQSExtendedClientPointers(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()

	ref := Ref{Bucket: "bucket", Key: "key"}
	payload := []byte("payload")
	requireStorePut(t, store, ref, payload)

	pointerBody := mustMarshalJSON(t, []any{
		SQSExtendedClientPointerClass,
		map[string]string{"s3BucketName": ref.Bucket, "s3Key": ref.Key},
	})

	processor := BatchProcessor{Store: store}

	resp, err := processor.Process(ctx, []events.SQSMessage{
		{MessageId: "msg-1", Body: string(pointerBody)},
	}, func(_ context.Context, _ Message) error {
		return errors.New("boom")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.BatchItemFailures) != 1 || resp.BatchItemFailures[0].ItemIdentifier != "msg-1" {
		t.Fatalf("expected msg-1 to fail, got %#v", resp.BatchItemFailures)
	}
	if store.deletes != 0 {
		t.Fatalf("expected no deletes on handler failure, got %d", store.deletes)
	}
	if got, err := store.Get(ctx, ref); err != nil || string(got) != string(payload) {
		t.Fatalf("expected payload object to remain; got=%q err=%v", got, err)
	}
}

func mustMarshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal json: %v", err)
	}
	return data
}

func strPtr(s string) *string {
	return &s
}
