# SQS Large Payload via S3 (Extend `SQSProcessorProps`) Roadmap

Status: draft (for alignment)

<!-- AI Training Signal: This document is a product/engineering roadmap for adding first-class "SQS extended payloads" support to Lift. -->

## Problem Statement

AWS SQS message bodies have a hard limit of **256KB**. Some Lift workloads need durable queue semantics (SQS retries/DLQ, batching, concurrency control) while carrying payloads larger than 256KB.

We want Lift to support the common “extended payload” pattern:

- **Producer** stores the full payload in S3 when it would exceed SQS limits.
- Producer publishes an SQS message containing a **small pointer envelope** to the S3 object.
- **Consumer** hydrates the message from S3 and **deletes the S3 object on successful processing** (with lifecycle TTL as a safety net).

This mirrors an existing production pattern used in other systems (S3 overflow for payload limits), but adapted for SQS.

## Goals

1. **CDK support (one-line opt-in)**: Extend `constructs.SQSProcessorProps` with a payload-bucket option that provisions and wires an S3 bucket for large payload storage with secure defaults.
2. **Runtime helpers (Go)**: Provide Lift helper code to:
   - offload oversized payloads to S3 (no gzip)
   - generate and parse a pointer envelope
   - hydrate messages from S3
   - delete S3 objects on success
3. **Correct SQS failure semantics**: Support **partial batch failure** (`ReportBatchItemFailures`) so successful messages do not retry, enabling safe per-message S3 deletion.
4. **Minimal configuration / derived naming**:
   - Do not require a large set of env vars to operate.
   - Prefer deterministic/derived resource names in CDK (using Lift naming conventions) when naming context is available.
   - Ensure the pointer envelope is **self-describing** (`bucket` + `key`) so consumers do not need bucket env vars.

## Non-Goals

- Compression (explicitly **no gzip**).
- Changing SQS limits or supporting “chunking” across multiple SQS messages.
- Cross-account payload buckets or complex multi-region replication.
- A full event-orchestration system (this is specifically about SQS payload size limits).

## Proposed Design (High-Level)

### Offload rule

- If the serialized message body is **> 256KB**, store it in S3 and send an SQS pointer envelope instead.
- Threshold is fixed; no runtime tuning required.

### Pointer envelope (SQS message body)

SQS message body becomes a small JSON object:

```json
{
  "type": "lift:sqs-large-payload:v1",
  "bucket": "myapp-live-sqs-large-payload",
  "key": "sqs/orders-queue-live/messages/01J.../payload.json",
  "sha256": "…optional…",
  "bytes": 123456
}
```

Notes:
- Include `bucket` and `key` so consumers can hydrate/delete without environment configuration.
- Include `type` for unambiguous detection.
- `sha256` is optional but recommended for integrity checks.

### Deletion policy

- **Delete from S3 on success** (per message).
- Configure S3 lifecycle TTL as a fallback for leak cleanup (e.g., 7–14 days; default should align with typical SQS retention/workflow duration).

### Naming strategy (CDK)

- When stable naming inputs exist, derive deterministic bucket names using Lift’s `pkg/naming` conventions.
- When not available, allow CDK to auto-name resources; the pointer envelope will still include the resolved bucket name at publish time.

## Milestones

### Milestone 1 — Spec & Public API (CDK + Runtime)

**Deliverables**
- Define `SQSLargePayloadProps` and add it to `constructs.SQSProcessorProps`:
  - enable/disable
  - `ExistingBucket` or `BucketProps` (create-if-nil)
  - lifecycle TTL configuration
  - optional prefix override (default derived from queue name)
- Define pointer envelope schema and Go types.
- Define the runtime helper surface (package name, function signatures, errors).

**Acceptance Criteria**
- A single document section (this roadmap + API reference update later) defines:
  - envelope JSON schema and versioning (`lift:sqs-large-payload:v1`)
  - offload threshold rule (`> 256KB`)
  - deletion policy (delete on success + TTL fallback)
- The `SQSProcessorProps` extension is backward-compatible (existing code compiles unchanged).

---

### Milestone 2 — CDK: Extend `SQSProcessor` with S3 Payload Bucket

**Deliverables**
- Implement bucket creation/attachment inside `NewSQSProcessor` when `LargePayload` is enabled:
  - Secure defaults: block public access, encryption, lifecycle expiration.
  - Prefix-scoped IAM: grant the processor Lambda `s3:GetObject` + `s3:DeleteObject` for the prefix.
