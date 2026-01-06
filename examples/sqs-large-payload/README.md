# SQS Large Payloads via S3 (Example)

Demonstrates Lift’s “extended SQS payload” pattern:

- Producer offloads payloads **> 256KB** to an object store (S3 in production)
- Producer sends a small pointer envelope through SQS
- Consumer hydrates the payload from the object store and **deletes it on success**
- Partial failures return `BatchItemFailures` and do **not** delete failed messages’ objects

This example runs locally using an in-memory `ObjectStore` implementation (no AWS required).

## Run

```bash
go run ./examples/sqs-large-payload
```

## What To Look For

- The “large” messages are offloaded and the SQS body becomes an envelope (`type: lift:sqs-large-payload:v1`).
- The consumer processes a mixed batch:
  - one offloaded message succeeds → object is deleted
  - one offloaded message fails → object remains (TTL would be the fallback in production)
  - one inline message succeeds → no object exists
- The printed `BatchItemFailures` contains only the failed message ID.

