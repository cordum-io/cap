from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Optional

import pytest


SDK_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = SDK_ROOT / "scripts" / "validate_release.py"
PYPROJECT = SDK_ROOT / "pyproject.toml"


def run_validator(
    tag: Optional[str],
    pyproject: Optional[Path] = None,
    artifact_report: Optional[Path] = None,
) -> subprocess.CompletedProcess[str]:
    command = [sys.executable, str(SCRIPT)]
    if tag is not None:
        command.extend(["--tag", tag])
    if pyproject is not None:
        command.extend(["--pyproject", str(pyproject)])
    if artifact_report is not None:
        command.extend(["--artifact-report", str(artifact_report)])
    return subprocess.run(command, capture_output=True, check=False, text=True)


def write_pyproject(
    path: Path,
    body: str = '[project]\nname = "cap-sdk-python"\nversion = "2.5.3"\n',
) -> Path:
    path.write_text(body, encoding="utf-8")
    return path


def write_report(
    path: Path,
    name: str = "cap-sdk-python",
    version: str = "2.5.3",
    schema: str = "cap-python-artifact-verification/v1",
) -> Path:
    report = {"distribution": {"name": name, "version": version}, "schema": schema}
    path.write_text(json.dumps(report), encoding="utf-8")
    return path


def assert_failed(result: subprocess.CompletedProcess[str], message: str) -> None:
    assert result.returncode == 1
    assert result.stdout == ""
    assert result.stderr == f"release validation failed: {message}\n"


def test_current_version_tag_succeeds_with_default_pyproject() -> None:
    result = run_validator("v2.5.3")

    assert result.returncode == 0
    assert result.stderr == ""
    assert result.stdout == '{"tag":"v2.5.3","version":"2.5.3"}\n'
    assert json.loads(result.stdout) == {"tag": "v2.5.3", "version": "2.5.3"}


