# Memory Pointer Spec

CAP keeps payloads off the bus by referencing external memory through opaque pointers.

## Pointer Types
- `context_ptr`: location of input payload written by the gateway or client.
- `result_ptr`: location of output payload written by the worker or orchestrator.
- `redacted_context_ptr`: optional sanitized input produced by the Safety Kernel on deny/throttle.

## Format
- Pointers are opaque URIs: examples include `redis://ctx/<job_id>`, `s3://bucket/key`, `file:///var/cap/ctx/job-id`, `https://object-store/...`.
- Consumers MUST treat pointers as opaque; dereferencing is implementation-specific.
- Pointers SHOULD be stable and immutable for the lifetime of the job.

## Size and Encoding
- Inputs/outputs SHOULD be stored as UTF-8 text or binary blobs; CAP does not mandate schema.
- For structured data, JSON or MsgPack are recommended for portability.
- Large artifacts (models, archives) SHOULD be chunked and referenced by manifest to avoid enormous single objects.

## TTL and Retention
- Gateways SHOULD set TTL on `context_ptr` objects based on workload (minutes to hours).
- Workers SHOULD set TTL on `result_ptr` objects (hours to days) to allow downstream consumption.
- Reapers MAY garbage-collect expired pointers; CAP does not prescribe retention policies.

## Integrity and Access Control
- Implementations SHOULD authenticate/authorize access to pointer locations (e.g., scoped Redis credentials, signed URLs).
- Pointers SHOULD NOT include secrets inline; prefer bearer tokens or signed URLs with short expiry.
- When redaction occurs, `redacted_context_ptr` SHOULD point to a sanitized copy with the same TTL policy as the original.
