"""Static contracts for the Python CI gates."""

from pathlib import Path

import pytest
import workflow_support


ROOT = Path(__file__).resolve().parents[3]
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "ci.yml"


def _workflow() -> str:
    return WORKFLOW_PATH.read_text(encoding="utf-8")


def test_job_view_excludes_comments_and_disabled_steps() -> None:
    workflow = """jobs:
  python:
    steps:
      # comment-only-command
      - if: false
        run: disabled-command
      - run: live-command
"""
    commands = workflow_support.run_text(workflow, "python")
    assert "live-command" in commands
    assert "comment-only-command" not in commands
    assert "disabled-command" not in commands


def test_run_view_excludes_shell_comment_bait() -> None:
    workflow = """jobs:
  python:
    steps:
      - run: |
          # comment-only-command
          echo safe # inline-command-bait
      - run: echo "python -m pytest -q sdk/python/tests"
"""
    commands = workflow_support.run_text(workflow, "python")
    assert "echo safe" in commands
    assert "comment-only-command" not in commands
    assert "inline-command-bait" not in commands
    with pytest.raises(AssertionError, match="exactly one"):
        workflow_support.run_step_index(
            workflow, "python", "comment-only-command"
        )
    with pytest.raises(AssertionError, match="exactly one"):
        workflow_support.run_step_index(
            workflow, "python", "python -m pytest -q sdk/python/tests"
        )


@pytest.mark.parametrize("condition", ("false", "${{ matrix.run_job }}"))
def test_job_view_rejects_disabled_or_conditional_jobs(condition: str) -> None:
    workflow = f"""jobs:
  python:
    if: {condition}
    steps:
      - run: required-command
"""
    with pytest.raises(AssertionError, match=r"job python.*condition"):
        workflow_support.run_text(workflow, "python")


def test_job_view_rejects_conditional_steps() -> None:
    workflow = """jobs:
  python:
    steps:
      - if: ${{ matrix.run_step }}
        run: required-command
"""
    with pytest.raises(AssertionError, match=r"step 0.*condition"):
        workflow_support.run_text(workflow, "python")


def test_job_view_excludes_non_executable_step_labels() -> None:
    workflow = """jobs:
  python:
    steps:
      - name: required-command
        run: echo safe
"""
    assert "required-command" not in workflow_support.run_text(workflow, "python")


@pytest.mark.parametrize("value", ("true", "${{ matrix.allow_failure }}"))
def test_job_view_rejects_continue_on_error(
    value: str,
) -> None:
    workflow = f"""jobs:
  python:
    steps:
      - continue-on-error: {value}
        run: required-command
"""
    with pytest.raises(AssertionError, match="continue-on-error"):
        workflow_support.run_text(workflow, "python")


def test_validation_checkouts_disable_persisted_credentials() -> None:
    for name in ("python", "python-nats", "python-codegen", "python-typing", "python-build"):
        workflow_support.require_action_input(
            _workflow(), name, "actions/checkout@", "persist-credentials", "false"
        )


def test_action_input_check_cannot_borrow_unrelated_false_values() -> None:
    workflow = """jobs:
  python:
    strategy:
      fail-fast: false
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: true
"""
    with pytest.raises(AssertionError, match="persist-credentials"):
        workflow_support.require_action_input(
            workflow, "python", "actions/checkout@", "persist-credentials", "false"
        )


def test_job_config_check_cannot_borrow_step_text() -> None:
    workflow = """jobs:
  python:
    steps:
      - run: "echo 'id-token: write'"
"""
    with pytest.raises(AssertionError, match="permissions"):
        workflow_support.job_value(workflow, "python", "permissions", "id-token")


def test_step_order_ignores_non_run_and_non_action_bait() -> None:
    workflow = """jobs:
  python:
    steps:
      - env:
          BAIT: validate-release
        run: echo safe
      - uses: actions/upload-artifact@v4
      - run: validate-release
"""
    assert workflow_support.action_step_index(
        workflow, "python", "actions/upload-artifact@"
    ) == 1
    assert workflow_support.run_step_index(
        workflow, "python", "validate-release"
    ) == 2


def test_python_matrix_covers_every_declared_version() -> None:
    workflow = _workflow()
    assert workflow_support.job_value(
        workflow, "python", "strategy", "matrix", "python-version"
    ) == [
        "3.9",
        "3.10",
        "3.11",
        "3.12",
        "3.13",
        "3.14",
    ]
    workflow_support.require_action_input(
        workflow,
        "python",
        "actions/setup-python@",
        "python-version",
        "${{ matrix.python-version }}",
    )


def test_python_matrix_uses_pytest_and_exact_artifact_verifier() -> None:
    workflow = _workflow()
    tests = workflow_support.run_step_text(
        workflow, "python", "python -m pytest -q sdk/python/tests"
    )
    workflow_support.run_step_index(workflow, "python", "python -m build --outdir")
    verifier = workflow_support.run_step_text(
        workflow, "python", "python sdk/python/scripts/verify_artifacts.py"
    )
    assert "--ignore=sdk/python/tests/integration" in tests
    assert "unittest discover" not in tests
    assert all(token in verifier for token in ("--wheel", "--sdist", "--python python"))


def test_real_nats_lane_is_pinned_mandatory_and_explicit() -> None:
    workflow = _workflow()
    commands = workflow_support.run_text(workflow, "python-nats")
    assert workflow_support.job_value(
        workflow, "python-nats", "services", "nats", "image"
    ) == "nats:2.10.29-alpine"
    assert workflow_support.job_value(
        workflow, "python-nats", "timeout-minutes"
    ) == "10"
    assert workflow_support.job_value(
        workflow, "python-nats", "env", "CAP_TEST_NATS_URL"
    ) == "nats://127.0.0.1:4222"
    workflow_support.run_step_index(
        workflow, "python-nats",
        "python -m pytest -q sdk/python/tests/integration/test_worker_nats.py",
    )
    assert "|| true" not in commands


def test_codegen_lane_installs_pins_and_checks_without_mutation() -> None:
    workflow = _workflow()
    install = workflow_support.run_step_text(
        workflow, "python-codegen", "python -m pip install --upgrade pip"
    )
    workflow_support.run_step_index(
        workflow, "python-codegen", "python sdk/python/scripts/generate_protos.py --check"
    )
    assert "-r sdk/python/requirements-codegen.txt" in install
    commands = workflow_support.run_text(workflow, "python-codegen")
    assert "make_protos" not in commands


def test_typing_lane_checks_strict_external_consumer() -> None:
    commands = workflow_support.run_step_text(
        _workflow(), "python-typing", "python -m mypy --strict"
    )
    assert "sdk/python/tests/typing/consumer.py" in commands
    for flag in (
        "--follow-imports=silent",
        "--disallow-any-expr",
        "--disallow-any-explicit",
        "--disallow-any-decorated",
        "--disallow-any-unimported",
        "--no-incremental",
        "--python-version=3.9",
    ):
        assert flag in commands


def test_build_lane_builds_once_and_verifies_exact_pair() -> None:
    workflow = _workflow()
    build = workflow_support.run_step_text(
        workflow, "python-build", "python -m build --outdir"
    )
    workflow_support.run_step_index(workflow, "python-build", "python -m twine check")
    verifier = workflow_support.run_step_text(
        workflow, "python-build", "python sdk/python/scripts/verify_artifacts.py"
    )
    assert build.count("python -m build") == 1
    assert "--wheel" in verifier and "--sdist" in verifier
