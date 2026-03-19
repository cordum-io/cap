# CrewAI + CAP Safety Governance

Demonstrates how the CAP `@guard` decorator adds safety governance to CrewAI tool functions. Three tools show the three safety decision types: **ALLOW**, **DENY**, and **REQUIRE_APPROVAL**.

## Prerequisites

- Python 3.9+

## Setup

```bash
cd examples/framework-crewai
pip install -r requirements.txt
pip install -e ../../sdk/python-guard
python main.py
```

## What It Does

The demo configures a `MockCordumClient` with three policies:

| Tool | Decision | Behavior |
| --- | --- | --- |
| `search_database` | ALLOW | Executes normally |
| `delete_file` | DENY | Blocked — raises `CordumBlockedError` |
| `send_email` | REQUIRE_APPROVAL | Submitted for approval, auto-approved by mock |

Each tool is a standard CrewAI `@tool` function with a `@guard` decorator that intercepts execution and evaluates the safety policy before the function runs.

## Expected Output

```
============================================================
  CrewAI + CAP Safety Governance Demo
============================================================

[1] search_database — Policy: ALLOW
----------------------------------------
  Result: Found 3 results for 'quarterly report': [report-Q1.pdf, ...]

[2] delete_file — Policy: DENY
----------------------------------------
  BLOCKED: CordumBlockedError(decision=deny, reason=Destructive file ...)

[3] send_email — Policy: REQUIRE_APPROVAL
----------------------------------------
  Result: Email sent to client@example.com: Q1 Report
  (MockCordumClient auto-approved the request)

============================================================
  CAP Audit Trail
============================================================
  1. capability=search_database     decision=allow          risk_tags=[]
  2. capability=delete_file         decision=deny           risk_tags=['destructive', 'write']
  3. capability=send_email          decision=require_approval risk_tags=['external', 'communication']
```

## How It Works

1. `MockCordumClient` simulates the Cordum Safety Kernel — no live gateway needed
2. `@guard(client, policy=..., capability=..., risk_tags=[...])` wraps each tool function
3. Before the tool executes, the guard calls `client.evaluate_policy()` with the tool's capability and risk tags
4. Based on the decision:
   - **ALLOW**: function executes normally
   - **DENY**: `CordumBlockedError` is raised, function never runs
   - **REQUIRE_APPROVAL**: a job is submitted for approval; on success the function executes
5. All decisions are logged in `mock.call_log` for audit

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
