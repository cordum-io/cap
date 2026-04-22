# Phase-2 Worker Handshake — Adapter Transparency Note

The CAP Go runtime's `Agent.Start()` automatically sends a Phase-2 worker handshake to the scheduler before subscribing to job topics. This is a transparent upgrade for framework adapter maintainers.

## For adapter maintainers

If your adapter re-vends `Agent.Start()` (LangChain, CrewAI, AutoGen, OpenAI Agents SDK), **no code change is required**. Pass through the existing `Agent` configuration; the runtime handles the handshake.

Useful fields if your adapter wants to expose explicit overrides to the user:

| Field | Purpose | Default |
|---|---|---|
| `Tenant` | Scope the agent to a tenant record in `AgentIdentityStore`. | empty (handshake skipped in enforce mode when empty) |
| `SDKVersion` | Version string used by the scheduler to bucket fleets. Adapters typically set this to `"cordum-<framework>/<version>"`. | `"cap-go/v2"` |
| `HandshakeMode` | `off` / `warn` / `enforce`. | `off` |
| `HandshakeTimeout` | Request/reply deadline. | 10s |
| `HandshakeRetries` | Exponential-backoff retry count before giving up. | 3 |

## Operator rollout

Operators control the scheduler-side enforcement via `CORDUM_HEARTBEAT_MODE` + `CORDUM_SDK_HANDSHAKE`. The SDK-side default is `off` so existing deploys keep working; operators flip adapters to `warn` → `enforce` on the same cadence as the scheduler-side flag.

## Verifying the upgrade

After upgrading your adapter:

1. Set `Agent.HandshakeMode = HandshakeModeWarn` in your adapter's initialisation code.
2. Start an agent. Check scheduler logs for `handshake accepted` with your `agent_id`.
3. Trigger a job. Check scheduler logs for `session token valid` on the inbound packet.

If the scheduler rejects the handshake, inspect the `reason` field in the structured log. The `HandshakeReject*` constants in `cap/sdk/go/handshake.go` document every possible reason.

## Non-Go SDKs

The Python and Node SDKs expose an equivalent surface. The Phase-2 handshake wire format is identical across languages so an adapter bundled with a Python agent and a Go scheduler interoperates transparently.

See the per-SDK READMEs for language-specific configuration.
