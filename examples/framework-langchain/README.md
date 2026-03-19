# LangChain + CAP Safety Governance

Demonstrates how `CordumToolGuard` wraps LangChain tools with CAP safety decisions. Tools are evaluated against policy before execution — blocked tools return a `[BLOCKED]` message instead of running.

## Prerequisites

- Python 3.9+

## Setup

```bash
cd examples/framework-langchain
pip install -r requirements.txt
pip install -e ../../sdk/python-guard
python main.py
```

## What It Does

The demo configures a `MockCordumClient` with two policies:

| Tool | Decision | Behavior |
| --- | --- | --- |
| `web_search` | ALLOW | Executes normally |
| `calculator` | ALLOW | Executes normally |
| `file_write` | DENY | Returns `[BLOCKED]` message |

All three tools are standard LangChain `BaseTool` subclasses, wrapped with `CordumToolGuard.wrap()` which intercepts `_run` to evaluate policy before execution.

## Expected Output

```
============================================================
  LangChain + CAP Safety Governance Demo
============================================================

[1] web_search — Policy: ALLOW
----------------------------------------
  Result: Results for 'CAP protocol specification': [cordum.io - ...]

[2] calculator — Policy: ALLOW
----------------------------------------
  Result: 42 * 3.14 = 131.88

[3] file_write — Policy: DENY
----------------------------------------
  Result: [BLOCKED] file_write: File system write access blocked by safety policy

============================================================
  CAP Audit Trail
============================================================
  1. tool=web_search       decision=allow          policy=langchain_demo
  2. tool=calculator       decision=allow          policy=langchain_demo
  3. tool=file_write       decision=deny           policy=langchain_demo
```

## How It Works

1. `MockCordumClient` simulates the Cordum Safety Kernel — no live gateway needed
2. `CordumToolGuard(client, policy=...).wrap(tools)` monkey-patches each tool's `_run` method
3. Before the tool executes, the guard calls `client.evaluate_policy()` with the tool's name as the capability
4. Based on the decision:
   - **ALLOW**: `_run` executes normally, result returned
   - **DENY**: `_run` is never called, returns `[BLOCKED] tool_name: reason`
   - **THROTTLE**: sleeps for the configured duration, then executes
5. All decisions are logged in `mock.call_log` for audit

## Production Usage

Replace `MockCordumClient` with `CordumClient` to connect to a live Cordum gateway:

```python
from cordum_guard.client import CordumClient
from cordum_guard.langchain import CordumToolGuard

client = CordumClient(
    gateway_url="https://localhost:8081",
    api_key="your-api-key",
    tenant_id="default",
)

guard = CordumToolGuard(client, policy="production_ops")
guarded_tools = guard.wrap(your_tools)
agent = initialize_agent(guarded_tools, llm)
```

## Learn More

- [python-guard SDK](../../sdk/python-guard/)
- [CAP Spec](../../spec/00-index.md)
- [Cordum Documentation](https://github.com/cordum-io/cordum)
