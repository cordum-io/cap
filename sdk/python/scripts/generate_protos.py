#!/usr/bin/env python
"""Hermetically generate CAP Python stubs in temp and check tracked bytes."""

from __future__ import annotations

import argparse
import difflib
import json
import os
import subprocess
import sys
import tempfile
from importlib import metadata
from pathlib import Path, PurePosixPath
from typing import Dict, Mapping, Optional, Sequence, Set, Tuple


REPO_ROOT = Path(__file__).resolve().parents[3]
SDK_ROOT = Path(__file__).resolve().parents[1]
PROTO_ROOT = REPO_ROOT / "proto" / "cordum" / "agent" / "v1"
PROTO_INCLUDE = REPO_ROOT / "proto"
TRACKED_ROOT = SDK_ROOT / "cap" / "pb"
REQUIREMENTS = SDK_ROOT / "requirements-codegen.txt"
GENERATED_ROOT = PurePosixPath("cordum/agent/v1")
PINNED_VERSIONS = {
    "grpcio": "1.76.0",
    "grpcio-tools": "1.76.0",
    "protobuf": "6.33.5",
}
CODEGEN_TIMEOUT_SECONDS = 600


class CodegenError(RuntimeError):
    """A fail-closed code-generation contract violation."""


def _require(condition: bool, message: str) -> None:
    if not condition:
        raise CodegenError(message)


def load_pins() -> Mapping[str, str]:
    _require(REQUIREMENTS.is_file(), f"missing codegen requirements: {REQUIREMENTS}")
    pins: Dict[str, str] = {}
    for raw in REQUIREMENTS.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        _require(line.count("==") == 1, f"codegen dependency is not exactly pinned: {line}")
        name, version = line.split("==", 1)
        _require(bool(name and version), f"invalid codegen pin: {line}")
        _require(name not in pins, f"duplicate codegen pin: {name}")
        pins[name] = version
    _require(pins == PINNED_VERSIONS, f"unexpected codegen pins: {pins}")
    return pins


def verify_tool_versions() -> Mapping[str, str]:
    pins = load_pins()
    installed: Dict[str, str] = {}
    for name, expected in pins.items():
        try:
            actual = metadata.version(name)
        except metadata.PackageNotFoundError as exc:
            raise CodegenError(f"required codegen tool is missing: {name}=={expected}") from exc
        _require(actual == expected, f"{name} installed {actual}, required {expected}")
        installed[name] = actual
    return installed


def canonical_protos() -> Tuple[Path, ...]:
    _require(PROTO_ROOT.is_dir(), f"canonical proto directory is missing: {PROTO_ROOT}")
    protos = tuple(sorted(PROTO_ROOT.glob("*.proto"), key=lambda path: path.name))
    _require(bool(protos), f"no canonical protos found under {PROTO_ROOT}")
    return protos


def expected_outputs(protos: Sequence[Path]) -> Tuple[PurePosixPath, ...]:
    outputs = [
        GENERATED_ROOT / f"{proto.stem}_{suffix}.py"
        for proto in protos
        for suffix in ("pb2", "pb2_grpc")
    ]
    return tuple(sorted(outputs, key=lambda path: path.as_posix()))


def _grpc_include() -> Path:
    try:
        distribution = metadata.distribution("grpcio-tools")
    except metadata.PackageNotFoundError as exc:
        raise CodegenError("required codegen tool is missing: grpcio-tools") from exc
    include = Path(distribution.locate_file("grpc_tools/_proto")).resolve()
    _require(include.is_dir(), f"grpcio-tools include directory is missing: {include}")
    return include


def run_protoc(destination: Path, protos: Tuple[Path, ...]) -> None:
    destination.mkdir(parents=True, exist_ok=True)
    command = [
        sys.executable,
        "-m",
        "grpc_tools.protoc",
        f"-I{PROTO_INCLUDE}",
        f"-I{_grpc_include()}",
        f"--python_out={destination}",
        f"--grpc_python_out={destination}",
        *(str(path) for path in protos),
    ]
    environment = os.environ.copy()
    environment.pop("PYTHONHOME", None)
    environment.pop("PYTHONPATH", None)
    try:
        result = subprocess.run(
            command,
            cwd=str(REPO_ROOT),
            env=environment,
            text=True,
            capture_output=True,
            check=False,
            timeout=CODEGEN_TIMEOUT_SECONDS,
        )
    except subprocess.TimeoutExpired as exc:
        raise CodegenError(
            f"grpc_tools.protoc timed out after {CODEGEN_TIMEOUT_SECONDS} seconds"
        ) from exc
    if result.returncode:
        output = (result.stdout + "\n" + result.stderr).strip()[-4000:]
        raise CodegenError(f"grpc_tools.protoc failed ({result.returncode})\n{output}")


def _inventory(root: Path) -> Set[PurePosixPath]:
    found: Set[PurePosixPath] = set()
    if not root.is_dir():
        return found
    for path in root.rglob("*.py"):
        if path.name.endswith("_pb2.py") or path.name.endswith("_pb2_grpc.py"):
            found.add(PurePosixPath(path.relative_to(root).as_posix()))
    return found


def _check_inventory(root: Path, expected: Set[PurePosixPath], label: str) -> None:
    actual = _inventory(root)
    missing = sorted(path.as_posix() for path in expected - actual)
    extra = sorted(path.as_posix() for path in actual - expected)
    _require(not missing, f"{label} outputs are missing: {missing}")
    _require(not extra, f"extra {label} outputs: {extra}")


def _normalized_bytes(path: Path) -> bytes:
    return path.read_bytes().replace(b"\r\n", b"\n")


def _byte_diff(relative: PurePosixPath, tracked: bytes, generated: bytes) -> str:
    before = tracked.decode("utf-8", errors="replace").splitlines()
    after = generated.decode("utf-8", errors="replace").splitlines()
    lines = difflib.unified_diff(before, after, fromfile=f"tracked/{relative}",
                                 tofile=f"generated/{relative}", n=2, lineterm="")
    return "\n".join(list(lines)[:40])[:4000]


def compare_outputs(
    generated_root: Path,
    tracked_root: Path,
    outputs: Sequence[PurePosixPath],
) -> None:
    expected = set(outputs)
    _check_inventory(generated_root, expected, "generated")
    _check_inventory(tracked_root, expected, "tracked")
    drifts = []
    for relative in outputs:
        generated = _normalized_bytes(generated_root / Path(*relative.parts))
        tracked = _normalized_bytes(tracked_root / Path(*relative.parts))
        if generated != tracked:
            drifts.append(_byte_diff(relative, tracked, generated))
    _require(not drifts, "generated byte drift:\n" + "\n".join(drifts))


def check_generated() -> Mapping[str, object]:
    versions = verify_tool_versions()
    protos = canonical_protos()
    outputs = expected_outputs(protos)
    with tempfile.TemporaryDirectory(prefix="cap-python-codegen-") as temporary:
        generated_root = Path(temporary).resolve()
        run_protoc(generated_root, protos)
        compare_outputs(generated_root, TRACKED_ROOT, outputs)
    return {
        "mode": "check",
        "output_files": [path.as_posix() for path in outputs],
        "outputs": len(outputs),
        "protos": [path.name for path in protos],
        "tool_versions": dict(sorted(versions.items())),
    }


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", required=True,
                        help="generate in temp and compare with tracked files")
    return parser


def main(argv: Optional[Sequence[str]] = None) -> int:
    _parser().parse_args(argv)
    try:
        report = check_generated()
    except (CodegenError, OSError, UnicodeError) as exc:
        print(f"Python codegen check failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
