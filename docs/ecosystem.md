# CAP in the agent ecosystem

CAP is one layer in a stack of complementary open standards. It does not compete
with A2A or MCP — it sits alongside them and depends on none of them. This page
describes what each layer does, according to its current published specification,
and where CAP fits.

## The layers

```mermaid
flowchart TB
    subgraph Collaboration
      A2A["A2A — peer agent discovery,<br/>collaboration, task & artifact exchange"]
    end
    subgraph Capabilities
      MCP["MCP — tools, resources,<br/>and prompts for a model/agent"]
    end
    subgraph Execution
      CAP["CAP — broker-native governed<br/>workload admission, dispatch,<br/>worker pools, workflows"]
    end
    subgraph CrossCutting["Cross-cutting"]
      OTel["OpenTelemetry — telemetry<br/>and trace correlation"]
      CE["CloudEvents — portable<br/>event representation"]
    end
    A2A --> CAP
    MCP --> CAP
    CAP -. emits traces .-> OTel
    CAP -. can be mapped to .-> CE
```

### A2A (Agent2Agent)

A2A is a Linux Foundation-governed protocol for **agent-to-agent collaboration**:
agents advertise capabilities (Agent Cards), discover peers, and exchange tasks
and artifacts. A2A answers "which agent can help, and how do two agents talk?"

### MCP (Model Context Protocol)

MCP connects a host/model to **tools, resources, and prompts** through MCP
servers over stdio or Streamable HTTP. MCP answers "what capabilities and
context can this model reach?" An MCP client can live inside a CAP worker.

### CAP (Cordum Agent Protocol)

CAP is the **execution and governance** layer for distributed agent workloads on
a message broker. Its differentiator is broker-native operational concerns that
neither A2A nor MCP defines:

- job admission with policy checks before dispatch (Safety Kernel hook);
- routing to worker pools with queue groups;
- worker capacity and liveness via heartbeats;
- attempt fencing and retry semantics;
- multi-step workflow execution with parent/child correlation;
- payload-light envelopes (pointers keep blobs off the bus).

CAP answers "how is a governed workload admitted, dispatched, executed, and
tracked across a pool of workers?"

### OpenTelemetry

CAP carries a stable `trace_id` so activity correlates in an OpenTelemetry
backend. OpenTelemetry is **telemetry and correlation only**; it is **not**
authoritative for tenant, principal, or policy identity — those come from
authenticated transport/session records, never from a self-reported trace or
label.

### CloudEvents

CloudEvents is a portable **event envelope**. A CAP event can be represented as a
CloudEvent for interop with event-driven infrastructure. CloudEvents describes
event shape; it does not define admission, dispatch, or worker semantics.

## Operational scenarios

- **Governed tool call at scale.** An MCP client inside a CAP worker calls a
  tool. CAP admits the job through the Safety Kernel, dispatches it to the
  worker pool, fences the attempt, and records the terminal result — governance
  MCP itself does not provide.
- **Multi-agent task hand-off.** Two agents negotiate a task over A2A; the
  accepting agent runs the actual work as a CAP job so it is policy-checked,
  load-balanced across a pool, and retried on failure.
- **Correlated observability.** A workflow fans out child CAP jobs; each carries
  the parent `trace_id`, so the whole tree is reconstructable in an
  OpenTelemetry backend without CAP being the telemetry store.

## What CAP is not

CAP is deliberately narrow. CAP is **not**:

- a model or prompt API (that is the model provider / MCP);
- an agent framework or SDK-for-building-agents;
- a replacement for MCP or A2A — it complements them;
- a message broker — it runs **on** a broker (NATS today);
- a scheduler or control-plane **product** — it is the wire contract a control
  plane implements (Cordum is one such reference control plane);
- a blob store — payloads live behind pointers in external storage;
- a telemetry backend — it emits correlation data for OpenTelemetry;
- a generic event envelope — it is a job/execution protocol, not CloudEvents;
- a security guarantee **by conformance** — CAP defines safety and signing
  hooks, but conforming to CAP does not by itself make a deployment secure;
  operators must configure identity, policy, and key management.

## Transport support

- **NATS — supported.** Core NATS request/reply and pub/sub, exercised by an
  end-to-end job round-trip in CI (see `../spec/09-transport-profile.md`).
- **Kafka, RabbitMQ, and other buses — experimental.** No behavioral
  conformance evidence yet; not supported bindings until a transport
  conformance suite (TCK) demonstrates the required semantics.

The machine-readable transport status lives in `../release/manifest.json` and is
rendered into `reference.md`.

## Reference control plane

Cordum is a **reference control plane** that implements CAP (API Gateway,
Scheduler, Safety Kernel, Workflow Engine). CAP is the open protocol; Cordum is
one implementation of it. Anyone can implement CAP.

## Interoperability

These interop pieces ship today. None of them changes the CAP wire format.

| Standard | Version | What ships | Reference |
| --- | --- | --- | --- |
| A2A | 1.0, through `github.com/a2aproject/a2a-go/v2` v2.6.0 | `cmd/cap-a2a-bridge`, a draft CAP-native gateway. It serves SendMessage, GetTask and CancelTask, turns each authenticated message into a signed CAP job, and carries input-required and auth-required in a versioned turn-result envelope. | [mapping](../spec/interop/a2a-mapping.md), [example](../examples/a2a-bridge/) |
| W3C Trace Context | Level 1 | Lowercase `traceparent`/`tracestate` NATS headers on job submit, process and result in the Go, Python and Node SDK clients and Agent runtimes. Context is extracted only after admission or validation, and baggage is never propagated. | [contract](../spec/interop/trace-context.md) |
| OpenTelemetry messaging semantic conventions | v1.44.0 (Development status upstream) | PRODUCER `publish` and CONSUMER `process` spans with the attributes pinned by the trace-context vectors. | [contract](../spec/interop/trace-context.md) |
| CloudEvents | specversion 1.0 | A mapping, not an exporter, for job progress, job result, alert and workflow-event packets, with deterministic vectors. No CAP SDK or Cordum component emits these events. | [mapping](../spec/interop/cloudevents-mapping.md) |
| AsyncAPI | 3.1.0 | A description of the 12 NATS subjects in the spec/09 subject table, checked in CI together with the interop vectors. | [contract](../asyncapi/cap.asyncapi.yaml) |
| MCP | 2024-11-05, as implemented by Cordum | A CAP worker demo in Cordum calls Cordum's governed MCP endpoint. Its mutating `cordum_create_workflow` call waits for a distinct operator's approval. | [Cordum demo](https://github.com/cordum-io/cordum/tree/main/demo/cap-mcp-governed) |

**A2A production limitation.** The bridge is admitted only while the Cordum
scheduler runs with handshake mode `off` or `warn`, which is the compat profile.
Under `enforce` or CAP-PRODUCTION it is rejected. The CAP-only end-to-end test
uses an ungoverned test scheduler; the governed run is a Cordum CI workflow.

## Interoperability roadmap

The following are **roadmap**, not current CAP behavior — do not treat them as
shipped:

- a CloudEvents exporter (today there is only the mapping);
- a typed `WorkflowEvent` payload;
- A2A streaming (`SendStreamingMessage` and `SubscribeToTask` are unsupported);
- A2A artifacts carried as `ResourceRef`s;
- the six Cordum MCP mutating hooks other than `cordum_create_workflow`, which
  are not wired yet;
- trace propagation in the Go SDK's `worker.Worker` and `ManagedWorker`;
- production A2A fronting through the Cordum gateway.
