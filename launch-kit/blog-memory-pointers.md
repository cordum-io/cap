# How CAP Solves the Context Window Problem with Pointers

LLM context windows are finite and buses choke on large payloads. CAP avoids both by using pointers.

## The Problem
- Huge prompts and artifacts don’t fit comfortably on pub/sub payload limits.
- Inlining data leaks secrets into transit logs and observability pipelines.

## CAP’s Pointer Approach
- `context_ptr` and `result_ptr` are opaque URIs (Redis, S3, HTTPS, etc.).
- Buses carry envelopes, not blobs.
- Safety Kernel can return a `redacted_context_ptr` to sanitize before dispatch.

## Benefits
- Scale: payload size decoupled from bus limits.
- Security: data stays in controlled storage with ACLs/signed URLs.
- Performance: workers pull only what they need; schedulers act on metadata.

## Implementation Tips
- TTL inputs (minutes/hours) and outputs (hours/days) by tenant/pool.
- Prefer JSON/MsgPack for structured data; chunk large binaries.
- Don’t put secrets in pointers—use scoped credentials or signed URLs.

## Takeaway
Stop fighting context windows and bus size limits. Use pointers and keep the wire fast, cheap, and safer.
