"""AutoGen + CAP Safety Governance Demo

Demonstrates how CAP's @guard decorator adds safety governance to AutoGen tool
functions. Three tools show the three safety decision types:
  - lookup_data: ALLOW (executes normally)
  - execute_code: DENY (blocked by policy)
  - summarize_report: REQUIRE_APPROVAL (approved via mock, then executes)

Uses MockCordumClient — no live infrastructure required.
"""

from cordum_guard.mock import MockCordumClient
from cordum_guard.types import Decision
from cordum_guard.guard import guard
from cordum_guard.exceptions import CordumBlockedError

from autogen_core.tools import FunctionTool
from autogen_agentchat.agents import AssistantAgent


# ---------------------------------------------------------------------------
# 1. Configure MockCordumClient with safety policies
# ---------------------------------------------------------------------------

mock = MockCordumClient(default_decision=Decision.ALLOW)

# Code execution is blocked — high-risk operation
mock.set_policy_response(
    "execute_code",
    Decision.DENY,
    reason="Code execution blocked by safety policy",
)

# Report summarization requires human approval
mock.set_policy_response(
    "summarize_report",
    Decision.REQUIRE_APPROVAL,
    reason="Report summarization requires human approval before sending",
)

# lookup_data uses the default: ALLOW


# ---------------------------------------------------------------------------
# 2. Define tool functions with CAP safety guards
# ---------------------------------------------------------------------------

@guard(mock, policy="data_ops", capability="lookup_data")
def lookup_data(query: str) -> str:
    """Look up data from the internal knowledge base."""
    return f"Found 5 results for '{query}': [sales-Q1.csv, forecast-2026.pdf, team-metrics.json, budget.xlsx, roadmap.md]"


@guard(mock, policy="compute_ops", capability="execute_code", risk_tags=["dangerous", "compute"])
def execute_code(code: str) -> str:
    """Execute arbitrary code on the system."""
    return f"Executed: {code}"


@guard(mock, policy="reporting", capability="summarize_report", risk_tags=["external", "reporting"])
def summarize_report(report_name: str, audience: str) -> str:
    """Summarize a report for a given audience."""
    return f"Summary of '{report_name}' prepared for {audience}: [3 key findings, 2 recommendations]"


# ---------------------------------------------------------------------------
# 3. Wrap guarded functions as AutoGen FunctionTools
# ---------------------------------------------------------------------------

# FunctionTool wraps plain callables for use with AutoGen agents.
# The @guard decorator is transparent — AutoGen sees a normal function,
# but CAP safety checks run before every invocation.
lookup_tool = FunctionTool(lookup_data, description="Look up data from the internal knowledge base")
execute_tool = FunctionTool(execute_code, description="Execute arbitrary code on the system")
summarize_tool = FunctionTool(summarize_report, description="Summarize a report for a given audience")

# In a real scenario, you would pass these tools to an AssistantAgent with
# a model_client (LLM). The agent decides which tools to call, and the
# @guard decorator enforces safety policy on every invocation:
#
#   assistant = AssistantAgent(
#       name="research_assistant",
#       model_client=your_llm_client,
#       tools=[lookup_tool, execute_tool, summarize_tool],
#   )


# ---------------------------------------------------------------------------
# 4. Demo: Call each guarded function to show safety decisions
# ---------------------------------------------------------------------------

def main():
    print("=" * 60)
    print("  AutoGen + CAP Safety Governance Demo")
    print("=" * 60)
    print()

    # --- Tool 1: lookup_data (ALLOW) ---
    print("[1] lookup_data — Policy: ALLOW")
    print("-" * 40)
    try:
        result = lookup_data(query="quarterly sales")
        print(f"  Result: {result}")
    except CordumBlockedError as e:
        print(f"  BLOCKED: {e}")
    print()

    # --- Tool 2: execute_code (DENY) ---
    print("[2] execute_code — Policy: DENY")
    print("-" * 40)
    try:
        result = execute_code(code="import os; os.system('rm -rf /')")
        print(f"  Result: {result}")
    except CordumBlockedError as e:
        print(f"  BLOCKED: {e}")
    print()

    # --- Tool 3: summarize_report (REQUIRE_APPROVAL) ---
    print("[3] summarize_report — Policy: REQUIRE_APPROVAL")
    print("-" * 40)
    try:
        result = summarize_report(report_name="Q1 Sales Report", audience="board of directors")
        print(f"  Result: {result}")
        print("  (MockCordumClient auto-approved the request)")
    except CordumBlockedError as e:
        print(f"  BLOCKED: {e}")
    print()

    # --- AutoGen tool integration info ---
    print("=" * 60)
    print("  AutoGen Tool Integration")
    print("=" * 60)
    print(f"  {len([lookup_tool, execute_tool, summarize_tool])} FunctionTools created from @guard-decorated functions")
    print(f"  Tool names: {lookup_tool.name}, {execute_tool.name}, {summarize_tool.name}")
    print(f"  In a real scenario, pass these tools to AssistantAgent with a")
    print(f"  model_client — CAP guards run transparently on every invocation.")
    print()

    # --- Audit trail ---
    print("=" * 60)
    print("  CAP Audit Trail")
    print("=" * 60)
    log_snapshot = list(mock.call_log)
    for i, entry in enumerate(log_snapshot, 1):
        resp = mock._responses.get(entry.capability) or mock._responses.get(entry.topic)
        decision_str = resp.decision.value if resp else mock._default_decision.value
        print(f"  {i}. capability={entry.capability:20s} "
              f"decision={decision_str:20s} "
              f"risk_tags={entry.risk_tags}")

    print()
    print("Done. All three CAP safety decisions demonstrated with AutoGen tools.")


if __name__ == "__main__":
    main()
