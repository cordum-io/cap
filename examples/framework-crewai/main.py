"""CrewAI + CAP Safety Governance Demo

Demonstrates how CAP's @guard decorator adds safety governance to CrewAI tools.
Three tools show the three safety decision types:
  - search_database: ALLOW (executes normally)
  - delete_file: DENY (blocked by policy)
  - send_email: REQUIRE_APPROVAL (approved via mock, then executes)

Uses MockCordumClient — no live infrastructure required.
"""

import os
import sys

# Suppress LiteLLM telemetry/warnings for a clean demo
os.environ.setdefault("LITELLM_LOG", "ERROR")
os.environ.setdefault("OPENAI_API_KEY", "sk-fake-key-for-demo")

from cordum_guard.mock import MockCordumClient
from cordum_guard.types import Decision
from cordum_guard.guard import guard
from cordum_guard.exceptions import CordumBlockedError

from crewai import Agent, Task, Crew
from crewai.tools import tool


# ---------------------------------------------------------------------------
# 1. Configure MockCordumClient with safety policies
# ---------------------------------------------------------------------------

mock = MockCordumClient(default_decision=Decision.ALLOW)

# Destructive operations are blocked
mock.set_policy_response(
    "delete_file",
    Decision.DENY,
    reason="Destructive file operations blocked by safety policy",
)

# External communications require human approval
mock.set_policy_response(
    "send_email",
    Decision.REQUIRE_APPROVAL,
    reason="External communication requires human approval",
)

# search_database uses the default: ALLOW


# ---------------------------------------------------------------------------
# 2. Define tools with CAP safety guards
# ---------------------------------------------------------------------------

@tool("Search Database")
@guard(mock, policy="data_ops", capability="search_database")
def search_database(query: str) -> str:
    """Search the internal database for information."""
    return f"Found 3 results for '{query}': [report-Q1.pdf, metrics-2026.csv, summary.docx]"


@tool("Delete File")
@guard(mock, policy="file_ops", capability="delete_file", risk_tags=["destructive", "write"])
def delete_file(filename: str) -> str:
    """Delete a file from the system."""
    return f"Deleted {filename}"


@tool("Send Email")
@guard(mock, policy="comms", capability="send_email", risk_tags=["external", "communication"])
def send_email(recipient: str, subject: str, body: str) -> str:
    """Send an email to an external recipient."""
    return f"Email sent to {recipient}: {subject}"


# ---------------------------------------------------------------------------
# 3. Create CrewAI agents and tasks
# ---------------------------------------------------------------------------

researcher = Agent(
    role="Research Analyst",
    goal="Find and organize information from internal databases",
    backstory="You are a diligent research analyst who searches databases for relevant data.",
    tools=[search_database],
    verbose=True,
    llm="fake",  # Not calling a real LLM — tools are the focus
)

ops_agent = Agent(
    role="Operations Manager",
    goal="Perform file and communication operations as requested",
    backstory="You handle file management and external communications.",
    tools=[delete_file, send_email],
    verbose=True,
    llm="fake",
)


# ---------------------------------------------------------------------------
# 4. Demo: Call each tool directly to show safety decisions
# ---------------------------------------------------------------------------

def main():
    print("=" * 60)
    print("  CrewAI + CAP Safety Governance Demo")
    print("=" * 60)
    print()

    # --- Tool 1: search_database (ALLOW) ---
    print("[1] search_database — Policy: ALLOW")
    print("-" * 40)
    try:
        result = search_database.run(query="quarterly report")
        print(f"  Result: {result}")
    except CordumBlockedError as e:
        print(f"  BLOCKED: {e}")
    print()

    # --- Tool 2: delete_file (DENY) ---
    print("[2] delete_file — Policy: DENY")
    print("-" * 40)
    try:
        result = delete_file.run(filename="important-data.csv")
        print(f"  Result: {result}")
    except CordumBlockedError as e:
        print(f"  BLOCKED: {e}")
    print()

    # --- Tool 3: send_email (REQUIRE_APPROVAL) ---
    print("[3] send_email — Policy: REQUIRE_APPROVAL")
    print("-" * 40)
    try:
        result = send_email.run(
            recipient="client@example.com",
            subject="Q1 Report",
            body="Please find attached the quarterly report.",
        )
        print(f"  Result: {result}")
        print("  (MockCordumClient auto-approved the request)")
    except CordumBlockedError as e:
        print(f"  BLOCKED: {e}")
    print()

    # --- Audit trail ---
    print("=" * 60)
    print("  CAP Audit Trail")
    print("=" * 60)
    # Snapshot the log to avoid mutation during iteration
    log_snapshot = list(mock.call_log)
    for i, entry in enumerate(log_snapshot, 1):
        # Look up the configured decision without calling evaluate_policy again
        resp = mock._responses.get(entry.capability) or mock._responses.get(entry.topic)
        decision_str = resp.decision.value if resp else mock._default_decision.value
        print(f"  {i}. capability={entry.capability:20s} "
              f"decision={decision_str:20s} "
              f"risk_tags={entry.risk_tags}")

    print()
    print("Done. All three CAP safety decisions demonstrated.")


if __name__ == "__main__":
    main()
