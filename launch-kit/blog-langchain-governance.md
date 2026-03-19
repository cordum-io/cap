# Adding Governance to Your LangChain Agent in 10 Minutes

LangChain makes it easy to build agents that call tools autonomously. But autonomy without oversight is a liability. What happens when your agent decides to write to the filesystem, send an email, or delete a database record? Without a safety layer, the tool executes — no questions asked.

**cordum-guard** adds a governance layer to your LangChain tools in about 10 lines of code. Every tool invocation is checked against a safety policy before it runs. Dangerous actions get blocked. Sensitive actions require approval. Safe actions execute normally. And everything is logged for audit.

Here's how to set it up.

## Step 1: Install

```bash
pip install "cordum-guard[langchain]" langchain-core
```

The `[langchain]` extra pulls in `langchain-core` and enables the `CordumToolGuard` wrapper. That's the only dependency you need — no separate adapter package.

## Step 2: Define Your Tools

Start with standard LangChain tools. Here are three that cover a range of risk levels — a web search, a calculator, and a filesystem writer:

```python
from typing import Optional, Type
from pydantic import BaseModel, Field
from langchain_core.tools import BaseTool
from langchain_core.callbacks.manager import CallbackManagerForToolRun


class WebSearchInput(BaseModel):
    query: str = Field(description="Search query")


class WebSearchTool(BaseTool):
    name: str = "web_search"
    description: str = "Search the web for information"
    args_schema: Type[BaseModel] = WebSearchInput

    def _run(self, query: str, run_manager: Optional[CallbackManagerForToolRun] = None) -> str:
        return f"Results for '{query}': [cordum.io - Agent Control Plane, cap spec - protocol docs]"


class CalculatorInput(BaseModel):
    expression: str = Field(description="Math expression to evaluate")


class CalculatorTool(BaseTool):
    name: str = "calculator"
    description: str = "Evaluate a mathematical expression"
    args_schema: Type[BaseModel] = CalculatorInput

    def _run(self, expression: str, run_manager: Optional[CallbackManagerForToolRun] = None) -> str:
        try:
            allowed = set("0123456789+-*/.(). ")
            if not all(c in allowed for c in expression):
                return "Error: invalid characters in expression"
            result = eval(expression, {"__builtins__": {}})
            return f"{expression} = {result}"
        except Exception as e:
            return f"Error: {e}"


class FileWriteInput(BaseModel):
    path: str = Field(description="File path to write to")
    content: str = Field(description="Content to write")


class FileWriteTool(BaseTool):
    name: str = "file_write"
    description: str = "Write content to a file on the filesystem"
    args_schema: Type[BaseModel] = FileWriteInput

    def _run(self, path: str, content: str, run_manager: Optional[CallbackManagerForToolRun] = None) -> str:
        return f"Wrote {len(content)} bytes to {path}"
```

Nothing special here — these are standard LangChain `BaseTool` subclasses. The governance comes next.

## Step 3: Configure Safety Policy

Create a `MockCordumClient` and define which tools should be allowed, denied, or throttled:

```python
from cordum_guard.mock import MockCordumClient
from cordum_guard.types import Decision

mock = MockCordumClient(default_decision=Decision.ALLOW)

# Block file system write operations
mock.set_policy_response(
    "file_write",
    Decision.DENY,
    reason="File system write access blocked by safety policy",
)
```

The `default_decision` controls what happens to tools without an explicit policy — here, everything is allowed unless you say otherwise. The `set_policy_response` call configures `file_write` to be denied. CAP supports four decision types:

- **ALLOW** — tool executes normally
- **DENY** — tool is blocked, returns a `[BLOCKED]` message
- **THROTTLE** — tool executes after a configurable delay
- **REQUIRE_APPROVAL** — tool waits for human approval before executing

## Step 4: Wrap Your Tools

This is the key step — one line that adds governance to all your tools at once:

```python
from cordum_guard.langchain import CordumToolGuard

raw_tools = [WebSearchTool(), CalculatorTool(), FileWriteTool()]

guard = CordumToolGuard(mock, policy="langchain_demo", risk_tags=["agent"])
guarded_tools = guard.wrap(raw_tools)
```

`CordumToolGuard.wrap()` returns new tool instances with safety checks injected into `_run`. Your original tools are unchanged. Pass `guarded_tools` to your agent instead of the originals.

## Step 5: Run and Observe

Invoke each guarded tool to see safety decisions in action:

```python
guarded_tools[0].invoke({"query": "CAP protocol specification"})
# => "Results for 'CAP protocol specification': [cordum.io - ...]"

guarded_tools[1].invoke({"expression": "42 * 3.14"})
# => "42 * 3.14 = 131.88"

guarded_tools[2].invoke({"path": "/etc/secrets.txt", "content": "sensitive data"})
# => "[BLOCKED] file_write: File system write access blocked by safety policy"
```

The first two tools execute normally — their capability matched the default ALLOW policy. The third was blocked before `_run` ever executed. The agent receives a clear `[BLOCKED]` message it can reason about.

Every decision is recorded in the audit trail:

```python
for entry in mock.call_log:
    print(f"tool={entry.capability}  decision=...")
# tool=web_search      decision=allow
# tool=calculator      decision=allow
# tool=file_write      decision=deny
```

## Going to Production

`MockCordumClient` is great for development and testing. For production, swap it for a real `CordumClient` that connects to the Cordum Safety Kernel:

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
```

The real Safety Kernel gives you centralized policy management, `REQUIRE_APPROVAL` for human-in-the-loop workflows (approvals appear in the Cordum dashboard), full audit logging with trace IDs, and policy evaluation caching for performance. Your tool code stays exactly the same — only the client changes.

## Next Steps

- Run the full example: [`examples/framework-langchain/`](../examples/framework-langchain/)
- See the same pattern with CrewAI: [`examples/framework-crewai/`](../examples/framework-crewai/)
- See the same pattern with AutoGen: [`examples/framework-autogen/`](../examples/framework-autogen/)
- Read the python-guard SDK docs: [`sdk/python-guard/`](../sdk/python-guard/)
- Explore the full CAP specification: [`spec/`](../spec/00-index.md)
