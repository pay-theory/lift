// Package sqslargepayload provides helpers for handling SQS payloads that exceed
// the 256KB SQS message size limit by storing the full payload in S3 and sending
// a small pointer envelope through SQS.
//
// Design summary:
//   - Offload when payload size is > 256KB (no compression).
//   - SQS message body becomes a JSON envelope containing S3 bucket + key.
//   - Consumers hydrate from S3 and delete the object on success (TTL is a fallback).
//
// Compatibility:
//   - The BatchProcessor also recognizes S3 pointer bodies produced by the Amazon SQS extended
//     client libraries (Java/Python), which encode pointers as a JSON array:
//     [<pointer-class>, {"s3BucketName": "...", "s3Key": "..."}].
package sqslargepayload
