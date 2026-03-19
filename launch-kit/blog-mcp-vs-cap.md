# MCP vs CAP — Why Your AI Agents Need Both Protocols

MCP changed how models interact with tools. It gave us a clean, standardized interface for tool calling — and it works. But as teams move from single-model prototypes to distributed agent fleets, a new set of problems emerges that MCP was never designed to solve.

This isn't a criticism. MCP and CAP operate at different layers of the stack. Understanding where each one fits is the key to building agents that are both capable *and* governable.

## What MCP Does Well

Credit where it's due: MCP solved a real problem elegantly.

Before MCP, every model-to-tool integration was bespoke. Each vendor had its own function-calling format, its own schema conventions, its own error handling. MCP standardized this into a clean contract: a model declares what tools it can call, the runtime validates inputs against JSON schemas, and results flow back through a typed interface.

For single-model tool calling, MCP is excellent. It defines the conversation between a model and its immediate environment — read a file, query a database, call an API. The protocol is simple, the tooling ecosystem is growing fast, and the developer experience is smooth.

If your architecture is one model, one runtime, and a set of local tools — MCP is all you need.

## Where MCP Stops

The moment you have two agents that need to coordinate, MCP's boundaries become visible. Not because MCP is broken, but because these concerns were never in scope:

**Scheduling.** When 50 agents compete for work across 5 pools, who decides which agent handles which job? MCP has no concept of queue groups, pool routing, priority lanes, or retry policies. Every team rolls their own dispatcher.

**Safety.** MCP tool calls execute immediately. There's no pre-dispatch policy hook — no way to evaluate whether an agent *should* run a tool before it runs. Deleting a production database and reading a public API get the same treatment.

**State.** What happens when an agent crashes mid-execution? MCP has no job lifecycle — no way to distinguish "running" from "timed out" from "failed but retryable." Recovery is ad hoc.

**Liveness.** How do you know which agents are alive, their current load, and remaining capacity? MCP has no heartbeat protocol. Schedulers can't route work to healthy agents because they don't know who's healthy.

**Workflows.** Multi-step operations — "search, then analyze, then summarize, then email" — require parent-child job coordination, step-level tracing, and compensation on failure. MCP is single-call by design.

These aren't edge cases. They're the table stakes of production agent infrastructure.

## What CAP Adds

The Cordum Agent Protocol (CAP) is an open wire protocol (Apache-2.0) that fills exactly these gaps. It standardizes how gateways, schedulers, workers, orchestrators, and safety services exchange jobs over a pub/sub bus.

Every message is a **BusPacket** — a protobuf envelope carrying a `trace_id`, `sender_id`, protocol version, and exactly one payload. Jobs follow a deterministic **state machine** (`PENDING → SCHEDULED → DISPATCHED → RUNNING → SUCCEEDED | FAILED | DENIED`). Large payloads stay off the wire via **pointers** (`context_ptr`, `result_ptr`), keeping the bus lean and secure.

Before any job is dispatched, the scheduler calls a **Safety Kernel** — a dedicated policy decision point that returns ALLOW, DENY, REQUIRE_APPROVAL, or THROTTLE. Safety is a protocol requirement, not an afterthought.

**Heartbeats** on `sys.heartbeat` report pool membership, load, and capacity. **Workflows** use `workflow_id` and `parent_job_id` metadata to coordinate multi-step job graphs. And the entire protocol runs over any pub/sub that supports subjects and competing consumers — NATS by default, Kafka as an alternative.

## They Work Together

Here's the concrete picture. In Cordum's ecosystem, the [MCP-bridge pack](https://github.com/cordum-io/cordum-packs) wraps an MCP server as a CAP worker. An MCP tool call becomes a governed CAP job:

```
Agent                    CAP Bus                  MCP-Bridge Worker        MCP Server
  |                        |                          |                      |
  |-- BusPacket{JobRequest} -->                       |                      |
  |                        |-- Safety Kernel check -->|                      |
  |                        |<-- ALLOW/DENY -----------|                      |
  |                        |                          |-- MCP tool_call ---->|
  |                        |                          |<-- MCP result -------|
  |                        |<-- BusPacket{JobResult} --|                      |
  |<-- result -------------|                          |                      |
```

The MCP tool call itself is unchanged — same schema, same interface. But now it's wrapped in a CAP job with a safety check, a state machine, heartbeat monitoring, and an audit trail. Read-only MCP tools get an ALLOW decision and execute immediately. Write operations get REQUIRE_APPROVAL and pause for human sign-off.

MCP defines *what the tool does*. CAP defines *whether the tool is allowed to run*.

## Comparison Table

| Concern | MCP | CAP |
| --- | --- | --- |
| **Scope** | Single model ↔ tools | Distributed agent clusters |
| **Transport** | stdio / HTTP | Any pub/sub (NATS, Kafka) |
| **Safety** | None | Pre-dispatch Safety Kernel |
| **State** | No lifecycle | Deterministic state machine |
| **Scheduling** | None | Pool routing, queue groups, retries |
| **Workflows** | Single call | Parent/child jobs, DAG steps |
| **Memory** | Inline payloads | Pointer-based, off the wire |
| **Heartbeats** | None | Liveness, load, capacity |
| **License** | MIT | Apache-2.0 |

## When to Use Which

**MCP alone** — when your architecture is a single model calling local tools. MCP handles it cleanly.

**CAP alone** — when you're building distributed agent infrastructure and don't need MCP's tool-calling conventions. CAP workers can consume jobs directly via protobuf.

**Both** — when you want the best of both: MCP's tool ecosystem with CAP's governance layer. Run MCP inside your CAP workers. Every tool call gets scheduling, safety, state tracking, and auditability for free.

The choice isn't MCP *or* CAP. It's recognizing that they solve different problems at different layers — and that production agents need both.

---

[CAP Protocol](https://github.com/cordum-io/cap) · [Cordum](https://github.com/cordum-io/cordum) · [Integration Packs](https://packs.cordum.io) · [Discord](https://discord.gg/U4NpXtjP)
