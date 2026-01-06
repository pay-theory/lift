package sqslargepayload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

// FailureKind categorizes failures encountered while processing SQS messages.
type FailureKind string

const (
	FailureHydrate FailureKind = "hydrate"
	FailureHandler FailureKind = "handler"
	FailureDelete  FailureKind = "delete"
)

// Failure describes an error encountered while processing an individual SQS message.
type Failure struct {
	Envelope *Envelope
	Err      error
	Kind     FailureKind
	Message  Message
}

func (f Failure) Error() string {
	messageID := f.Message.Record.MessageId
	if messageID == "" {
		messageID = "<missing>"
	}
	return fmt.Sprintf("sqs large payload %s failure for message %s: %v", f.Kind, messageID, f.Err)
}

func (f Failure) Unwrap() error {
	return f.Err
}

// BatchHooks provides callback hooks for observing failures during batch processing.
type BatchHooks struct {
	OnFailure        func(ctx context.Context, failure Failure)
	OnHydrateFailure func(ctx context.Context, failure Failure)
	OnHandlerFailure func(ctx context.Context, failure Failure)
	OnDeleteFailure  func(ctx context.Context, failure Failure)
}

// Message is the per-record context passed to the user handler.
type Message struct {
	Record   events.SQSMessage
	Envelope *Envelope
	RawBody  []byte
	Payload  []byte
}

// MessageHandler processes an individual message (inline or hydrated).
// Returning an error marks the message as failed in the batch response.
type MessageHandler func(ctx context.Context, message Message) error

// BatchProcessor processes SQS event batches containing a mix of inline and S3-offloaded payloads.
//
// For each record:
//   - If the body is a Lift large-payload envelope, hydrate from the object store.
//   - Call the per-message handler with the hydrated payload bytes.
//   - On handler success, delete the S3 object (best-effort by default).
//   - Return an events.SQSEventResponse containing only the failed message IDs.
type BatchProcessor struct {
	Store             ObjectStore
	Hooks             BatchHooks
	Options           Options
	FailOnDeleteError bool
}

// Process handles a batch of SQS messages and returns an SQS batch response suitable for
// ReportBatchItemFailures.
func (p BatchProcessor) Process(ctx context.Context, records []events.SQSMessage, handler MessageHandler) (events.SQSEventResponse, error) {
	if handler == nil {
		return events.SQSEventResponse{}, errors.New("sqs large payload batch handler is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	failures := make([]events.SQSBatchItemFailure, 0)

	for _, record := range records {
		messageID := record.MessageId
		if messageID == "" {
			return events.SQSEventResponse{}, errors.New("sqs message missing messageId")
		}

		rawBody := []byte(record.Body)

		message := Message{
			Record:  record,
			RawBody: rawBody,
		}

		pointer, err := parseOffloadPointer(rawBody)
		if err != nil {
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: messageID})
			p.emitFailure(ctx, Failure{Kind: FailureHydrate, Message: message, Envelope: pointer.envelope, Err: err})
			continue
		}

		if pointer.kind == offloadPointerNone {
			message.Payload = rawBody
		} else {
			if p.Store == nil {
				failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: messageID})
				p.emitFailure(ctx, Failure{
					Kind:     FailureHydrate,
					Message:  message,
					Envelope: pointer.envelope,
					Err:      errors.New("sqs large payload store is nil"),
				})
				continue
			}

			payload, err := p.hydratePointer(ctx, pointer)
			if err != nil {
				failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: messageID})
				p.emitFailure(ctx, Failure{Kind: FailureHydrate, Message: message, Envelope: pointer.envelope, Err: err})
				continue
			}

			message.Payload = payload
			message.Envelope = pointer.envelope
		}

		if err := handler(ctx, message); err != nil {
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: messageID})
			p.emitFailure(ctx, Failure{Kind: FailureHandler, Message: message, Envelope: pointer.envelope, Err: err})
			continue
		}

		if p.Store == nil {
			continue
		}

		if err := p.deletePointer(ctx, pointer, message); err != nil && p.FailOnDeleteError {
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: messageID})
		}
	}

	return events.SQSEventResponse{BatchItemFailures: failures}, nil
}

func (p BatchProcessor) hydratePointer(ctx context.Context, pointer offloadPointer) ([]byte, error) {
	if p.Store == nil {
		return nil, errors.New("sqs large payload store is nil")
	}

	switch pointer.kind {
	case offloadPointerLiftEnvelope:
		if pointer.envelope == nil {
			return nil, errors.New("sqs large payload envelope is nil")
		}
		return Hydrate(ctx, p.Store, *pointer.envelope, p.Options)
	case offloadPointerSQSExtendedClient:
		return p.Store.Get(ctx, pointer.extended.Ref())
	default:
		return nil, errors.New("sqs large payload pointer is not offloaded")
	}
}

func (p BatchProcessor) deletePointer(ctx context.Context, pointer offloadPointer, message Message) error {
	if p.Store == nil {
		return nil
	}

	switch pointer.kind {
	case offloadPointerLiftEnvelope:
		if pointer.envelope == nil {
			return nil
		}
		if err := Delete(ctx, p.Store, *pointer.envelope); err != nil {
			p.emitFailure(ctx, Failure{Kind: FailureDelete, Message: message, Envelope: pointer.envelope, Err: err})
			return err
		}
	case offloadPointerSQSExtendedClient:
		if err := p.Store.Delete(ctx, pointer.extended.Ref()); err != nil {
			p.emitFailure(ctx, Failure{Kind: FailureDelete, Message: message, Envelope: nil, Err: err})
			return err
		}
	}

	return nil
}

func (p BatchProcessor) emitFailure(ctx context.Context, failure Failure) {
	switch failure.Kind {
	case FailureHydrate:
		if p.Hooks.OnHydrateFailure != nil {
			p.Hooks.OnHydrateFailure(ctx, failure)
		}
	case FailureHandler:
		if p.Hooks.OnHandlerFailure != nil {
			p.Hooks.OnHandlerFailure(ctx, failure)
		}
	case FailureDelete:
		if p.Hooks.OnDeleteFailure != nil {
			p.Hooks.OnDeleteFailure(ctx, failure)
		}
	}

	if p.Hooks.OnFailure != nil {
		p.Hooks.OnFailure(ctx, failure)
	}
}

func parseLiftEnvelope(body []byte) (*Envelope, bool, error) {
	var probe struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, false, nil
	}
	if probe.Type != EnvelopeTypeV1 {
		return nil, false, nil
	}

	envelope, err := ParseEnvelope(body)
	if err != nil {
		return nil, true, err
	}
	return &envelope, true, nil
}

type offloadPointerKind uint8

const (
	offloadPointerNone offloadPointerKind = iota
	offloadPointerLiftEnvelope
	offloadPointerSQSExtendedClient
)

type offloadPointer struct {
	envelope *Envelope
	extended SQSExtendedClientPointer
	kind     offloadPointerKind
}

func parseOffloadPointer(body []byte) (offloadPointer, error) {
	envelope, isEnvelope, err := parseLiftEnvelope(body)
	if isEnvelope {
		return offloadPointer{kind: offloadPointerLiftEnvelope, envelope: envelope}, err
	}

	pointer, ok, err := ParseSQSExtendedClientPointer(body)
	if ok {
		return offloadPointer{kind: offloadPointerSQSExtendedClient, extended: pointer}, err
	}

	return offloadPointer{kind: offloadPointerNone}, nil
}
