# Context and Memory Model

CAP treats context as a first-class concern: jobs point to external memory instead of embedding payloads, and workers may retrieve/summarize context based on hints provided in `JobRequest`.

## Logical Memory
- `memory_id` identifies the logical corpus for a job (e.g., `repo:foo/bar`, `chat:session-123`, `kb:incident-42`).
- Gateways SHOULD set `memory_id` when the job is tied to a durable corpus; workers MAY use it to drive retrieval or caching.

## Pointers (recap)
- `context_ptr`: pointer to input payload (written by client/gateway).
- `result_ptr`: pointer to output payload (written by worker/orchestrator).
- `redacted_context_ptr`: sanitized input (written by Safety Kernel when redaction is applied).
- Pointers are opaque URIs (`redis://ctx/<job_id>`, `s3://bucket/key`, `vector://kb/<id>`); consumers MUST NOT assume storage details.

## Context Hints
`JobRequest.context_hints` communicates how context should be handled:
- `max_input_tokens`: budget for input tokens per model call.
- `allow_summarization`: worker MAY summarize context to fit budgets.
- `allow_retrieval`: worker MAY perform retrieval over `memory_id` instead of using `context_ptr` verbatim.
- `tags`: optional labels to scope retrieval (`logs`, `code`, `prod`, etc.).

Workers SHOULD respect these hints when deciding whether to load context, retrieve from a memory fabric, or summarize before tool/model execution.

## Memory Fabric Expectations
- Memory backends (Redis, object storage, vector DBs) are outside CAP’s wire contract but MUST be addressable via pointers.
- TTLs and retention are implementation-defined; gateways/workers SHOULD set TTLs appropriate to workload and tenant policy.
- Access control applies at pointer targets: keep credentials scoped; prefer signed URLs or per-tenant tokens.

## Redaction and Safety
- When the Safety Kernel redacts input, it SHOULD write `redacted_context_ptr` and set policy reason in the safety response.
- Workers and schedulers SHOULD prefer `redacted_context_ptr` over `context_ptr` when present.
