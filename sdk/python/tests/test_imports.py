"""Compatibility contract tests for Python SDK metadata and generated code."""

from __future__ import annotations

import ast
import re
from functools import lru_cache
from typing import Dict, Iterable, Set

from packaging.requirements import Requirement
from packaging.version import Version

try:
    import tomllib
except ModuleNotFoundError:  # pragma: no cover - exercised on Python 3.9/3.10
    import tomli as tomllib

from artifact_support import REPO_ROOT, SDK_ROOT


GENERATED_ROOT = SDK_ROOT / "cap" / "pb" / "cordum" / "agent" / "v1"
PROTOBUF_VERSION = re.compile(r"Protobuf Python Version: ([^\s]+)")
GRPC_VERSION = re.compile(r"GRPC_GENERATED_VERSION = ['\"]([^'\"]+)['\"]")

EXPECTED_CLASSIFIERS = {
    f"Programming Language :: Python :: 3.{minor}"
    for minor in range(9, 15)
}


@lru_cache(maxsize=1)
def pyproject_data() -> Dict[str, object]:
    with (SDK_ROOT / "pyproject.toml").open("rb") as handle:
        return tomllib.load(handle)


def project_metadata() -> Dict[str, object]:
    return pyproject_data()["project"]


def development_requirements() -> Set[str]:
    optional = project_metadata().get("optional-dependencies", {})
    raw_requirements = optional.get("dev", [])
    return {Requirement(raw).name for raw in raw_requirements}


def dependency(name: str) -> Requirement:
    raw_dependencies = project_metadata().get("dependencies", [])
    requirements = [Requirement(raw) for raw in raw_dependencies]
    matches = [requirement for requirement in requirements if requirement.name == name]
    assert len(matches) == 1, f"expected one {name} dependency, found: {matches}"
    return matches[0]


def generated_versions(pattern: str, expression: re.Pattern[str]) -> Set[Version]:
    versions: Set[Version] = set()
    paths = sorted(GENERATED_ROOT.glob(pattern))
    assert paths, f"no generated files matched {pattern}"
    for path in paths:
        match = expression.search(path.read_text(encoding="utf-8"))
        assert match is not None, f"missing version header: {path.name}"
        versions.add(Version(match.group(1)))
    return versions


def declared_floor(requirement: Requirement) -> Version:
    lower_operators = {">", ">=", "==", "~="}
    candidates = [
        Version(specifier.version)
        for specifier in requirement.specifier
        if specifier.operator in lower_operators and "*" not in specifier.version
    ]
    assert candidates, f"{requirement.name} must declare an explicit lower bound"
    return max(candidates)


def only_version(versions: Iterable[Version], tool: str) -> Version:
    values = set(versions)
    assert len(values) == 1, f"mixed {tool} generated versions: {sorted(values)}"
    return next(iter(values))


def test_generated_headers_use_one_version_per_tool() -> None:
    protobuf = generated_versions("*_pb2.py", PROTOBUF_VERSION)
    grpc = generated_versions("*_pb2_grpc.py", GRPC_VERSION)
    assert len(protobuf) == 1, f"mixed protobuf versions: {sorted(protobuf)}"
    assert len(grpc) == 1, f"mixed grpc versions: {sorted(grpc)}"


def test_build_backend_is_pinned_for_python_39() -> None:
    build_system = pyproject_data()["build-system"]
    assert set(build_system["requires"]) == {
        "setuptools==82.0.1",
        "wheel==0.47.0",
    }
    assert build_system["build-backend"] == "setuptools.build_meta"


def test_package_discovery_is_limited_to_cap() -> None:
    find = pyproject_data()["tool"]["setuptools"]["packages"]["find"]
    assert find["include"] == ["cap", "cap.*"]
    assert find["namespaces"] is False


def test_sources_parse_as_python_39() -> None:
    sources = sorted((SDK_ROOT / "cap").rglob("*.py"))
    assert sources, "no Python SDK sources found"
    for path in sources:
        ast.parse(path.read_text(encoding="utf-8"), path.name, feature_version=(3, 9))


def test_protobuf_floor_matches_generated_runtime() -> None:
    requirement = dependency("protobuf")
    generated = only_version(
        generated_versions("*_pb2.py", PROTOBUF_VERSION),
        "protobuf",
    )
    floor = declared_floor(requirement)
    assert floor >= generated, f"protobuf floor {floor} is older than generated {generated}"
    assert not requirement.specifier.contains("7.0.0"), requirement


def test_grpcio_floor_matches_generated_runtime() -> None:
    requirement = dependency("grpcio")
    generated = only_version(
        generated_versions("*_pb2_grpc.py", GRPC_VERSION),
        "grpc",
    )
    floor = declared_floor(requirement)
    assert floor >= generated, f"grpcio floor {floor} is older than generated {generated}"
    assert not requirement.specifier.contains("2.0.0"), requirement


def test_declares_verified_python_classifiers() -> None:
    classifiers = set(project_metadata().get("classifiers", []))
    missing = EXPECTED_CLASSIFIERS - classifiers
    assert not missing, f"missing Python classifiers: {sorted(missing)}"


def test_dev_extra_contains_release_and_typing_gates() -> None:
    required = {"build", "mypy", "packaging", "pdoc", "pytest", "tomli", "twine"}
    missing = required - development_requirements()
    assert not missing, f"dev extra is missing: {sorted(missing)}"


def test_package_init_does_not_install_protobuf_validation_shim() -> None:
    source = (SDK_ROOT / "cap" / "__init__.py").read_text(encoding="utf-8")
    assert "SimpleNamespace" not in source
    assert 'sys.modules["google.protobuf.runtime_version"]' not in source


def test_package_license_matches_repository_license() -> None:
    package_license = SDK_ROOT / "LICENSE"
    assert package_license.is_file(), "sdk/python/LICENSE is missing"
    assert package_license.read_bytes() == (REPO_ROOT / "LICENSE").read_bytes()


def test_typing_marker_exists() -> None:
    marker = SDK_ROOT / "cap" / "py.typed"
    assert marker.is_file(), "sdk/python/cap/py.typed is missing"