def test_custom_pyproject_path_succeeds(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")

    result = run_validator("v2.5.3", pyproject)

    assert result.returncode == 0
    assert result.stderr == ""


def test_mismatched_tag_fails_closed(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")

    result = run_validator("v2.5.4", pyproject)

    assert_failed(result, "tag does not match project version")


@pytest.mark.parametrize("tag", ["V2.5.3", "2.5.3", "refs/tags/v2.5.3"])
def test_tag_variants_are_rejected(tmp_path: Path, tag: str) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")

    result = run_validator(tag, pyproject)

    assert result.returncode == 1
    assert result.stdout == ""
    assert result.stderr.startswith("release validation failed: ")


def test_empty_tag_is_rejected_as_invalid_ref(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")

    result = run_validator("", pyproject)

    assert_failed(result, "tag is not a valid release ref name")


def test_missing_tag_argument_fails_with_exit_one() -> None:
    result = run_validator(None)

    assert result.returncode == 1
    assert result.stdout == ""
    assert result.stderr.startswith("release validation failed: invalid arguments:")


def test_malformed_project_version_is_rejected(tmp_path: Path) -> None:
    body = '[project]\nname = "cap-sdk-python"\nversion = "not a version"\n'
    pyproject = write_pyproject(tmp_path / "pyproject.toml", body)

    result = run_validator("vnot a version", pyproject)

    assert_failed(result, "project version is not valid PEP 440")


def test_noncanonical_project_version_is_rejected(tmp_path: Path) -> None:
    body = '[project]\nname = "cap-sdk-python"\nversion = "2.5.3-rc1"\n'
    pyproject = write_pyproject(tmp_path / "pyproject.toml", body)

    result = run_validator("v2.5.3-rc1", pyproject)

    assert_failed(result, "project version is not canonical PEP 440")


def test_local_project_version_is_rejected_for_public_release(tmp_path: Path) -> None:
    body = '[project]\nname = "cap-sdk-python"\nversion = "2.5.3+private"\n'
    pyproject = write_pyproject(tmp_path / "pyproject.toml", body)

    result = run_validator("v2.5.3+private", pyproject)

    assert_failed(result, "project version contains a local identifier")


@pytest.mark.parametrize(
    ("body", "message"),
    [
        ('[tool.setuptools]\n', "project table is missing"),
        ('[project]\nname = "cap-sdk-python"\n', "project version is missing"),
        ('[project]\nname = "cap-sdk-python"\nversion = 253\n', "project version must be a string"),
    ],
)
def test_missing_or_invalid_project_metadata_fails(tmp_path: Path, body: str, message: str) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml", body)

    result = run_validator("v2.5.3", pyproject)

    assert_failed(result, message)


def test_missing_pyproject_fails(tmp_path: Path) -> None:
    missing = tmp_path / "missing.toml"

    result = run_validator("v2.5.3", missing)

    assert_failed(result, "pyproject file cannot be read")


def test_malformed_toml_fails_without_traceback(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml", '[project\nname = "cap-sdk-python"\n')

    result = run_validator("v2.5.3", pyproject)

    assert_failed(result, "pyproject file is not valid TOML")


def test_validation_does_not_mutate_pyproject(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")
    before = (pyproject.read_bytes(), pyproject.stat().st_mtime_ns)

    result = run_validator("v2.5.3", pyproject)

    assert result.returncode == 0
    assert (pyproject.read_bytes(), pyproject.stat().st_mtime_ns) == before


@pytest.mark.parametrize("name", ["different-package", "Cap-SDK-Python", ""])
def test_project_name_must_match_distribution(tmp_path: Path, name: str) -> None:
    body = f'[project]\nname = "{name}"\nversion = "2.5.3"\n'
    pyproject = write_pyproject(tmp_path / "pyproject.toml", body)

    result = run_validator("v2.5.3", pyproject)

    assert_failed(result, "project name must be cap-sdk-python")


def test_matching_artifact_report_succeeds(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")
    report = write_report(tmp_path / "report.json")
    before = (pyproject.read_bytes(), report.read_bytes())

    result = run_validator("v2.5.3", pyproject, report)

    assert result.returncode == 0
    assert result.stdout == '{"tag":"v2.5.3","version":"2.5.3"}\n'
    assert result.stderr == ""
    assert (pyproject.read_bytes(), report.read_bytes()) == before


@pytest.mark.parametrize(
    ("name", "version", "message"),
    [
        ("other-package", "2.5.3", "artifact report distribution name mismatch"),
        ("cap-sdk-python", "9.9.9", "artifact report distribution version mismatch"),
    ],
)
def test_artifact_report_distribution_must_match(
    tmp_path: Path, name: str, version: str, message: str
) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")
    report = write_report(tmp_path / "report.json", name=name, version=version)

    result = run_validator("v2.5.3", pyproject, report)

    assert_failed(result, message)


def test_artifact_report_schema_must_match(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")
    report = write_report(tmp_path / "report.json", schema="cap-python-artifact-verification/v2")

    result = run_validator("v2.5.3", pyproject, report)

    assert_failed(result, "artifact report schema mismatch")


def test_artifact_report_distribution_is_required(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")
    report = tmp_path / "report.json"
    report.write_text('{"schema":"cap-python-artifact-verification/v1"}', encoding="utf-8")

    result = run_validator("v2.5.3", pyproject, report)

    assert_failed(result, "artifact report distribution is invalid")


def test_malformed_artifact_report_fails(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")
    report = tmp_path / "report.json"
    report.write_text("not-json", encoding="utf-8")

    result = run_validator("v2.5.3", pyproject, report)

    assert_failed(result, "artifact report is not valid JSON")


def test_artifact_report_must_be_json_object(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")
    report = tmp_path / "report.json"
    report.write_text("[]", encoding="utf-8")

    result = run_validator("v2.5.3", pyproject, report)

    assert_failed(result, "artifact report must be a JSON object")


def test_missing_artifact_report_fails(tmp_path: Path) -> None:
    pyproject = write_pyproject(tmp_path / "pyproject.toml")

    result = run_validator("v2.5.3", pyproject, tmp_path / "missing.json")

    assert_failed(result, "artifact report file cannot be read")
