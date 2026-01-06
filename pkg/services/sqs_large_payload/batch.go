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
	Kind     FailureKind
	Message  Message
	Envelope *Envelope
	Err      error
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
	RawBody  []byte
	Payload  []byte
	Envelope *Envelope
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
	Options           Options
	FailOnDeleteError bool
	Hooks             BatchHooks
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
		if record.MessageId == "" {
			return events.SQSEventResponse{}, errors.New("sqs message missing messageId")
		}

		rawBody := []byte(record.Body)

		envelope, isEnvelope, err := parseLiftEnvelope(rawBody)
		message := Message{
			Record:  record,
			RawBody: rawBody,
		}

		if isEnvelope {
			message.Envelope = envelope
			if err != nil {
				failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
				p.emitFailure(ctx, Failure{Kind: FailureHydrate, Message: message, Envelope: envelope, Err: err})
				continue
			}

			if p.Store == nil {
				failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
				p.emitFailure(ctx, Failure{
					Kind:     FailureHydrate,
					Message:  message,
					Envelope: envelope,
					Err:      errors.New("sqs large payload store is nil"),
				})
				continue
			}

			payload, err := Hydrate(ctx, p.Store, *envelope, p.Options)
			if err != nil {
				failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
				p.emitFailure(ctx, Failure{Kind: FailureHydrate, Message: message, Envelope: envelope, Err: err})
				continue
			}
			message.Payload = payload
		} else {
			message.Payload = rawBody
		}

		if err := handler(ctx, message); err != nil {
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
			p.emitFailure(ctx, Failure{Kind: FailureHandler, Message: message, Envelope: envelope, Err: err})
			continue
		}

		if envelope == nil || p.Store == nil {
			continue
		}

		if err := Delete(ctx, p.Store, *envelope); err != nil {
			p.emitFailure(ctx, Failure{Kind: FailureDelete, Message: message, Envelope: envelope, Err: err})
			if p.FailOnDeleteError {
				failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
			}
		}
	}

	return events.SQSEventResponse{BatchItemFailures: failures}, nil
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
