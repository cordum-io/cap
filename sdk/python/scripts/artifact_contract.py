"""Immutable source-byte and archive-inventory contract for CAP artifacts."""

from __future__ import annotations

import re
import sys
from email import policy
from email.parser import Parser
from pathlib import Path, PurePosixPath
from typing import Callable, Mapping, Sequence, Set, Tuple, cast

from packaging.requirements import Requirement

if sys.version_info >= (3, 11):
    from tomllib import loads as load_toml
else:  # pragma: no cover - exercised by the Python 3.9 gate
    from tomli import loads as load_toml


Require = Callable[[bool, str], None]
WHEEL_METADATA = ("METADATA", "RECORD", "WHEEL", "top_level.txt", "licenses/LICENSE")
SDIST_EGG_INFO = ("PKG-INFO", "SOURCES.txt", "dependency_links.txt", "requires.txt", "top_level.txt")
SDIST_ROOT_FILES = ("LICENSE", "PKG-INFO", "README.md", "pyproject.toml", "setup.cfg")
SETUP_CFG = b"[egg_info]\ntag_build = \ntag_date = 0\n\n"


def verify_artifact_names(
    wheel: Path,
    sdist: Path,
    distribution: str,
    version: str,
    require: Require,
) -> None:
    """Bind artifact paths to canonical pure-Python distribution filenames."""
    stem = re.sub(r"[-_.]+", "_", distribution).lower()
    expected_wheel = f"{stem}-{version}-py3-none-any.whl"
    expected_sdist = f"{stem}-{version}.tar.gz"
    require(wheel.name == expected_wheel, f"wheel canonical filename must be {expected_wheel}")
    require(sdist.name == expected_sdist, f"sdist canonical filename must be {expected_sdist}")


def _object_mapping(value: object, label: str, require: Require) -> Mapping[str, object]:
    require(isinstance(value, dict), f"immutable pyproject {label} must be a table")
    return cast(Mapping[str, object], value)


def _string_sequence(value: object, label: str, require: Require) -> Sequence[str]:
    require(isinstance(value, list), f"immutable pyproject {label} must be an array")
    items = cast(Sequence[object], value)
    require(all(isinstance(item, str) for item in items), f"immutable pyproject {label} must contain strings")
    return cast(Sequence[str], items)


def _extra_requirement(raw: str, extra: str) -> str:
    requirement = Requirement(raw)
    base = str(requirement).split(";", 1)[0].strip()
    marker = str(requirement.marker) if requirement.marker is not None else ""
    combined = f'{marker} and extra == "{extra}"' if marker else f'extra == "{extra}"'
    return str(Requirement(f"{base}; {combined}"))


def _project_metadata(sdk_root: Path, require: Require) -> Tuple[str, str, str, Tuple[str, ...]]:
    document = load_toml((sdk_root / "pyproject.toml").read_text(encoding="utf-8"))
    project = _object_mapping(document.get("project"), "project", require)
    name = project.get("name")
    version = project.get("version")
    requires_python = project.get("requires-python")
    require(isinstance(name, str), "immutable pyproject project.name must be text")
    require(isinstance(version, str), "immutable pyproject project.version must be text")
    require(isinstance(requires_python, str), "immutable pyproject requires-python must be text")
    requirements = [str(Requirement(raw)) for raw in _string_sequence(
        project.get("dependencies"), "dependencies", require
    )]
    optional = _object_mapping(project.get("optional-dependencies", {}), "optional-dependencies", require)
    for extra, raw_values in optional.items():
        values = _string_sequence(raw_values, f"optional-dependencies.{extra}", require)
        requirements.extend(_extra_requirement(raw, extra) for raw in values)
    return cast(str, name), cast(str, version), cast(str, requires_python), tuple(sorted(requirements))


def _core_metadata(data: bytes, label: str, require: Require) -> Tuple[str, str, str, Tuple[str, ...]]:
    message = Parser(policy=policy.default).parsestr(data.decode("utf-8"))
    name = str(message.get("Name", "")).strip()
    version = str(message.get("Version", "")).strip()
    requires_python = str(message.get("Requires-Python", "")).strip()
    requirements = tuple(sorted(str(Requirement(str(value))) for value in message.get_all(
        "Requires-Dist", []
    )))
    require(bool(name and version and requires_python), f"{label} core metadata is incomplete")
    return name, version, requires_python, requirements


def verify_build_metadata(
    wheel_metadata: bytes,
    sdist_metadata: bytes,
    sdist: Mapping[str, bytes],
    sdk_root: Path,
    require: Require,
) -> None:
    """Bind executable sdist config and dependency metadata to pyproject."""
    expected = _project_metadata(sdk_root, require)
    require(_core_metadata(wheel_metadata, "wheel", require) == expected,
            "wheel Name/Version/Requires-Python/Requires-Dist differ from immutable pyproject")
    require(_core_metadata(sdist_metadata, "sdist", require) == expected,
            "sdist Name/Version/Requires-Python/Requires-Dist differ from immutable pyproject")
    egg_metadata = sdist.get("cap_sdk_python.egg-info/PKG-INFO")
    require(egg_metadata == sdist_metadata, "sdist egg-info PKG-INFO differs from root PKG-INFO")
    setup = sdist.get("setup.cfg", b"").replace(b"\r\n", b"\n")
    require(setup == SETUP_CFG, "sdist setup.cfg differs from canonical setuptools output")


