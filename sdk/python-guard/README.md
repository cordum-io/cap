# cordum-guard

Safety governance for Python AI agents. Add Cordum safety checks to **existing** Python code with a single decorator — no rewrite required.

## Install

```bash
pip install cordum-guard
```

With LangChain or LlamaIndex support:

```bash
pip install cordum-guard[langchain]
pip install cordum-guard[llamaindex]
```

## Quick Start

```python
from cordum_guard import CordumClient, guard

client = CordumClient("http://localhost:8081", api_key="your-api-key")

@guard(client, policy="financial_ops", risk_tags=["write", "financial"])
def execute_transfer(amount: float, to_account: str):
    bank_api.transfer(amount, to_account)
```

The `@guard` decorator intercepts every call to `execute_transfer`:

1. Evaluates the safety policy via the Cordum Safety Kernel, sending the call's scalar
   arguments as `arg.*` labels (see [Policy labels](#policy-labels))
2. **allow** / **allow_with_constraints** — function runs normally
3. **deny** — raises `CordumBlockedError`
4. **require_approval** — submits the call as a job with the same labels and waits for human
   approval in the dashboard; values of secret-like arguments are masked in the job prompt
5. **throttle** — delays execution per policy
6. **any other verdict** — raises `CordumBlockedError` (fail closed)

## LangChain Integration

```python
from cordum_guard import CordumClient
from cordum_guard.langchain import CordumToolGuard

client = CordumClient("http://localhost:8081", api_key="your-api-key")
guarded_tools = CordumToolGuard(client, policy="agent_ops").wrap(tools)
agent = initialize_agent(guarded_tools, llm)
```

## LlamaIndex Integration

```python
from cordum_guard import CordumClient
from cordum_guard.llamaindex import CordumToolGuard

client = CordumClient("http://localhost:8081", api_key="your-api-key")
guarded_tools = CordumToolGuard(client, policy="agent_ops").wrap(tools)
```

`call()`, `acall()` and `real_fn` (which CodeActAgent hands to its code executor) are all
guarded.

## Tool guard decisions

`CordumToolGuard` (LangChain and LlamaIndex) evaluates every tool call, sync and async, and
treats every verdict except **allow** and **allow_with_constraints** as blocking:

| Verdict | Tool runs? | Agent / caller sees |
|---------|------------|---------------------|
| `allow` | yes | the tool result |
| `allow_with_constraints` | yes | the tool result; constraints are **not** enforced (see below) |
| `throttle` | yes, after the delay | the tool result |
| `deny` | no | `[BLOCKED] <tool>: <reason>` as the tool result |
| `require_approval` (kernel `REQUIRE_HUMAN`) | no | `CordumApprovalRequiredError` is raised |
| anything else (unknown, `DECISION_TYPE_UNSPECIFIED`) | no | `[BLOCKED] <tool>: <reason>` (fail closed) |

A response with `approvalRequired: true` is treated as `require_approval` unless the verdict
is `deny`.

```python
from cordum_guard import CordumApprovalRequiredError

try:
    agent.invoke({"input": "Transfer $15,000 from Bob to Alice"})
except CordumApprovalRequiredError as exc:
    # The tool did NOT run.
    print(exc.reason, exc.rule_id, exc.policy_snapshot, exc.approval_ref, exc.job_id)
```

Agent runtimes decide whether a tool exception reaches you. LangGraph's default `ToolNode`
re-raises it from `agent.invoke`. With `ToolNode(..., handle_tool_errors=True)` the model
receives the error text instead. In both cases the tool does not run.

`CordumApprovalRequiredError` subclasses `CordumBlockedError`, so existing
`except CordumBlockedError` handlers still fail closed. It carries `decision` (the full
`SafetyDecision`), `reason`, `rule_id`, `policy_snapshot`, `approval_ref` and `job_id`.
Cordum keys approvals by job ID, so `approval_ref == job_id`, and the gateway only sets them
when the evaluation was bound to a job: `/api/v1/policy/evaluate` echoes the request
`job_id`, and the tool guards evaluate without one, so both are empty for tool calls. An
evaluation creates no approval. To get a human decision, submit the action as a job with the
same labels (it appears under **Approvals** with its job ID) and poll it with
`client.wait_for_decision(job_id)`; that is what `@guard` does.

**allow_with_constraints:** the action runs, but in-process guards cannot apply the returned
constraints (budgets, sandbox, toolchain, diff, redaction level). They are available as
`SafetyDecision.constraints` from `client.evaluate_policy(...)` for callers that can enforce
them. Use `deny` or `require_approval` for rules that must hold in-process.

## Policy labels

Labels are sent at the top level of the evaluate request, where the Safety Kernel matches
`labels`, `label_allowlist` and `label_threshold` rules, and are also copied to
`meta.labels` for older consumers. The guards always send `policy` plus `tool` and
`description` (tool guards) or `function` (`@guard`).

**Arguments:** scalar call arguments are sent as `arg.<parameter name>` labels so policies can
see their values:

- Only `bool`, `int`, `float`, `Decimal` and `str` values are sent. Bools become
  `true` / `false`; numbers use `str()`, except NaN, which is sent as `inf` so
  `label_threshold` rules still trip. `None`, lists, dicts and objects are skipped.
- Positional arguments are named from the function signature; extra `*args` are not sent.
- Names must match `[A-Za-z_][A-Za-z0-9_]{0,63}`.
- At most 16 argument labels per call, in argument order. Values are cut at 256 characters.
- Arguments with secret-like names are never sent: `password`, `passwd`, `passphrase`,
  `secret`, `token`, `api_key` / `apikey`, `access_key`, `private_key`, `authorization`,
  `auth`, `credential(s)`, `cookie` or `session`, as a whole word of the name
  (`client_secret`, `accessToken` and `session_id` match; `max_tokens` does not). Do not pass
  credentials through other argument names.
- **Opt out** with `include_args=False` on `CordumToolGuard(...)` or `@guard(...)` when tools
  take sensitive free text. No `arg.*` labels are sent, and `@guard` also leaves argument values
  out of its approval job prompt. Rules on `arg.*` labels then never fire for that guard, so
  gate those tools by capability or risk tag instead.

The `arg.` prefix keeps model-chosen argument names out of label keys the kernel interprets
itself (`agent_id`, `secrets_present`, `mcp.*`, `_delegation.*`). Write policies against the
prefixed name:

```yaml
rules:
  - id: financial-approval
    match:
      capabilities: [transfer_funds]
      label_threshold:
        arg.amount: 10000
    decision: require_approval
    reason: "Transfers over $10,000 require human approval."
```

The kernel skips a `label_threshold` check when the label is missing or not numeric, so a
rule on an argument that is not sent (secret-like name, non-scalar, past the 16-label cap,
`include_args=False`) never fires.

## Async Support

The `@guard` decorator works with both sync and async functions:

```python
@guard(client, policy="ops")
async def async_operation():
    return await some_api_call()
```

## Configuration

### CordumClient

| Parameter     | Default       | Description                          |
|---------------|---------------|--------------------------------------|
| `gateway_url` | —             | Cordum gateway URL                   |
| `api_key`     | —             | API key for authentication           |
| `tenant_id`   | `"default"`   | Tenant identifier                    |
| `timeout`     | `30.0`        | HTTP request timeout (seconds)       |
| `cache_ttl`   | `0`           | Cache TTL in seconds (0 = disabled)  |
| `cache_max_size` | `1000`     | Max cached policy entries             |
| `on_error`    | `"closed"`    | Failure mode: `"closed"`, `"open"`, or callable |

### @guard decorator

| Parameter    | Default        | Description                              |
|--------------|----------------|------------------------------------------|
| `client`     | —              | CordumClient instance                    |
| `policy`     | `""`           | Policy name (label for traceability)     |
| `risk_tags`  | `[]`           | Risk tags sent to the Safety Kernel      |
| `capability` | function name  | Override the capability identifier       |
| `topic`      | `"job.guard"`  | NATS topic for policy evaluation         |
| `timeout`    | `300.0`        | Max wait for approval decisions (seconds)|
| `include_args` | `True`       | Send scalar arguments as `arg.*` labels; `False` also keeps them out of the approval prompt |

### CordumToolGuard (LangChain / LlamaIndex)

| Parameter    | Default        | Description                          |
|--------------|----------------|--------------------------------------|
| `client`     | —              | CordumClient instance                |
| `policy`     | `""`           | Policy name                          |
| `risk_tags`  | `[]`           | Risk tags for all wrapped tools      |
| `topic`      | `"job.guard"`  | NATS topic for evaluation            |
| `include_args` | `True`       | Send scalar tool arguments as `arg.*` labels |

## Testing

Use `MockCordumClient` to test guarded code without a live gateway:

```python
from cordum_guard import guard, MockCordumClient, Decision

mock = MockCordumClient(default_decision=Decision.ALLOW)
mock.set_policy_response("dangerous-ops", Decision.DENY)

@guard(mock, capability="safe-op")
def safe_func():
    return "works"

@guard(mock, capability="dangerous-ops")
def risky_func():
    return "blocked"

assert safe_func() == "works"
# risky_func() raises CordumBlockedError

# Inspect what was evaluated:
assert len(mock.call_log) == 1
assert mock.call_log[0].capability == "safe-op"
```

## Caching

Enable TTL-based caching to reduce gateway round-trips:

```python
client = CordumClient(
    gateway_url="http://localhost:8081",
    api_key="my-key",
    cache_ttl=30,       # cache decisions for 30s (0 = disabled)
    cache_max_size=500,  # max cached entries
)
# ALLOW/DENY/THROTTLE cached; REQUIRE_APPROVAL always fresh
# Bypass cache per-call: client.evaluate_policy(..., cache=False)
# Clear cache: client.clear_cache()
```

The cache key covers topic, capability, risk tags and labels, so calls with different
argument values never share a cached verdict.

## Failure Modes

Configure what happens when the gateway is unreachable:

```python
# Default: fail-closed (raise CordumConnectionError)
client = CordumClient("http://localhost:8081", api_key="key", on_error="closed")

# Fail-open: allow operations when gateway is down
client = CordumClient("http://localhost:8081", api_key="key", on_error="open")

# Callback: custom logic per error
def my_fallback(error):
    if "critical" in str(error):
        raise error  # fail closed for critical
    return SafetyDecision(decision=Decision.ALLOW)

client = CordumClient("http://localhost:8081", api_key="key", on_error=my_fallback)
```

Only connection/timeout errors trigger fail-open. Auth errors (401/403) and explicit DENY responses always propagate normally.

The client reads the gateway's protojson verdicts (`DECISION_TYPE_ALLOW`,
`DECISION_TYPE_REQUIRE_HUMAN`, ...) as well as plain names. A response with no verdict (for
example an empty 200 body) and an unrecognised verdict, including `DECISION_TYPE_UNSPECIFIED`,
both fail closed as DENY in every profile.

## License

Apache-2.0
