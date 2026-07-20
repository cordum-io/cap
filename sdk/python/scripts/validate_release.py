#!/usr/bin/env python3
"""Fail closed unless a release tag exactly matches canonical package metadata."""

from __future__ import annotations

import argparse
import importlib
import json
import re
import sys
from pathlib import Path
from typing import Callable, Mapping, NoReturn, Optional, Sequence, cast

from packaging.version import InvalidVersion, Version


DEFAULT_PYPROJECT = Path(__file__).resolve().parents[1] / "pyproject.toml"
TAG_PATTERN = re.compile(r"[A-Za-z0-9][A-Za-z0-9.!+_-]*\Z")
EXPECTED_DISTRIBUTION = "cap-sdk-python"
ARTIFACT_REPORT_SCHEMA = "cap-python-artifact-verification/v1"
TomlLoader = Callable[[str], Mapping[str, object]]
TOML_MODULE = importlib.import_module("tomllib" if sys.version_info >= (3, 11) else "tomli")
LOAD_TOML = cast(TomlLoader, getattr(TOML_MODULE, "loads"))


class ValidationError(ValueError):
    """An expected release-input validation failure."""


class FailClosedArgumentParser(argparse.ArgumentParser):
    """Translate invalid CLI input into the validator's exit-one contract."""

    def error(self, message: str) -> NoReturn:
        raise ValidationError(f"invalid arguments: {message}")


def parse_project(path: Path) -> dict[str, object]:
    try:
        content = path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        raise ValidationError("pyproject file cannot be read") from exc
    try:
        parsed = LOAD_TOML(content)
    except ValueError as exc:
        raise ValidationError("pyproject file is not valid TOML") from exc
    return dict(parsed)


def canonical_project_version(path: Path) -> str:
    metadata = parse_project(path)
    project = metadata.get("project")
    if not isinstance(project, dict):
        raise ValidationError("project table is missing")
    if project.get("name") != EXPECTED_DISTRIBUTION:
        raise ValidationError(f"project name must be {EXPECTED_DISTRIBUTION}")
    if "version" not in project:
        raise ValidationError("project version is missing")
    version = project["version"]
    if not isinstance(version, str):
        raise ValidationError("project version must be a string")
    try:
        parsed = Version(version)
    except InvalidVersion as exc:
        raise ValidationError("project version is not valid PEP 440") from exc
    if str(parsed) != version:
        raise ValidationError("project version is not canonical PEP 440")
    if parsed.local is not None:
        raise ValidationError("project version contains a local identifier")
    return version


def validate_tag(tag: str, version: str) -> None:
    if not tag or tag != tag.strip() or TAG_PATTERN.fullmatch(tag) is None:
        raise ValidationError("tag is not a valid release ref name")
    if tag != f"v{version}":
        raise ValidationError("tag does not match project version")


def read_artifact_report(path: Path) -> dict[str, object]:
    try:
        content = path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        raise ValidationError("artifact report file cannot be read") from exc
    try:
        report = json.loads(content)
    except json.JSONDecodeError as exc:
        raise ValidationError("artifact report is not valid JSON") from exc
    if not isinstance(report, dict):
        raise ValidationError("artifact report must be a JSON object")
    return report


def validate_artifact_report(path: Path, version: str) -> None:
    report = read_artifact_report(path)
    if report.get("schema") != ARTIFACT_REPORT_SCHEMA:
        raise ValidationError("artifact report schema mismatch")
    distribution = report.get("distribution")
    if not isinstance(distribution, dict):
        raise ValidationError("artifact report distribution is invalid")
    if distribution.get("name") != EXPECTED_DISTRIBUTION:
        raise ValidationError("artifact report distribution name mismatch")
    if distribution.get("version") != version:
        raise ValidationError("artifact report distribution version mismatch")


def validate_release(
    tag: str, pyproject: Path, artifact_report: Optional[Path] = None
) -> dict[str, str]:
    version = canonical_project_version(pyproject)
    validate_tag(tag, version)
    if artifact_report is not None:
        validate_artifact_report(artifact_report, version)
    return {"tag": tag, "version": version}


def build_parser() -> argparse.ArgumentParser:
    parser = FailClosedArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True, help="exact Git release tag")
    parser.add_argument(
        "--pyproject",
        type=Path,
        default=DEFAULT_PYPROJECT,
        help="package pyproject.toml (defaults to the SDK metadata)",
    )
    parser.add_argument(
        "--artifact-report",
        type=Path,
        help="optional JSON report emitted by verify_artifacts.py",
    )
    return parser


def main(argv: Optional[Sequence[str]] = None) -> int:
    try:
        args = build_parser().parse_args(argv)
        result = validate_release(args.tag, args.pyproject, args.artifact_report)
    except ValidationError as exc:
        print(f"release validation failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
