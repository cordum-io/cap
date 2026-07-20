"""Static contracts for deterministic, local-only playground orchestration."""

from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
PLAYGROUND = ROOT / "playground"
CI = ROOT / ".github" / "workflows" / "ci.yml"
PLAYGROUND_WORKFLOW = ROOT / ".github" / "workflows" / "playground.yml"


def _compose() -> dict[str, object]:
    document = yaml.safe_load((PLAYGROUND / "docker-compose.yml").read_text())
    assert isinstance(document, dict)
    return document


def test_nats_is_pinned_healthy_and_not_host_exposed() -> None:
    services = _compose()["services"]
    assert isinstance(services, dict)
    nats = services["nats"]
    assert nats["image"].startswith("nats:2.12.6-alpine@sha256:")
    assert nats["command"] == ["-m", "8222"]
    assert "ports" not in nats
    assert "http://127.0.0.1:8222/healthz" in " ".join(nats["healthcheck"]["test"])


def test_dependencies_and_timeouts_are_fail_closed() -> None:
    services = _compose()["services"]
    worker = services["echo-worker"]
    submit = services["submit"]
    assert worker["depends_on"]["nats"]["condition"] == "service_healthy"
    assert "restart" not in worker
    assert submit["depends_on"]["nats"]["condition"] == "service_healthy"
    assert submit["depends_on"]["echo-worker"]["condition"] == "service_started"
    assert submit["environment"] == {
        "NATS_URL": "nats://nats:4222",
        "NATS_MONITOR_URL": "http://nats:8222",
        "CAP_NATS_CONNECT_TIMEOUT_SECONDS": "5",
        "CAP_WORKER_READY_TIMEOUT_SECONDS": "20",
        "CAP_RESULT_TIMEOUT_SECONDS": "15",
        "CAP_CLEANUP_TIMEOUT_SECONDS": "2",
        "PYTHONUNBUFFERED": "1",
    }


def test_python_images_and_build_contexts_are_immutable_and_minimal() -> None:
    for service in ("submit", "worker"):
        directory = PLAYGROUND / service
        first_line = (directory / "Dockerfile").read_text().splitlines()[0]
        assert first_line.startswith("FROM python:3.11.15-slim-bookworm@sha256:")
        patterns = (directory / "Dockerfile.dockerignore").read_text().splitlines()
        assert patterns[0] == "**"
        assert "!sdk/python/**" in patterns
        assert f"!playground/{service}/**" in patterns
        assert "**/*.egg-info/" in patterns
        assert "**/.pytest_cache/" in patterns
        assert "**/.mypy_cache/" in patterns
        assert "**/build/" in patterns
        assert "**/dist/" in patterns


def test_readme_preserves_submit_verdict_and_guarantees_cleanup() -> None:
    readme = (PLAYGROUND / "README.md").read_text()
    command = "docker compose up --build --abort-on-container-exit --exit-code-from submit"
    assert command in readme
    assert "trap cleanup EXIT INT TERM" in readme
    assert "bounded_compose_down()" in readme
    assert 'kill -KILL "$down_pid"' in readme
    assert "docker compose down -v --remove-orphans --timeout 5" in readme
    assert "Development-only" in readme


def test_host_broker_docs_bind_loopback_and_parse_real_subsz_shape() -> None:
    guide = (ROOT / "docs" / "getting-started.md").read_text()
    troubleshooting = (ROOT / "docs" / "troubleshooting.md").read_text()
    assert "-p 127.0.0.1:4222:4222" in guide
    assert "-p 127.0.0.1:8222:8222" in guide
    assert "-p 127.0.0.1:4222:4222" in troubleshooting
    assert "-p 127.0.0.1:8222:8222" in troubleshooting
    assert "s.get('subject') == 'job.echo'" in troubleshooting
    assert "'job.echo' in d.get('subscriptions_list', [])" not in troubleshooting


def _workflow_text() -> str:
    if not PLAYGROUND_WORKFLOW.is_file():
        return ""
    return PLAYGROUND_WORKFLOW.read_text(encoding="utf-8")


def test_required_ci_calls_bounded_playground_gate() -> None:
    ci = CI.read_text(encoding="utf-8")
    ci_document = yaml.safe_load(ci)
    workflow = _workflow_text()
    assert "uses: ./.github/workflows/playground.yml" in ci
    assert ci_document["jobs"]["playground"]["if"] == "${{ always() }}"
    assert "workflow_call:" in workflow
    assert "timeout-minutes:" in workflow
    assert "continue-on-error" not in workflow


def test_playground_ci_runs_unit_and_failed_result_contracts() -> None:
    workflow = _workflow_text()
    submit_tests = (PLAYGROUND / "tests" / "test_submit.py").read_text()
    assert "python -m pytest -q playground/tests" in workflow
    assert "test_non_success_status_returns_expected_code" in submit_tests
    assert "job_pb2.JOB_STATUS_FAILED, EXIT_TERMINAL" in submit_tests


def test_playground_ci_no_worker_run_is_real_and_fail_closed() -> None:
    workflow = _workflow_text()
    assert "docker compose up -d --wait --wait-timeout 30 nats" in workflow
    assert "docker compose run --rm --no-deps" in workflow
    assert "CAP_WORKER_READY_TIMEOUT_SECONDS=1" in workflow
    assert 'test "$no_worker_status" -ne 0' in workflow
    assert 'test "$no_worker_status" -eq 3' in workflow
    assert '! grep -q "CAP Playground Demo Complete!"' in workflow


def test_playground_ci_repeats_exact_authoritative_command() -> None:
    workflow = _workflow_text()
    exact = "docker compose up --build --abort-on-container-exit --exit-code-from submit"
    assert "for attempt in 1 2 3" in workflow
    assert exact in workflow
    assert 'test "$compose_status" -eq 0' in workflow
    assert 'test "$marker_count" -eq 1' in workflow


def test_playground_ci_always_captures_logs_and_cleans_state() -> None:
    workflow = _workflow_text()
    document = yaml.safe_load(workflow)
    assert "PLAYGROUND_LOGS: ${{ runner.temp }}" not in workflow
    assert workflow.count('PLAYGROUND_LOGS="$RUNNER_TEMP/cap-playground-logs"') == 4
    assert document["jobs"]["compose"]["if"] == "${{ always() }}"
    assert workflow.count("timeout 20s docker compose logs --no-color") == 4
    assert "docker compose logs --no-color" in workflow
    assert "docker compose down -v --remove-orphans --timeout 5" in workflow
    assert "trap cleanup EXIT INT TERM" in workflow
    assert "if: ${{ failure() || cancelled() }}" in workflow
    assert "actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02" in workflow
    assert "name: playground-${{ github.run_id }}-${{ github.run_attempt }}" in workflow
    assert "sleep " not in workflow


def test_playground_ci_pins_bootstrap_actions() -> None:
    workflow = _workflow_text()
    assert "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683" in workflow
    assert "actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065" in workflow


def test_final_cleanup_survives_missing_checkout() -> None:
    document = yaml.safe_load(_workflow_text())
    steps = document["jobs"]["compose"]["steps"]
    cleanup = next(step for step in steps if step.get("name", "").startswith("Always"))
    assert "working-directory" not in cleanup
    assert "if [[ -f playground/docker-compose.yml ]]; then" in cleanup["run"]
    assert "cd playground" in cleanup["run"]
    assert "checkout/playground unavailable" in cleanup["run"]
