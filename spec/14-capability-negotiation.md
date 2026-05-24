# Capability Negotiation

CAP components advertise their role, supported protocol versions, and feature capabilities via a `Handshake` message published on connect. This enables schedulers and controllers to build a live registry of the topology and tailor behavior to each component's capabilities.

## Handshake Flow

1. Component connects to the message bus.
2. Component publishes `BusPacket{Handshake}` to `sys.handshake`.
3. Schedulers and controllers consume `sys.handshake` and record the component's role, supported versions, and capabilities in their component registry.
4. Schedulers MAY use the registry to route jobs only to workers that advertise required capabilities (e.g., `compensation`, `progress`).

Components SHOULD publish a Handshake immediately after establishing a bus connection. The Handshake is informational; no explicit acknowledgment is defined. Schedulers that need confirmation of capability support SHOULD use the registry rather than waiting for a response.

## Handshake Message

```proto
enum ComponentRole {
    COMPONENT_ROLE_UNSPECIFIED = 0;
    COMPONENT_ROLE_GATEWAY = 1;
    COMPONENT_ROLE_SCHEDULER = 2;
    COMPONENT_ROLE_WORKER = 3;
    COMPONENT_ROLE_ORCHESTRATOR = 4;
    COMPONENT_ROLE_CONTROLLER = 5;
}

message Handshake {
    string component_id = 1;
    ComponentRole role = 2;
    repeated int32 supported_versions = 3;
    map<string, bool> capabilities = 4;
    string sdk_version = 5;
    repeated string ready_topics = 6;
    string agent_name = 7;
}
```

See `proto/cordum/agent/v1/handshake.proto` for the canonical definition.

## Capability Registry

The `capabilities` map uses string keys with boolean values. A key set to `true` indicates the component supports that feature. A missing key or `false` value indicates the feature is not supported.

### Initial Capability Keys

| Key             | Description                                                        |
|-----------------|--------------------------------------------------------------------|
| `signatures`    | Component signs outgoing BusPackets and verifies incoming signatures. |
| `compensation`  | Worker supports compensation/rollback actions.                     |
| `progress`      | Worker emits `JobProgress` messages during execution.              |
| `cancel`        | Worker handles `JobCancel` requests.                               |
| `error_codes`   | Component populates `error_code_enum` in JobResult.                |
| `heartbeats`    | Component emits periodic Heartbeat messages.                       |
| `workflows`     | Component supports multi-step workflow orchestration.              |

Implementations MAY define additional capability keys. Custom keys SHOULD use a namespaced prefix (e.g., `x-myorg-feature`) to avoid collisions with future protocol-defined keys.

## Readiness Topics

`ready_topics` lists the concrete job subjects/topics the worker is currently ready to serve. This is distinct from `capabilities`:

- `capabilities` answers **what** the component can do.
- `ready_topics` answers **where** the worker is currently available to receive work.

Schedulers MAY use `ready_topics` as an additional dispatch filter before selecting a worker. This lets a worker advertise broad static capabilities while temporarily narrowing the set of routed topics during startup or reconfiguration.

Workers SHOULD publish `ready_topics` in a stable order so registries and tests can compare handshake payloads deterministically. Older workers that omit `ready_topics` remain wire-compatible; schedulers SHOULD treat the field as unknown/unspecified rather than as an error.

## Agent Display Name

`agent_name` (field 7) is an optional human-facing **display label** for the component (e.g., `Claude Code — Billing Bot`), surfaced in registries, dashboards, and audit summaries so operators can attribute activity to a recognizable name rather than an opaque `component_id`. SDKs sanitize and bound it (trim, collapse internal whitespace, drop control characters, cap at 128 characters).

`agent_name` is **not an authentication authority**. It is self-reported and spoofable, so schedulers, registries, and audit pipelines MUST prefer authenticated identity records (worker credential / Agent Identity) over it and MUST NOT use it for authorization or identity resolution. It MUST NOT carry secrets, tokens, or PII. Older components that omit `agent_name` remain wire-compatible; consumers SHOULD treat the field as unknown/unspecified rather than as an error.

## Version Negotiation

Components advertise the wire versions they support via `supported_versions`. Schedulers SHOULD pick the highest version common to both the scheduler and the target component. If no common version exists, the scheduler SHOULD reject jobs to that component with `ERROR_CODE_PROTOCOL_VERSION_MISMATCH`.

Currently, the only defined wire version is `1`. Components SHOULD include `1` in `supported_versions`.

## Reconnection and Liveness

- Components SHOULD re-publish their Handshake on every reconnect to the bus.
- Schedulers MAY expire registry entries for components that have not sent a Heartbeat or Handshake within a configured timeout.
- Components that never send a Handshake are assumed to support CORE-level capabilities only (backward compatibility with pre-handshake deployments). Schedulers SHOULD NOT require Handshake for basic job dispatch.
- If `ready_topics` is absent, schedulers SHOULD fall back to legacy behavior and rely on the existing routing/heartbeat signals rather than rejecting the worker outright.

## Security

- Handshake packets SHOULD be signed like any other BusPacket when signatures are enabled.
- Schedulers SHOULD verify Handshake signatures before trusting the advertised capabilities.
- The `component_id` in a Handshake SHOULD match the `sender_id` in the enclosing BusPacket.

## Subject

Handshake messages are published to `sys.handshake`. Schedulers and controllers SHOULD subscribe to this subject. Queue groups SHOULD NOT be used for `sys.handshake` so that all schedulers receive every Handshake.