- Expose on the construct:
  - `LargePayloadBucket` (bucket reference)
  - `LargePayloadPrefix` (string)
  - helper grants: `GrantLargePayloadPublish(grantee)` (PutObject) and `GrantLargePayloadConsume(grantee)` (Get/Delete)
- CDK tests covering synthesis outputs and IAM policies.

**Acceptance Criteria**
- Synthesized template contains `AWS::S3::Bucket` with a lifecycle expiration rule when `LargePayload` is enabled.
- Synthesized template shows the Lambda role has:
  - `s3:GetObject` and `s3:DeleteObject` permissions scoped to the bucket + prefix
- No new “tuning” env vars are required by the consumer for hydration (consumer can hydrate based solely on the pointer envelope).

---

### Milestone 3 — Runtime: S3 Offload + Hydration + Delete Helpers

**Deliverables**
- Add a Lift runtime helper package (proposed location: `pkg/services/sqs_large_payload`) implementing:
  - Encode/decode pointer envelopes
  - S3 `PutObject` for offload (no gzip)
  - S3 `GetObject` for hydration
  - S3 `DeleteObject` on success
- Add a small in-memory/mockable interface to unit test without AWS.
- Add unit tests for:
  - offload decision at `>256KB`
  - envelope parse/validation
  - hydrate replaces message body correctly
  - delete called only when appropriate

**Acceptance Criteria**
- Helpers can:
  - offload and return a pointer envelope
  - hydrate a pointer envelope into original bytes
  - delete the referenced object
- Unit tests cover threshold and envelope validation.
- No gzip/compression code paths exist.

---

### Milestone 4 — Lift Core: First-Class SQS Batch Responses

**Deliverables**
- Update Lift request handling so SQS-triggered handlers can return `events.SQSEventResponse` (raw) when `ReportBatchItemFailures` is enabled.
- Add tests ensuring:
  - SQS returns raw batch response (not wrapped in API Gateway proxy response)
  - existing HTTP/APIGW behavior is unchanged

**Acceptance Criteria**
- A handler registered via `app.SQS(...)` can return `events.SQSEventResponse`, and the Lambda result is exactly the batch response object.
- Partial failures work end-to-end in tests (successful messages are not re-driven).

---

### Milestone 5 — “Batteries Included” SQS Large Payload Handler Helper

**Deliverables**
- Provide a helper (or middleware) that standardizes the pattern:
  - iterate records
  - hydrate per-message if pointer envelope
  - call a per-message user handler
  - build `BatchItemFailures` for failures only
  - delete S3 objects for successes only
- Include hooks for logging/metrics and a clear error taxonomy:
  - hydrate failure
  - handler failure
  - delete failure (non-fatal; TTL fallback)

**Acceptance Criteria**
- Helper supports mixed batches (some pointer messages, some inline messages).
- On mixed success/failure batches:
  - only successful messages’ S3 payloads are deleted
  - failed messages are returned in `BatchItemFailures`
- Delete failures do not incorrectly mark the message as failed (unless explicitly configured).

---

### Milestone 6 — Documentation + Examples

**Deliverables**
- New doc page describing the pattern and how to enable it via `SQSProcessorProps.LargePayload`.
- Add a working example demonstrating:
  - a producer that offloads large payloads to S3 and sends pointers to SQS
  - a consumer using Lift + the helper to hydrate and delete-on-success
- Update CDK/API references as needed.

**Acceptance Criteria**
- Example compiles and demonstrates:
  - payload `>256KB` offloads to S3
  - SQS receives pointer envelope
  - consumer hydrates payload and deletes the S3 object on success
  - partial failures do not delete failed messages’ payloads

## Risks & Mitigations

- **S3 object leaks** (delete fails): mitigate with lifecycle TTL + logging/metrics for delete failures.
- **Duplicate delivery** (SQS at-least-once): helper should be idempotency-friendly; deleting a missing object should be treated as success.
- **Security**: enforce least-privilege IAM (prefix-scoped), block public access, and avoid embedding sensitive data in the pointer envelope beyond bucket/key.

## Open Questions

- Default TTL duration: should it mirror queue retention (14 days), or a shorter TTL (e.g., 7 days)?
ANSWER: 7 days.
- Should the helper support optional integrity verification (`sha256`) by default or only when provided?
ANSWER: By default verify integrity, but allow disabling it.
- Should Lift add a small “publisher binding” helper on `SQSProcessor` to simplify wiring producers (grant + optional convenience env injection)?
ANSWER: Yes.
