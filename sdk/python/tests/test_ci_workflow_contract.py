"""Static contracts for the Python CI gates."""

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[3]
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "ci.yml"


def _workflow() -> str:
    return WORKFLOW_PATH.read_text(encoding="utf-8")


def _job(name: str) -> str:
    match = re.search(
        rf"(?ms)^  {re.escape(name)}:\n(?P<body>.*?)(?=^  [a-zA-Z0-9_-]+:\n|\Z)",
        _workflow(),
    )
    assert match is not None, f"missing CI job: {name}"
    return match.group(0)


def test_python_matrix_covers_every_declared_version() -> None:
    job = _job("python")
    match = re.search(r"python-version:\s*\[([^]]+)]", job)
    assert match is not None, "python job must use an explicit version matrix"
    assert re.findall(r"\d+\.\d+", match.group(1)) == [
        "3.9",
        "3.10",
        "3.11",
        "3.12",
        "3.13",
        "3.14",
    ]
    assert "python-version: ${{ matrix.python-version }}" in job


def test_python_matrix_uses_pytest_and_exact_artifact_verifier() -> None:
    job = _job("python")
    assert "python -m pytest -q sdk/python/tests" in job
    assert "--ignore=sdk/python/tests/integration" in job
    assert "unittest discover" not in _workflow()
    assert "python -m build --outdir" in job
    assert "sdk/python/scripts/verify_artifacts.py" in job
    assert "--wheel" in job and "--sdist" in job and "--python python" in job


def test_real_nats_lane_is_pinned_mandatory_and_explicit() -> None:
    job = _job("python-nats")
    assert "image: nats:2.10.29-alpine" in job
    assert "timeout-minutes: 10" in job
    assert "CAP_TEST_NATS_URL: nats://127.0.0.1:4222" in job
    assert "sdk/python/tests/integration/test_worker_nats.py" in job
    assert "continue-on-error" not in job
    assert "|| true" not in job


def test_codegen_lane_installs_pins_and_checks_without_mutation() -> None:
    job = _job("python-codegen")
    assert "-r sdk/python/requirements-codegen.txt" in job
    assert "python sdk/python/scripts/generate_protos.py --check" in job
    assert "make_protos" not in job


def test_typing_lane_checks_strict_external_consumer() -> None:
    job = _job("python-typing")
    assert "python -m mypy --strict" in job
    assert "sdk/python/tests/typing/consumer.py" in job
    for flag in (
        "--follow-imports=silent",
        "--disallow-any-expr",
        "--disallow-any-explicit",
        "--disallow-any-decorated",
        "--disallow-any-unimported",
        "--no-incremental",
        "--python-version=3.9",
    ):
        assert flag in job


def test_build_lane_builds_once_and_verifies_exact_pair() -> None:
    job = _job("python-build")
    assert job.count("python -m build") == 1
    assert "python -m twine check" in job
    assert "sdk/python/scripts/verify_artifacts.py" in job
    assert "--wheel" in job and "--sdist" in job
