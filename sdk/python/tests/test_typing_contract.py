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
        # Target the running interpreter: the installed dependencies were
        # resolved for it, and pydantic >= 2.14 (Python >= 3.10 only) uses
        # syntax a 3.9 target cannot parse. The 3.9 floor is still checked by
        # the 3.9 matrix entry and the CI/publish typing lanes, which run on 3.9.
        f"--python-version={sys.version_info[0]}.{sys.version_info[1]}",
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
