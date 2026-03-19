"""LangChain + CAP Safety Governance Demo

Demonstrates how CordumToolGuard wraps LangChain tools with CAP safety decisions.
Three tools show different safety outcomes:
  - web_search: ALLOW (executes normally)
  - calculator: ALLOW (executes normally)
  - file_write: DENY (blocked by policy, returns [BLOCKED] message)

Uses MockCordumClient — no live infrastructure required.
"""

from typing import Optional, Type

from pydantic import BaseModel, Field
from langchain_core.tools import BaseTool
from langchain_core.callbacks.manager import CallbackManagerForToolRun

from cordum_guard.mock import MockCordumClient
from cordum_guard.types import Decision
from cordum_guard.langchain import CordumToolGuard


# ---------------------------------------------------------------------------
# 1. Define LangChain tools
# ---------------------------------------------------------------------------

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
            # Only allow safe math operations
            allowed = set("0123456789+-*/.(). ")
            if not all(c in allowed for c in expression):
                return "Error: invalid characters in expression"
            result = eval(expression, {"__builtins__": {}})  # noqa: S307
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


# ---------------------------------------------------------------------------
# 2. Configure MockCordumClient with safety policies
# ---------------------------------------------------------------------------

mock = MockCordumClient(default_decision=Decision.ALLOW)

# Block file system write operations
mock.set_policy_response(
    "file_write",
    Decision.DENY,
    reason="File system write access blocked by safety policy",
)

# web_search and calculator use the default: ALLOW


# ---------------------------------------------------------------------------
# 3. Wrap tools with CordumToolGuard
# ---------------------------------------------------------------------------

raw_tools = [WebSearchTool(), CalculatorTool(), FileWriteTool()]

guard = CordumToolGuard(mock, policy="langchain_demo", risk_tags=["agent"])
guarded_tools = guard.wrap(raw_tools)


# ---------------------------------------------------------------------------
# 4. Demo: Invoke each guarded tool
# ---------------------------------------------------------------------------

def main():
    print("=" * 60)
    print("  LangChain + CAP Safety Governance Demo")
    print("=" * 60)
    print()

    # Tool 1: web_search (ALLOW)
    print("[1] web_search — Policy: ALLOW")
    print("-" * 40)
    result = guarded_tools[0].invoke({"query": "CAP protocol specification"})
    print(f"  Result: {result}")
    print()

    # Tool 2: calculator (ALLOW)
    print("[2] calculator — Policy: ALLOW")
    print("-" * 40)
    result = guarded_tools[1].invoke({"expression": "42 * 3.14"})
    print(f"  Result: {result}")
    print()

    # Tool 3: file_write (DENY)
    print("[3] file_write — Policy: DENY")
    print("-" * 40)
    result = guarded_tools[2].invoke({"path": "/etc/secrets.txt", "content": "sensitive data"})
    print(f"  Result: {result}")
    print()

    # Audit trail
    print("=" * 60)
    print("  CAP Audit Trail")
    print("=" * 60)
    for i, entry in enumerate(mock.call_log, 1):
        resp = mock._responses.get(entry.capability) or mock._responses.get(entry.topic)
        decision_str = resp.decision.value if resp else mock._default_decision.value
        print(f"  {i}. tool={entry.capability:15s} "
              f"decision={decision_str:20s} "
              f"policy={entry.labels.get('policy', '')}")

    print()
    print("Done. CordumToolGuard demonstrated ALLOW and DENY decisions.")


if __name__ == "__main__":
    main()
