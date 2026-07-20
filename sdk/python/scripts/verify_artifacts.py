#!/usr/bin/env python
"""Fail-closed verification for exact CAP Python wheel and sdist artifacts."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
import zipfile
from dataclasses import dataclass
from email import policy
from email.parser import Parser
from pathlib import Path, PurePosixPath
from typing import Callable, Dict, Mapping, Optional, Sequence, Tuple


SCRIPT_ROOT = Path(__file__).resolve().parent
if str(SCRIPT_ROOT) not in sys.path:
    sys.path.insert(0, str(SCRIPT_ROOT))
from artifact_contract import verify_artifact_names, verify_build_metadata, verify_inventory, verify_source_bytes, verify_worker_trust_smoke


REPO_ROOT = Path(__file__).resolve().parents[3]
SDK_ROOT = Path(__file__).resolve().parents[1]
GENERATED_ROOT = "cap/pb/cordum/agent/v1"
EXPECTED_STEMS = ("alert", "buspacket", "handshake", "heartbeat", "job", "policy", "safety")
EXPECTED_GENERATED = tuple(
    f"{GENERATED_ROOT}/{stem}_{suffix}.py"
    for stem in EXPECTED_STEMS
    for suffix in ("pb2", "pb2_grpc")
)
EXPECTED_PACKAGE_FILES = tuple(path.relative_to(SDK_ROOT).as_posix() for path in sorted((SDK_ROOT / "cap").rglob("*.py")))
GENERATED_IMPORTS = tuple(path[:-3].replace("/", ".") for path in EXPECTED_GENERATED)
EXPECTED_IMPORTS = tuple(sorted(
    {path[:-12].replace("/", ".") if path.endswith("/__init__.py") else path[:-3].replace("/", ".")
     for path in EXPECTED_PACKAGE_FILES}
    | set(GENERATED_IMPORTS) | {name.replace("cap.pb.", "") for name in GENERATED_IMPORTS}
))
SMOKE_SCRIPT = Path(__file__).with_name("artifact_smoke.py")
DEPENDENCY_FLOORS = {"protobuf": "6.31.1", "grpcio": "1.76.0"}

class VerificationError(RuntimeError):
    """An explicit artifact-contract violation."""


@dataclass(frozen=True)
class Inspection:
    name: str
    version: str
    wheel: Mapping[str, object]
    sdist: Mapping[str, object]
    readme_sha256: str
    license_sha256: str


def _require(condition: bool, message: str) -> None:
    if not condition:
        raise VerificationError(message)


def _digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _validate_member(name: str, kind: str) -> Tuple[str, ...]:
    _require("\\" not in name, f"{kind} member uses backslashes: {name}")
    path = PurePosixPath(name)
    parts = path.parts
    _require(bool(parts) and not path.is_absolute(), f"{kind} has unsafe member: {name}")
    _require(".." not in parts, f"{kind} has path traversal member: {name}")
    return parts


def _read_wheel(path: Path) -> Dict[str, bytes]:
    entries: Dict[str, bytes] = {}
    try:
        with zipfile.ZipFile(path) as archive:
            for item in archive.infolist():
                _validate_member(item.filename, "wheel")
                if item.is_dir():
                    continue
                _require(item.filename not in entries, f"wheel duplicate member: {item.filename}")
                entries[item.filename] = archive.read(item)
    except (OSError, zipfile.BadZipFile) as exc:
        raise VerificationError(f"cannot read wheel {path.name}: {exc}") from exc
    return entries


def _read_sdist(path: Path) -> Dict[str, bytes]:
    raw: Dict[str, bytes] = {}
    roots = set()
    try:
        with tarfile.open(path, "r:gz") as archive:
            for item in archive.getmembers():
                parts = _validate_member(item.name, "sdist")
                _require(item.isfile() or item.isdir(), f"sdist unsupported member: {item.name}")
                if item.isdir():
                    continue
                _require(len(parts) > 1, f"sdist member lacks root directory: {item.name}")
                roots.add(parts[0])
                relative = str(PurePosixPath(*parts[1:]))
                _require(relative not in raw, f"sdist duplicate member: {relative}")
                handle = archive.extractfile(item)
                _require(handle is not None, f"sdist member cannot be read: {item.name}")
                raw[relative] = handle.read() if handle is not None else b""
    except (OSError, tarfile.TarError) as exc:
        raise VerificationError(f"cannot read sdist {path.name}: {exc}") from exc
    _require(len(roots) == 1, f"sdist must have one root directory, found: {sorted(roots)}")
    return raw


def _unique(entries: Mapping[str, bytes], predicate: Callable[[str], bool], label: str) -> bytes:
    matches = [content for name, content in entries.items() if predicate(name)]
    _require(len(matches) == 1, f"expected one {label}, found {len(matches)}")
    return matches[0]


def _metadata(data: bytes, label: str) -> Tuple[str, str, str, str]:
    message = Parser(policy=policy.default).parsestr(data.decode("utf-8"))
    name = str(message.get("Name", "")).strip()
    version = str(message.get("Version", "")).strip()
    content_type = str(message.get("Description-Content-Type", "")).strip()
    payload = message.get_payload()
    _require(bool(name), f"{label} metadata has no Name")
    _require(bool(version), f"{label} metadata has no Version")
    _require(isinstance(payload, str), f"{label} metadata description is not text")
    return name, version, content_type, str(payload)


def _verify_generated(entries: Mapping[str, bytes], kind: str) -> None:
    actual = {
        name for name in entries
        if name.endswith("_pb2.py") or name.endswith("_pb2_grpc.py")
    }
    expected = set(EXPECTED_GENERATED)
    missing, extra = sorted(expected - actual), sorted(actual - expected)
    _require(not missing and not extra, f"{kind} generated modules missing={missing} extra={extra}")


def _verify_license_metadata(data: bytes) -> None:
    message = Parser(policy=policy.default).parsestr(data.decode("utf-8"))
    _require(message.get("License-Expression") == "Apache-2.0", "wheel metadata has wrong License-Expression")
    files = [str(value) for value in message.get_all("License-File", [])]
    _require(any(PurePosixPath(value).name == "LICENSE" for value in files), "wheel metadata does not declare LICENSE")


def _inventory(path: Path, entries: Mapping[str, bytes]) -> Mapping[str, object]:
    return {"entries": sorted(entries), "filename": path.name,
            "sha256": _digest(path.read_bytes()), "size": path.stat().st_size}


def _normalize(text: str) -> str:
    return text.replace("\r\n", "\n").strip()


def inspect_artifacts(wheel: Path, sdist: Path) -> Inspection:
    wheel, sdist = wheel.resolve(), sdist.resolve()
    _require(wheel.is_file() and wheel.suffix == ".whl", f"wheel path is not an exact .whl file: {wheel}")
    _require(sdist.is_file() and sdist.name.endswith(".tar.gz"), f"sdist path is not an exact .tar.gz file: {sdist}")
    wheel_entries, sdist_entries = _read_wheel(wheel), _read_sdist(sdist)
    _verify_generated(wheel_entries, "wheel")
    _verify_generated(sdist_entries, "sdist")
    source_readme, source_license = verify_source_bytes(wheel_entries, sdist_entries, SDK_ROOT, _require)
    verify_inventory(wheel_entries, sdist_entries, SDK_ROOT, _require)
    _require("README.md" in sdist_entries, "sdist is missing README.md")
    _require("pyproject.toml" in sdist_entries, "sdist is missing pyproject.toml")
    _require(bool(sdist_entries["pyproject.toml"].strip()), "sdist pyproject.toml is empty")
    wheel_meta = _unique(wheel_entries, lambda n: n.endswith(".dist-info/METADATA"), "wheel METADATA")
    _verify_license_metadata(wheel_meta)
    sdist_meta = _unique(sdist_entries, lambda n: n == "PKG-INFO", "sdist PKG-INFO")
    wheel_name, wheel_version, wheel_type, wheel_body = _metadata(wheel_meta, "wheel")
    sdist_name, sdist_version, _, sdist_body = _metadata(sdist_meta, "sdist")
    _require((wheel_name, wheel_version) == (sdist_name, sdist_version), "wheel/sdist Name or Version mismatch")
    verify_build_metadata(wheel_meta, sdist_meta, sdist_entries, SDK_ROOT, _require)
    verify_artifact_names(wheel, sdist, wheel_name, wheel_version, _require)
    _require(wheel_type.startswith("text/markdown"), "wheel README metadata is not text/markdown")
    readme = sdist_entries["README.md"].decode("utf-8")
    _require(_normalize(wheel_body) == _normalize(readme), "wheel README metadata differs from sdist README.md")
    _require(_normalize(sdist_body) == _normalize(readme), "sdist PKG-INFO differs from README.md")
    wheel_license = _unique(wheel_entries, lambda n: PurePosixPath(n).name == "LICENSE", "wheel LICENSE")
    sdist_license = _unique(sdist_entries, lambda n: PurePosixPath(n).name == "LICENSE", "sdist LICENSE")
    _require(bool(wheel_license.strip()), "LICENSE is empty")
    _require(wheel_license == sdist_license, "wheel/sdist LICENSE contents differ")
    return Inspection(wheel_name, wheel_version, _inventory(wheel, wheel_entries),
                      _inventory(sdist, sdist_entries), _digest(source_readme),
                      _digest(source_license))


def _clean_environment() -> Dict[str, str]:
    env = os.environ.copy()
    for name in ("PYTHONHOME", "PYTHONPATH", "VIRTUAL_ENV"):
        env.pop(name, None)
    env["PIP_DISABLE_PIP_VERSION_CHECK"] = "1"
    env["PYTHONNOUSERSITE"] = "1"
    return env


def _run(args: Sequence[str], cwd: Path, timeout: int) -> subprocess.CompletedProcess[str]:
    try:
        result = subprocess.run(tuple(args), cwd=str(cwd), env=_clean_environment(),
                                text=True, capture_output=True, check=False, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise VerificationError(f"command could not complete: {' '.join(args)}: {exc}") from exc
    if result.returncode:
        output = (result.stdout + "\n" + result.stderr).strip()[-4000:]
        raise VerificationError(f"command failed ({result.returncode}): {' '.join(args)}\n{output}")
    return result


def _resolve_python(candidate: str) -> Path:
    path = Path(candidate)
    resolved = path.resolve() if path.is_file() else Path(shutil.which(candidate) or "")
    _require(resolved.is_file(), f"Python executable not found: {candidate}")
    return resolved


def _venv_python(venv: Path) -> Path:
    matches = [path for path in (venv / "Scripts/python.exe", venv / "bin/python") if path.is_file()]
    _require(len(matches) == 1, f"cannot locate venv Python under {venv}")
    return matches[0]


def _inside(path: Path, root: Path) -> bool:
    return path == root or root in path.parents


def _consumer_check(artifact: Path, python: Path, timeout: int) -> Mapping[str, object]:
    with tempfile.TemporaryDirectory(prefix="cap-artifact-consumer-") as temporary:
        workspace = Path(temporary).resolve()
        _require(not _inside(workspace, REPO_ROOT), f"consumer workspace is inside repository: {workspace}")
        venv, consumer = workspace / "venv", workspace / "consumer"
        consumer.mkdir()
        _run((str(python), "-m", "venv", str(venv)), workspace, timeout)
        installed_python = _venv_python(venv)
        pins = tuple(f"{name}=={version}" for name, version in DEPENDENCY_FLOORS.items())
        _run((str(installed_python), "-m", "pip", "install", "--no-cache-dir", *pins), workspace, timeout)
        _run((str(installed_python), "-m", "pip", "install", "--no-cache-dir", str(artifact)), workspace, timeout)
        _run((str(installed_python), "-m", "pip", "check"), workspace, timeout)
        smoke = _run((str(installed_python), "-I", str(SMOKE_SCRIPT),
                      json.dumps(EXPECTED_IMPORTS), str(REPO_ROOT), str(venv)), consumer, timeout)
        try:
            evidence = json.loads(smoke.stdout)
        except json.JSONDecodeError as exc:
            raise VerificationError(f"consumer smoke returned invalid JSON: {smoke.stdout}") from exc
        _require(evidence.get("imports") == len(EXPECTED_IMPORTS), "consumer imported the wrong module count")
        versions = {
            "protobuf": evidence.get("protobuf_version"),
            "grpcio": evidence.get("grpcio_version"),
        }
        for name, expected in DEPENDENCY_FLOORS.items():
            _require(versions[name] == expected, f"{name} exact floor mismatch: {versions[name]} != {expected}")
        verify_worker_trust_smoke(evidence, _require)
        return {"dependency_versions": versions, "imports": len(EXPECTED_IMPORTS),
                "pip_check": "ok", "source_shadow": "absent", "worker_trust": "ok"}


def verify_artifacts(wheel: Path, sdist: Path, python: Path, timeout: int) -> Mapping[str, object]:
    _require(timeout > 0, "timeout must be positive")
    inspection = inspect_artifacts(wheel, sdist)
    consumers = {"sdist": _consumer_check(sdist.resolve(), python, timeout), "wheel": _consumer_check(wheel.resolve(), python, timeout)}
    return {
        "artifacts": {"sdist": inspection.sdist, "wheel": inspection.wheel},
        "consumer_checks": consumers,
        "distribution": {"name": inspection.name, "version": inspection.version},
        "generated_modules": list(GENERATED_IMPORTS),
        "imported_modules": list(EXPECTED_IMPORTS),
        "license_sha256": inspection.license_sha256,
        "readme_sha256": inspection.readme_sha256,
        "schema": "cap-python-artifact-verification/v1",
    }


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--wheel", required=True, type=Path, help="exact wheel path")
    parser.add_argument("--sdist", required=True, type=Path, help="exact .tar.gz path")
    parser.add_argument("--python", default=os.environ.get("CAP_TEST_PYTHON", sys.executable))
    parser.add_argument("--timeout", type=int, default=600, help="per-command timeout seconds")
    return parser


def main(argv: Optional[Sequence[str]] = None) -> int:
    args = _parser().parse_args(argv)
    try:
        report = verify_artifacts(args.wheel, args.sdist, _resolve_python(args.python), args.timeout)
    except (OSError, UnicodeError, VerificationError) as exc:
        print(f"artifact verification failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
