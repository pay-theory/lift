package sqslargepayload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestBatchProcessor_MixedBatchDeletesOnlySuccessfulLargePayloadObjects(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()

	okRef := Ref{Bucket: "bucket", Key: "sqs/queue/messages/ok"}
	okPayload := []byte("ok-payload")
	requireStorePut(t, store, okRef, okPayload)

	okEnv := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: okRef.Bucket,
		Key:    okRef.Key,
		SHA256: sha256Hex(okPayload),
		Bytes:  int64(len(okPayload)),
	}
	okBody := mustMarshalEnvelope(t, okEnv)

	failRef := Ref{Bucket: "bucket", Key: "sqs/queue/messages/fail"}
	failPayload := []byte("fail-payload")
	requireStorePut(t, store, failRef, failPayload)

	failEnv := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: failRef.Bucket,
		Key:    failRef.Key,
		SHA256: sha256Hex(failPayload),
		Bytes:  int64(len(failPayload)),
	}
	failBody := mustMarshalEnvelope(t, failEnv)

	records := []events.SQSMessage{
		{MessageId: "inline-1", Body: "inline"},
		{MessageId: "offload-ok", Body: string(okBody)},
		{MessageId: "offload-fail", Body: string(failBody)},
	}

	processor := BatchProcessor{Store: store}

	resp, err := processor.Process(ctx, records, func(_ context.Context, msg Message) error {
		switch msg.Record.MessageId {
		case "inline-1":
			if string(msg.Payload) != "inline" {
				t.Fatalf("expected inline payload, got %q", msg.Payload)
			}
			if msg.Envelope != nil {
				t.Fatalf("expected no envelope for inline message")
			}
			return nil
		case "offload-ok":
			if string(msg.Payload) != string(okPayload) {
				t.Fatalf("expected hydrated ok payload, got %q", msg.Payload)
			}
			if msg.Envelope == nil {
				t.Fatalf("expected envelope for offloaded message")
			}
			return nil
		case "offload-fail":
			if string(msg.Payload) != string(failPayload) {
				t.Fatalf("expected hydrated fail payload, got %q", msg.Payload)
			}
			return errors.New("boom")
		default:
			return nil
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.BatchItemFailures) != 1 || resp.BatchItemFailures[0].ItemIdentifier != "offload-fail" {
		t.Fatalf("expected one failed message offload-fail, got %#v", resp.BatchItemFailures)
	}

	if store.deletes != 1 {
		t.Fatalf("expected exactly 1 delete (successful offloaded message), got %d", store.deletes)
	}

	if _, err := store.Get(ctx, okRef); err == nil {
		t.Fatalf("expected ok payload object to be deleted")
	}
	if got, err := store.Get(ctx, failRef); err != nil || string(got) != string(failPayload) {
		t.Fatalf("expected failed payload object to remain; got=%q err=%v", got, err)
	}
}

func TestBatchProcessor_DeleteFailureDoesNotFailMessageByDefault(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()

	ref := Ref{Bucket: "bucket", Key: "sqs/queue/messages/delete-fail"}
	payload := []byte("payload")
	requireStorePut(t, store, ref, payload)

	env := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: ref.Bucket,
		Key:    ref.Key,
		SHA256: sha256Hex(payload),
		Bytes:  int64(len(payload)),
	}
	body := mustMarshalEnvelope(t, env)

	deleteErr := errors.New("delete failed")
	failStore := deleteFailStore{
		memoryStore: store,
		failRef:     ref,
		deleteErr:   deleteErr,
	}

	seenDeleteFailure := 0
	processor := BatchProcessor{
		Store: failStore,
		Hooks: BatchHooks{
			OnDeleteFailure: func(_ context.Context, failure Failure) {
				if !errors.Is(failure.Err, deleteErr) {
					t.Fatalf("expected delete error to propagate")
				}
				seenDeleteFailure++
			},
		},
	}

	resp, err := processor.Process(ctx, []events.SQSMessage{
		{MessageId: "msg-1", Body: string(body)},
	}, func(_ context.Context, _ Message) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected no failures, got %#v", resp.BatchItemFailures)
	}
	if seenDeleteFailure != 1 {
		t.Fatalf("expected delete failure hook to be called once, got %d", seenDeleteFailure)
	}
}

func TestBatchProcessor_DeleteFailureFailsMessageWhenStrict(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()

	ref := Ref{Bucket: "bucket", Key: "sqs/queue/messages/delete-fail"}
	payload := []byte("payload")
	requireStorePut(t, store, ref, payload)

	env := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: ref.Bucket,
		Key:    ref.Key,
		SHA256: sha256Hex(payload),
		Bytes:  int64(len(payload)),
	}
	body := mustMarshalEnvelope(t, env)

	failStore := deleteFailStore{
		memoryStore: store,
		failRef:     ref,
		deleteErr:   errors.New("delete failed"),
	}

	processor := BatchProcessor{
		Store:             failStore,
		FailOnDeleteError: true,
	}

	resp, err := processor.Process(ctx, []events.SQSMessage{
		{MessageId: "msg-1", Body: string(body)},
	}, func(_ context.Context, _ Message) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.BatchItemFailures) != 1 || resp.BatchItemFailures[0].ItemIdentifier != "msg-1" {
		t.Fatalf("expected msg-1 to fail due to delete error, got %#v", resp.BatchItemFailures)
	}
}

func TestBatchProcessor_HydrateFailureMarksMessageFailed(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()

	missingRef := Ref{Bucket: "bucket", Key: "sqs/queue/messages/missing"}
	payload := []byte("payload")
	env := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: missingRef.Bucket,
		Key:    missingRef.Key,
		SHA256: sha256Hex(payload),
		Bytes:  int64(len(payload)),
	}
	body := mustMarshalEnvelope(t, env)

	seenHandler := 0
	processor := BatchProcessor{Store: store}

	resp, err := processor.Process(ctx, []events.SQSMessage{
		{MessageId: "msg-1", Body: string(body)},
	}, func(_ context.Context, _ Message) error {
		seenHandler++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if seenHandler != 0 {
		t.Fatalf("expected handler to not be called on hydrate failure")
	}
	if len(resp.BatchItemFailures) != 1 || resp.BatchItemFailures[0].ItemIdentifier != "msg-1" {
		t.Fatalf("expected hydrate failure to mark msg-1 failed, got %#v", resp.BatchItemFailures)
	}
}

type deleteFailStore struct {
	*memoryStore
	failRef   Ref
	deleteErr error
}

func (d deleteFailStore) Delete(_ context.Context, ref Ref) error {
	if ref == d.failRef {
		return d.deleteErr
	}
	return d.memoryStore.Delete(context.Background(), ref)
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func mustMarshalEnvelope(t *testing.T, env Envelope) []byte {
	t.Helper()
	encoded, err := MarshalEnvelope(env)
	if err != nil {
		t.Fatalf("failed to marshal envelope: %v", err)
	}
	return encoded
}

func requireStorePut(t *testing.T, store ObjectStore, ref Ref, payload []byte) {
	t.Helper()
	if err := store.Put(context.Background(), ref, payload); err != nil {
		t.Fatalf("failed to seed store: %v", err)
	}
}
