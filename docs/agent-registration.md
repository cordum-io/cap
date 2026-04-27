# Agent Registration via CAP

This page documents the CAP `AgentClient` API for registering, looking up, and scoping AI agent identities against a Cordum control plane. It is the source-of-truth wrapper that service bootstraps depend on for idempotent registration, scope updates, and deterministic preapproval revocation.

## Surface

`sdk/go/agent.go` (Go reference SDK) exposes:

```go
type AgentClient struct {
    BaseURL    string // e.g. https://gateway.example.com
    APIKey     string // service-account X-API-Key (used for register/lookup/scope only)
    Tenant     string // tenant header value
    HTTPClient *http.Client
}

func (c *AgentClient) Register(ctx context.Context, spec AgentSpec) (id string, err error)
func (c *AgentClient) Lookup(ctx context.Context, name, tenant string) (*AgentIdentity, error)
func (c *AgentClient) SetScope(ctx context.Context, update AgentScopeUpdate) error
```

The wrappers hit the Cordum control-plane REST endpoints directly:

| Operation | HTTP                                  | Notes                                                                                  |
| --------- | ------------------------------------- | -------------------------------------------------------------------------------------- |
| Register  | `POST /api/v1/agents`                 | `AgentSpec` deliberately omits `preapproved_mutating_tools` — see [Scope rules](#scope-rules). |
| Lookup    | `GET /api/v1/agents?name=...`         | Single-result idempotency check; multiple matches return an error.                     |
| SetScope  | `PUT /api/v1/agents/{id}`             | Always sends `preapproved_mutating_tools` (including empty `[]`) for deterministic revoke. Supports an `Idempotency-Key` header. |

## Scope rules

- **Preapproval is post-registration.** `Register` never grants preapproved mutating tools; the create-time payload is intentionally narrow. Operators grant preapproval through `SetScope` after a separate review of the tool list. This split mirrors `registerAgentArgs` vs. `updateAgentRequest` in the gateway and keeps audit lineage clean.
- **`SetScope` is the revoke path.** Callers always pass the full `preapproved_mutating_tools` list — including `[]` to revoke everything — so the resulting `AgentIdentity` reflects exactly the operator's intent. Never rely on partial / merge semantics here.

## Real-world consumer: `cordum-llm-chat`

The Cordum LLM chat assistant (the platform's own copilot) is a CAP agent registered with `AgentClient` on first boot. The bootstrap is idempotent and audited like any other Cordum agent.

- **Bootstrap source:** [`core/llmchat/bootstrap.go`](https://github.com/cordum-io/cordum/blob/main/core/llmchat/bootstrap.go) — calls `Lookup("chat-assistant", tenant)` first, falls through to `Register` + `SetScope` on miss; refuses divergent identities.
- **Wire chain on first boot:**
  1. `Register(AgentSpec{Name: "chat-assistant", RiskTier: medium, AllowedTools: 14 read + cordum_query_policy + 5 mutators})`.
  2. `SetScope(AgentScopeUpdate{ID: <new-id>, PreapprovedMutatingTools: ["cordum_submit_job"]})` — exactly one preapproved tool per the chat assistant's epic rail #4 ("widening requires a policy-bundle update by an admin, not a code change").
- **Audit emission:** the `chat.bootstrap_registered` SIEM action (constant defined in [`core/audit/siem_actions.go`](https://github.com/cordum-io/cordum/blob/main/core/audit/siem_actions.go)) fires on first boot only; restart re-emits **zero** registration events because `Lookup` short-circuits.
- **Senior-review evidence:** the dogfooding QA — including the scope-first deny ordering, audit chain integrity under chat-driven load, and the per-session delegation token revocation flow — is recorded in [`cordum/docs/llmchat/governance-review.md`](https://github.com/cordum-io/cordum/blob/main/docs/llmchat/governance-review.md). See probe 1 for the round-trip evidence and probe 7 for the scope-filter ordering.

This pattern is the recommended reference for any future Cordum service that needs its own CAP-registered agent identity: never roll your own MCP fallback, never widen preapproval at registration time, always go through `SetScope` for revoke.

## Related

- [Getting started](getting-started.md) — minimal CAP worker.
- [Reference](reference.md) — full CAP wire contract.
- [Cordum LLM chat overview](https://github.com/cordum-io/cordum/blob/main/docs/llmchat/overview.md) — where this agent registration runs in production.
