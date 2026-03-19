# AutoGen + CAP Safety Governance

Demonstrates how the CAP `@guard` decorator adds safety governance to AutoGen tool functions. Three tools show the three safety decision types: **ALLOW**, **DENY**, and **REQUIRE_APPROVAL**.

## Prerequisites

- Python 3.9+

## Setup

```bash
cd examples/framework-autogen
pip install -r requirements.txt
pip install -e ../../sdk/python-guard
python main.py
```

## What It Does

The demo configures a `MockCordumClient` with three policies:

| Tool | Decision | Behavior |
| --- | --- | --- |
| `lookup_data` | ALLOW | Executes normally |
| `execute_code` | DENY | Blocked — raises `CordumBlockedError` |
| `summarize_report` | REQUIRE_APPROVAL | Submitted for approval, auto-approved by mock |

Each tool is a plain Python function with a `@guard` decorator, then wrapped as an AutoGen `FunctionTool`. The guard intercepts execution regardless of whether the tool is called directly or through AutoGen's agent loop. In a real scenario, pass the `FunctionTool` instances to an `AssistantAgent` with a model client.

## Expected Output

```
============================================================
  AutoGen + CAP Safety Governance Demo
============================================================

[1] lookup_data — Policy: ALLOW
----------------------------------------
  Result: Found 5 results for 'quarterly sales': [sales-Q1.csv, ...]

[2] execute_code — Policy: DENY
----------------------------------------
  BLOCKED: Action blocked by safety policy: Code execution blocked by safety policy

[3] summarize_report — Policy: REQUIRE_APPROVAL
----------------------------------------
  Result: Summary of 'Q1 Sales Report' prepared for board of directors: [...]
  (MockCordumClient auto-approved the request)

============================================================
  AutoGen Tool Integration
============================================================
  3 FunctionTools created from @guard-decorated functions
  Tool names: lookup_data, execute_code, summarize_report
  In a real scenario, pass these tools to AssistantAgent with a
  model_client — CAP guards run transparently on every invocation.

============================================================
  CAP Audit Trail
============================================================
  1. capability=lookup_data          decision=allow               risk_tags=[]
  2. capability=execute_code         decision=deny                risk_tags=['dangerous', 'compute']
  3. capability=summarize_report     decision=require_approval    risk_tags=['external', 'reporting']
```

## How It Works

1. `MockCordumClient` simulates the Cordum Safety Kernel — no live gateway needed
2. `@guard(client, policy=..., capability=..., risk_tags=[...])` wraps each tool function
3. Guarded functions are wrapped as `FunctionTool` instances for AutoGen agents
4. Before any tool executes, the guard calls `client.evaluate_policy()` with the tool's capability and risk tags
5. Based on the decision:
   - **ALLOW**: function executes normally
   - **DENY**: `CordumBlockedError` is raised, function never runs
   - **REQUIRE_APPROVAL**: a job is submitted for approval; on success the function executes
6. All decisions are logged in `mock.call_log` for audit

## Production Usage

Replace `MockCordumClient` with `CordumClient` to connect to a live Cordum gateway:

```python
from cordum_guard.client import CordumClient

client = CordumClient(
    gateway_url="https://localhost:8081",
    api_key="your-api-key",
    tenant_id="default",
)
```

## Learn More

- [python-guard SDK](../../sdk/python-guard/)
- [CAP Spec](../../spec/00-index.md)
- [Cordum Documentation](https://github.com/cordum-io/cordum)