def source_package_paths(sdk_root: Path) -> Tuple[str, ...]:
    """Return the exact package files that both artifacts must contain."""
    python_files = tuple(
        path.relative_to(sdk_root).as_posix()
        for path in sorted((sdk_root / "cap").rglob("*.py"))
    )
    return python_files + ("cap/py.typed",)


def source_test_paths(sdk_root: Path) -> Tuple[str, ...]:
    """Return the exact top-level tests emitted by the current sdist build."""
    return tuple(
        path.relative_to(sdk_root).as_posix()
        for path in sorted((sdk_root / "tests").glob("test_*.py"))
    )


def _require_source_file(sdk_root: Path, relative: str, require: Require) -> bytes:
    path = sdk_root / relative
    require(path.is_file(), f"immutable SDK source is missing {relative}")
    return path.read_bytes()


def _require_source_match(
    entries: Mapping[str, bytes],
    kind: str,
    relative: str,
    source: bytes,
    require: Require,
) -> None:
    require(relative in entries, f"{kind} is missing {relative}")
    require(
        entries[relative] == source,
        f"{kind} {relative} differs from immutable SDK source",
    )


def _license_path(entries: Mapping[str, bytes], kind: str, require: Require) -> str:
    matches = [name for name in entries if PurePosixPath(name).name == "LICENSE"]
    require(len(matches) == 1, f"expected one {kind} LICENSE, found {len(matches)}")
    return matches[0]


def verify_source_bytes(
    wheel: Mapping[str, bytes],
    sdist: Mapping[str, bytes],
    sdk_root: Path,
    require: Require,
) -> Tuple[bytes, bytes]:
    """Bind source-backed artifact bytes to this verifier's SDK tree."""
    for relative in source_package_paths(sdk_root):
        source = _require_source_file(sdk_root, relative, require)
        _require_source_match(wheel, "wheel", relative, source, require)
        _require_source_match(sdist, "sdist", relative, source, require)
    for relative in source_test_paths(sdk_root):
        source = _require_source_file(sdk_root, relative, require)
        _require_source_match(sdist, "sdist", relative, source, require)
    readme = _require_source_file(sdk_root, "README.md", require)
    license_text = _require_source_file(sdk_root, "LICENSE", require)
    _require_source_match(sdist, "sdist", "README.md", readme, require)
    _require_source_match(sdist, "sdist", "pyproject.toml", _require_source_file(
        sdk_root, "pyproject.toml", require
    ), require)
    _require_source_match(sdist, "sdist", "LICENSE", license_text, require)
    wheel_license = _license_path(wheel, "wheel", require)
    require(wheel[wheel_license] == license_text, "wheel LICENSE differs from immutable SDK source")
    return readme, license_text


def _wheel_metadata_root(entries: Mapping[str, bytes], require: Require) -> str:
    matches = [name for name in entries if name.endswith(".dist-info/METADATA")]
    require(len(matches) == 1, f"expected one wheel METADATA, found {len(matches)}")
    parts = PurePosixPath(matches[0]).parts
    require(len(parts) == 2, f"wheel METADATA has unexpected path: {matches[0]}")
    return parts[0]


def _expected_wheel(entries: Mapping[str, bytes], sdk_root: Path, require: Require) -> Set[str]:
    metadata_root = _wheel_metadata_root(entries, require)
    metadata = {f"{metadata_root}/{relative}" for relative in WHEEL_METADATA}
    return set(source_package_paths(sdk_root)) | metadata


def _expected_sdist(sdk_root: Path) -> Set[str]:
    egg_info = {f"cap_sdk_python.egg-info/{name}" for name in SDIST_EGG_INFO}
    return (
        set(source_package_paths(sdk_root))
        | set(source_test_paths(sdk_root))
        | set(SDIST_ROOT_FILES)
        | egg_info
    )


def _verify_inventory(
    entries: Mapping[str, bytes], expected: Set[str], kind: str, require: Require
) -> None:
    actual = set(entries)
    wanted = set(expected)
    missing = sorted(wanted - actual)
    unexpected = sorted(actual - wanted)
    require(
        not missing and not unexpected,
        f"{kind} inventory missing={missing} unexpected={unexpected}",
    )


def verify_inventory(
    wheel: Mapping[str, bytes],
    sdist: Mapping[str, bytes],
    sdk_root: Path,
    require: Require,
) -> None:
    """Reject every archive member outside the current build's exact allowlist."""
    _verify_inventory(wheel, _expected_wheel(wheel, sdk_root, require), "wheel", require)
    _verify_inventory(sdist, _expected_sdist(sdk_root), "sdist", require)
