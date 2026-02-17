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

1. Evaluates the safety policy via the Cordum Safety Kernel
2. **allow** — function runs normally
3. **deny** — raises `CordumBlockedError`
4. **require_approval** — waits for human approval in the dashboard
5. **throttle** — delays execution per policy

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

### @guard decorator

| Parameter    | Default        | Description                              |
|--------------|----------------|------------------------------------------|
| `client`     | —              | CordumClient instance                    |
| `policy`     | `""`           | Policy name (label for traceability)     |
| `risk_tags`  | `[]`           | Risk tags sent to the Safety Kernel      |
| `capability` | function name  | Override the capability identifier       |
| `topic`      | `"job.guard"`  | NATS topic for policy evaluation         |
| `timeout`    | `300.0`        | Max wait for approval decisions (seconds)|

### CordumToolGuard (LangChain / LlamaIndex)

| Parameter    | Default        | Description                          |
|--------------|----------------|--------------------------------------|
| `client`     | —              | CordumClient instance                |
| `policy`     | `""`           | Policy name                          |
| `risk_tags`  | `[]`           | Risk tags for all wrapped tools      |
| `topic`      | `"job.guard"`  | NATS topic for evaluation            |

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

## License

Apache-2.0
