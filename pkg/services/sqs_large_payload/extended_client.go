package sqslargepayload

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// SQSExtendedClientReservedAttributeName is the reserved message attribute name used by the
	// Amazon SQS extended client libraries for payload size metadata.
	SQSExtendedClientReservedAttributeName = "ExtendedPayloadSize"

	// SQSExtendedClientLegacyReservedAttributeName is the legacy reserved attribute name used by
	// older extended client implementations.
	SQSExtendedClientLegacyReservedAttributeName = "SQSLargePayloadSize"

	// SQSExtendedClientPointerClass is the pointer class name written into the message body by
	// newer SQS extended client implementations.
	SQSExtendedClientPointerClass = "software.amazon.payloadoffloading.PayloadS3Pointer"

	// SQSExtendedClientLegacyPointerClass is the pointer class name written into the message body by
	// legacy SQS extended client implementations.
	SQSExtendedClientLegacyPointerClass = "com.amazon.sqs.javamessaging.MessageS3Pointer"
)

var (
	ErrSQSExtendedClientPointerMissingBucket = errors.New("sqs extended client pointer missing s3BucketName")
	ErrSQSExtendedClientPointerMissingKey    = errors.New("sqs extended client pointer missing s3Key")
	ErrSQSExtendedClientPointerUnknownClass  = errors.New("sqs extended client pointer has unknown class")
)

// SQSExtendedClientPointer describes the S3 object pointer format used by the Amazon SQS
// extended client libraries (Java/Python).
//
// The SQS message body is a JSON array containing:
//   - a pointer class name (string)
//   - a dictionary containing s3BucketName + s3Key
type SQSExtendedClientPointer struct {
	Class  string
	Bucket string
	Key    string
}

// Ref returns the S3 object reference described by the pointer.
func (p SQSExtendedClientPointer) Ref() Ref {
	return Ref{Bucket: p.Bucket, Key: p.Key}
}

// Validate ensures the pointer is well-formed and uses a supported class name.
func (p SQSExtendedClientPointer) Validate() error {
	switch p.Class {
	case SQSExtendedClientPointerClass, SQSExtendedClientLegacyPointerClass:
	default:
		if p.Class == "" {
			return fmt.Errorf("%w: <missing>", ErrSQSExtendedClientPointerUnknownClass)
		}
		return fmt.Errorf("%w: %s", ErrSQSExtendedClientPointerUnknownClass, p.Class)
	}

	if p.Bucket == "" {
		return ErrSQSExtendedClientPointerMissingBucket
	}
	if p.Key == "" {
		return ErrSQSExtendedClientPointerMissingKey
	}
	return nil
}

// ParseSQSExtendedClientPointer detects and parses the pointer format used by the Amazon SQS
// extended client libraries.
//
// Returns (pointer, true, nil) when data matches the expected format.
// Returns (zero, false, nil) when data does not appear to be an extended-client pointer.
func ParseSQSExtendedClientPointer(data []byte) (SQSExtendedClientPointer, bool, error) {
	var parts []json.RawMessage
	if err := json.Unmarshal(data, &parts); err != nil {
		return SQSExtendedClientPointer{}, false, nil
	}
	if len(parts) != 2 {
		return SQSExtendedClientPointer{}, false, nil
	}

	var class string
	if err := json.Unmarshal(parts[0], &class); err != nil {
		return SQSExtendedClientPointer{}, false, nil
	}
	if class != SQSExtendedClientPointerClass && class != SQSExtendedClientLegacyPointerClass {
		return SQSExtendedClientPointer{}, false, nil
	}

	var details struct {
		Bucket string `json:"s3BucketName"`
		Key    string `json:"s3Key"`
	}
	if err := json.Unmarshal(parts[1], &details); err != nil {
		return SQSExtendedClientPointer{}, true, err
	}

	pointer := SQSExtendedClientPointer{
		Class:  class,
		Bucket: details.Bucket,
		Key:    details.Key,
	}
	if err := pointer.Validate(); err != nil {
		return SQSExtendedClientPointer{}, true, err
	}

	return pointer, true, nil
}

