"""Static type-contract gate for a downstream CAP SDK consumer."""

from pathlib import Path
import subprocess
import sys


SDK_ROOT = Path(__file__).resolve().parents[1]
CONSUMER = SDK_ROOT / "tests" / "typing" / "consumer.py"


def test_strict_external_consumer_type_checks() -> None:
    """Keep public annotations usable without suppressing or introducing Any."""
    command = [
        sys.executable,
        "-m",
        "mypy",
        "--strict",
        "--follow-imports=silent",
        "--disallow-any-expr",
        "--disallow-any-explicit",
        "--disallow-any-decorated",
        "--disallow-any-unimported",
        "--no-incremental",
        "--python-version=3.9",
        str(CONSUMER.relative_to(SDK_ROOT)),
    ]
    result = subprocess.run(
        command,
        cwd=SDK_ROOT,
        check=False,
        capture_output=True,
        text=True,
    )

    output = result.stdout + result.stderr
    assert result.returncode == 0, output
    assert "Success: no issues found in 1 source file" in output
