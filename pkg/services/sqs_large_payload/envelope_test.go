package sqslargepayload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func TestShouldOffload(t *testing.T) {
	payload := make([]byte, SQSMaxMessageBytes)
	if ShouldOffload(payload) {
		t.Fatalf("expected payload at limit (%d) to not offload", SQSMaxMessageBytes)
	}

	payload = make([]byte, SQSMaxMessageBytes+1)
	if !ShouldOffload(payload) {
		t.Fatalf("expected payload over limit (%d) to offload", SQSMaxMessageBytes)
	}
}

func TestParseEnvelope_Validation(t *testing.T) {
	_, err := ParseEnvelope([]byte(`{"bucket":"b","key":"k"}`))
	if !errors.Is(err, ErrEnvelopeMissingType) {
		t.Fatalf("expected ErrEnvelopeMissingType, got %v", err)
	}

	_, err = ParseEnvelope([]byte(`{"type":"unknown","bucket":"b","key":"k"}`))
	if !errors.Is(err, ErrEnvelopeUnknownType) {
		t.Fatalf("expected ErrEnvelopeUnknownType, got %v", err)
	}

	_, err = ParseEnvelope([]byte(`{"type":"` + EnvelopeTypeV1 + `","key":"k"}`))
	if !errors.Is(err, ErrEnvelopeMissingBucket) {
		t.Fatalf("expected ErrEnvelopeMissingBucket, got %v", err)
	}

	_, err = ParseEnvelope([]byte(`{"type":"` + EnvelopeTypeV1 + `","bucket":"b"}`))
	if !errors.Is(err, ErrEnvelopeMissingKey) {
		t.Fatalf("expected ErrEnvelopeMissingKey, got %v", err)
	}
}

func TestOffloadIfNeeded_NoOffload(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	ref := Ref{Bucket: "bucket", Key: "key"}
	payload := []byte("small")

	body, envelope, err := OffloadIfNeeded(ctx, store, ref, payload, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envelope != nil {
		t.Fatalf("expected nil envelope when not offloading, got %#v", envelope)
	}
	if string(body) != string(payload) {
		t.Fatalf("expected original payload to be returned")
	}
	if store.puts != 0 {
		t.Fatalf("expected no store puts, got %d", store.puts)
	}
}

func TestOffloadIfNeeded_OffloadsAndReturnsEnvelope(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	ref := Ref{Bucket: "bucket", Key: "sqs/queue/messages/abc"}
	payload := make([]byte, SQSMaxMessageBytes+1)
	payload[0] = 0x7f

	body, envelope, err := OffloadIfNeeded(ctx, store, ref, payload, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envelope == nil {
		t.Fatalf("expected envelope when offloading")
	}
	if store.puts != 1 {
		t.Fatalf("expected exactly 1 store put, got %d", store.puts)
	}

	got, err := ParseEnvelope(body)
	if err != nil {
		t.Fatalf("expected returned body to be an envelope, got error: %v", err)
	}
	if got.Bucket != ref.Bucket || got.Key != ref.Key {
		t.Fatalf("expected envelope ref to match, got bucket=%q key=%q", got.Bucket, got.Key)
	}
	if got.Bytes != int64(len(payload)) {
		t.Fatalf("expected envelope bytes=%d got %d", len(payload), got.Bytes)
	}
	if got.SHA256 == "" {
		t.Fatalf("expected sha256 to be set by default")
	}

	stored, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if len(stored) != len(payload) || stored[0] != payload[0] {
		t.Fatalf("expected stored payload to match original")
	}
}

func TestOffloadIfNeeded_DisableSHA256(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	ref := Ref{Bucket: "bucket", Key: "sqs/queue/messages/abc"}
	payload := make([]byte, SQSMaxMessageBytes+1)

	verify := false
	body, _, err := OffloadIfNeeded(ctx, store, ref, payload, Options{VerifySHA256: &verify})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ParseEnvelope(body)
	if err != nil {
		t.Fatalf("expected returned body to be an envelope, got error: %v", err)
	}
	if got.SHA256 != "" {
		t.Fatalf("expected sha256 to be omitted when disabled, got %q", got.SHA256)
	}
}

func TestHydrate_VerifyIntegrity_DefaultOn(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	ref := Ref{Bucket: "bucket", Key: "key"}
	payload := []byte("payload")
	if err := store.Put(ctx, ref, payload); err != nil {
		t.Fatalf("unexpected put error: %v", err)
	}

	sum := sha256.Sum256(payload)
	envelope := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: ref.Bucket,
		Key:    ref.Key,
		SHA256: hex.EncodeToString(sum[:]),
		Bytes:  int64(len(payload)),
	}

	out, err := Hydrate(ctx, store, envelope, Options{})
	if err != nil {
		t.Fatalf("unexpected hydrate error: %v", err)
	}
	if string(out) != string(payload) {
		t.Fatalf("expected hydrated payload to match")
	}
}

func TestHydrate_IntegrityMismatch(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	ref := Ref{Bucket: "bucket", Key: "key"}

	right := []byte("right")
	wrong := []byte("wrong")
	if err := store.Put(ctx, ref, wrong); err != nil {
		t.Fatalf("unexpected put error: %v", err)
	}

	sum := sha256.Sum256(right)
	envelope := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: ref.Bucket,
		Key:    ref.Key,
		SHA256: hex.EncodeToString(sum[:]),
		Bytes:  int64(len(right)),
	}

	_, err := Hydrate(ctx, store, envelope, Options{})
	if !errors.Is(err, ErrIntegrityMismatch) {
		t.Fatalf("expected ErrIntegrityMismatch, got %v", err)
	}
}

func TestDelete_RemovesObject(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	ref := Ref{Bucket: "bucket", Key: "key"}
	if err := store.Put(ctx, ref, []byte("payload")); err != nil {
		t.Fatalf("unexpected put error: %v", err)
	}

	envelope := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: ref.Bucket,
		Key:    ref.Key,
	}

	if err := Delete(ctx, store, envelope); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if store.deletes != 1 {
		t.Fatalf("expected exactly 1 delete, got %d", store.deletes)
	}
	if _, err := store.Get(ctx, ref); err == nil {
		t.Fatalf("expected object to be deleted")
	}
}

type memoryStore struct {
	objects map[Ref][]byte
	puts    int
	gets    int
	deletes int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{objects: make(map[Ref][]byte)}
}

func (m *memoryStore) Put(_ context.Context, ref Ref, payload []byte) error {
	m.puts++
	copied := make([]byte, len(payload))
	copy(copied, payload)
	m.objects[ref] = copied
	return nil
}

func (m *memoryStore) Get(_ context.Context, ref Ref) ([]byte, error) {
	m.gets++
	payload, ok := m.objects[ref]
	if !ok {
		return nil, errors.New("not found")
	}
	copied := make([]byte, len(payload))
	copy(copied, payload)
	return copied, nil
}

func (m *memoryStore) Delete(_ context.Context, ref Ref) error {
	m.deletes++
	delete(m.objects, ref)
	return nil
}
