package sqslargepayload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// EnvelopeTypeV1 is the pointer envelope version identifier written to SQS message bodies.
	EnvelopeTypeV1 = "lift:sqs-large-payload:v1"

	// SQSMaxMessageBytes is the hard SQS message size limit (body + attributes).
	// Lift's large payload support offloads when the serialized body exceeds this limit.
	SQSMaxMessageBytes = 256 * 1024
)

var (
	ErrEnvelopeMissingType   = errors.New("sqs large payload envelope missing type")
	ErrEnvelopeUnknownType   = errors.New("sqs large payload envelope has unknown type")
	ErrEnvelopeMissingBucket = errors.New("sqs large payload envelope missing bucket")
	ErrEnvelopeMissingKey    = errors.New("sqs large payload envelope missing key")
	ErrIntegrityMismatch     = errors.New("sqs large payload sha256 mismatch")
)

// Ref identifies an S3 object holding an offloaded payload.
type Ref struct {
	Bucket string
	Key    string
}

// Envelope is the JSON-encoded SQS message body used when payloads are offloaded to S3.
type Envelope struct {
	Type   string `json:"type"`
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
	SHA256 string `json:"sha256,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
}

// Ref returns the S3 object reference described by the envelope.
func (e Envelope) Ref() Ref {
	return Ref{Bucket: e.Bucket, Key: e.Key}
}

// Validate ensures the envelope is well-formed and uses a supported version.
func (e Envelope) Validate() error {
	if e.Type == "" {
		return ErrEnvelopeMissingType
	}
	if e.Type != EnvelopeTypeV1 {
		return fmt.Errorf("%w: %s", ErrEnvelopeUnknownType, e.Type)
	}
	if e.Bucket == "" {
		return ErrEnvelopeMissingBucket
	}
	if e.Key == "" {
		return ErrEnvelopeMissingKey
	}
	return nil
}

// Options configures offload/hydration behavior.
type Options struct {
	// VerifySHA256 enables integrity checks when the envelope includes a sha256 value.
	// Default: true.
	VerifySHA256 *bool
}

// DefaultOptions returns the recommended default options.
func DefaultOptions() Options {
	return Options{VerifySHA256: boolPtr(true)}
}

// ShouldOffload reports whether the payload should be stored in S3 instead of sent inline via SQS.
func ShouldOffload(payload []byte) bool {
	return len(payload) > SQSMaxMessageBytes
}

// MarshalEnvelope encodes the envelope as JSON.
func MarshalEnvelope(envelope Envelope) ([]byte, error) {
	if err := envelope.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(envelope)
}

// ParseEnvelope decodes and validates an envelope from JSON.
func ParseEnvelope(data []byte) (Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Envelope{}, err
	}
	if err := envelope.Validate(); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

// ObjectStore defines the minimum storage operations required for offloading and hydrating payloads.
// Implementations should provide S3-backed storage with least-privilege access.
type ObjectStore interface {
	Put(ctx context.Context, ref Ref, payload []byte) error
	Get(ctx context.Context, ref Ref) ([]byte, error)
	Delete(ctx context.Context, ref Ref) error
}

// OffloadIfNeeded stores payload in the provided object store when it exceeds SQS limits and
// returns the message body bytes that should be sent to SQS.
//
// When payload is stored in S3, this returns the marshaled Envelope as the new message body and
// a non-nil Envelope pointer. When the payload is small enough, this returns the original payload
// and a nil Envelope pointer.
func OffloadIfNeeded(ctx context.Context, store ObjectStore, ref Ref, payload []byte, opts Options) ([]byte, *Envelope, error) {
	if !ShouldOffload(payload) {
		return payload, nil, nil
	}

	verify := true
	if opts.VerifySHA256 != nil {
		verify = *opts.VerifySHA256
	}

	envelope := Envelope{
		Type:   EnvelopeTypeV1,
		Bucket: ref.Bucket,
		Key:    ref.Key,
		Bytes:  int64(len(payload)),
	}

	if verify {
		sum := sha256.Sum256(payload)
		envelope.SHA256 = hex.EncodeToString(sum[:])
	}

	if err := envelope.Validate(); err != nil {
		return nil, nil, err
	}
	if err := store.Put(ctx, ref, payload); err != nil {
		return nil, nil, err
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, nil, err
	}
	return encoded, &envelope, nil
}

// Hydrate resolves an envelope to its payload bytes, optionally verifying integrity.
func Hydrate(ctx context.Context, store ObjectStore, envelope Envelope, opts Options) ([]byte, error) {
	if err := envelope.Validate(); err != nil {
		return nil, err
	}

	verify := true
	if opts.VerifySHA256 != nil {
		verify = *opts.VerifySHA256
	}

	payload, err := store.Get(ctx, envelope.Ref())
	if err != nil {
		return nil, err
	}

	if verify && envelope.SHA256 != "" {
		sum := sha256.Sum256(payload)
		if hex.EncodeToString(sum[:]) != envelope.SHA256 {
			return nil, ErrIntegrityMismatch
		}
	}

	return payload, nil
}

// Delete deletes the S3 object referenced by the envelope.
func Delete(ctx context.Context, store ObjectStore, envelope Envelope) error {
	if err := envelope.Validate(); err != nil {
		return err
	}
	return store.Delete(ctx, envelope.Ref())
}

func boolPtr(v bool) *bool {
	return &v
}
