# CAP Examples

Runnable examples and wire-format samples for the Cordum Agent Protocol.

## Runnable Examples

| Example | Description | Language | Prerequisites |
| --- | --- | --- | --- |
| [simple-echo/](simple-echo/) | End-to-end job submission and result with sequence diagram | Go, Python, Node | NATS server |
| [workflow-repo-review/](workflow-repo-review/) | Multi-step parent/child workflow with aggregation | Go, Python, Node | NATS server |
| [framework-langchain/](framework-langchain/) | LangChain tools wrapped with CAP safety governance via `CordumToolGuard` | Python | Python 3.9+ |
| [framework-crewai/](framework-crewai/) | CrewAI agents with CAP `@guard` decorator for safety decisions | Python | Python 3.9+ |
| [framework-autogen/](framework-autogen/) | AutoGen agents with CAP `@guard` decorator and `FunctionTool` integration | Python | Python 3.9+ |

The framework examples use `MockCordumClient` — no NATS server or live infrastructure required.

## Wire Format Examples

Reference JSON payloads showing the CAP wire format:

| File | Description |
| --- | --- |
| [job-request.json](job-request.json) | Minimal `BusPacket{JobRequest}` for an echo pool |
| [job-request-compensation.json](job-request-compensation.json) | `JobRequest` with a compensation template for rollback |
| [job-result.json](job-result.json) | Matching `BusPacket{JobResult}` |
| [job-result-fatal.json](job-result-fatal.json) | `JobResult` with `JOB_STATUS_FAILED_FATAL` (rollback trigger) |
| [heartbeat.json](heartbeat.json) | Standalone `BusPacket{Heartbeat}` advertising pool membership and load |

All timestamps are illustrative; replace IDs and pointers as needed.

## Getting Started

- [Getting Started guide](../docs/getting-started.md) — set up NATS and run your first CAP worker
- [python-guard SDK](../sdk/python-guard/) — safety governance for Python AI agents
- [CAP Spec](../spec/00-index.md) — full protocol specification
