# Memory Pointers

CAP keeps large payloads off the bus. Jobs reference input and output via opaque pointers.

## Pointer Semantics
- `context_ptr`: points to immutable input for the job.
- `result_ptr`: points to output produced by the worker.
- Pointers are opaque URIs; consumers must not infer structure beyond the scheme.
- Pointers must remain valid for the life of the job (and optionally for retention windows).

## Recommended Schemes
- Redis-style keys (example):
  - `redis://ctx/<job_id>`
  - `redis://res/<job_id>`
- Object storage or HTTP URLs are also valid as long as access is authorized.

## Guidance
- Do not inline large payloads on the bus; use pointers instead.
- Keep payloads small and JSON-friendly when possible.
- Results should clearly indicate success or failure semantics in their payload structure (e.g., `{"ok":true,"data":...}`).

## Example Context and Result Payloads

Context JSON stored at `redis://ctx/job-123`:
```json
{
  "prompt": "Summarize the document at https://example.com/spec.pdf",
  "tenant": "acme",
  "priority": "interactive"
}
```

Result JSON stored at `redis://res/job-123`:
```json
{
  "ok": true,
  "summary": "... concise summary ...",
  "worker_id": "worker-summarize-1",
  "completed_at": "2025-01-10T12:34:56Z"
}
```
