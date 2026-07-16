"""Installed wheel and sdist contract tests for the Python SDK."""

from __future__ import annotations

import json
from pathlib import Path, PurePosixPath
from typing import Mapping

import pytest

from artifact_support import (
    BuiltArtifacts,
    DEPENDENCY_FLOORS,
    SDK_ROOT,
    archive_entries,
    build_distributions,
    expected_generated_paths,
    expected_import_names,
    smoke_installed_artifact,
    wheel_metadata,
)


@pytest.fixture(scope="session")
def built_artifacts(tmp_path_factory: pytest.TempPathFactory) -> BuiltArtifacts:
    workspace = tmp_path_factory.mktemp("python-sdk-artifacts")
    return build_distributions(workspace)


def artifact_path(artifacts: BuiltArtifacts, kind: str) -> Path:
    return artifacts.wheel if kind == "wheel" else artifacts.sdist


def artifact_entries(artifacts: BuiltArtifacts, kind: str) -> Mapping[str, bytes]:
    return archive_entries(artifact_path(artifacts, kind))


@pytest.mark.parametrize("kind", ("wheel", "sdist"))
def test_generated_modules_match_canonical_protos(
    built_artifacts: BuiltArtifacts,
    kind: str,
) -> None:
    names = set(artifact_entries(built_artifacts, kind))
    actual = {
        name
        for name in names
        if name.endswith("_pb2.py") or name.endswith("_pb2_grpc.py")
    }
    assert actual == expected_generated_paths()


@pytest.mark.parametrize("kind", ("wheel", "sdist"))
def test_artifact_includes_typing_marker(
    built_artifacts: BuiltArtifacts,
    kind: str,
) -> None:
    names = set(artifact_entries(built_artifacts, kind))
    assert "cap/py.typed" in names, f"{kind} is missing cap/py.typed"


@pytest.mark.parametrize("kind", ("wheel", "sdist"))
def test_artifact_includes_license_text(
    built_artifacts: BuiltArtifacts,
    kind: str,
) -> None:
    entries = artifact_entries(built_artifacts, kind)
    package_license = SDK_ROOT / "LICENSE"
    assert package_license.is_file(), "sdk/python/LICENSE is missing"
    matches = [
        content
        for name, content in entries.items()
        if PurePosixPath(name).name == "LICENSE"
    ]
    assert len(matches) == 1, f"{kind} LICENSE entries: {len(matches)}"
    assert matches[0] == package_license.read_bytes()


def test_sdist_includes_build_inputs(built_artifacts: BuiltArtifacts) -> None:
    names = set(archive_entries(built_artifacts.sdist))
    assert {"README.md", "pyproject.toml"} <= names


def test_wheel_metadata_contains_readme(built_artifacts: BuiltArtifacts) -> None:
    metadata = wheel_metadata(archive_entries(built_artifacts.wheel)).replace(
        "\r\n",
        "\n",
    )
    readme = (SDK_ROOT / "README.md").read_text(encoding="utf-8").strip()
    assert "Description-Content-Type: text/markdown" in metadata
    assert readme in metadata


def test_wheel_metadata_declares_license_file(built_artifacts: BuiltArtifacts) -> None:
    metadata = wheel_metadata(archive_entries(built_artifacts.wheel))
    assert "License-Expression: Apache-2.0" in metadata
    assert "License-File: LICENSE" in metadata


def test_wheel_excludes_source_artifacts(built_artifacts: BuiltArtifacts) -> None:
    names = set(archive_entries(built_artifacts.wheel))
    forbidden = [
        name
        for name in names
        if name.startswith(("build/", "dist/", "tests/"))
        or "/__pycache__/" in f"/{name}"
        or name.endswith(".pyc")
    ]
    assert forbidden == []


@pytest.mark.parametrize("kind", ("wheel", "sdist"))
def test_clean_installed_artifact_imports_all_generated_modules(
    built_artifacts: BuiltArtifacts,
    kind: str,
    tmp_path: Path,
) -> None:
    result = smoke_installed_artifact(
        artifact_path(built_artifacts, kind),
        tmp_path / f"consumer-{kind}",
    )
    assert result.returncode == 0, (
        f"{kind} import smoke failed ({result.returncode})\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )
    assert f'"imports": {len(expected_import_names())}' in result.stdout
    evidence = json.loads(result.stdout)
    assert evidence["protobuf_version"] == DEPENDENCY_FLOORS["protobuf"]
    assert evidence["grpcio_version"] == DEPENDENCY_FLOORS["grpcio"]
