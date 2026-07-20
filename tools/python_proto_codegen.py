"""Pinned grpcio-tools generation for both checked-in Python surfaces."""

from __future__ import annotations

import sys
from importlib import metadata
from pathlib import Path
from typing import Callable, Mapping, Sequence

REPO_ROOT = Path(__file__).resolve().parents[1]
REQUIREMENTS = REPO_ROOT / "sdk" / "python" / "requirements-codegen.txt"
PROTO_INCLUDE = REPO_ROOT / "proto"
PINNED_VERSIONS = {
    "grpcio": "1.76.0",
    "grpcio-tools": "1.76.0",
    "protobuf": "6.33.5",
}
RunCommand = Callable[[Sequence[str], Path], None]


class PythonCodegenError(RuntimeError):
    """A Python generator pin or invocation is not reproducible."""


def load_python_pins(path: Path = REQUIREMENTS) -> Mapping[str, str]:
    if not path.is_file():
        raise PythonCodegenError(f"missing Python codegen requirements: {path}")
    pins: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.count("==") != 1:
            raise PythonCodegenError(f"Python codegen dependency is not pinned: {line}")
        name, version = line.split("==", 1)
        if not name or not version or name in pins:
            raise PythonCodegenError(f"invalid Python codegen pin: {line}")
        pins[name] = version
    if pins != PINNED_VERSIONS:
        raise PythonCodegenError(f"unexpected Python codegen pins: {pins}")
    return pins


def validate_python_pins(path: Path = REQUIREMENTS) -> Mapping[str, str]:
    pins = load_python_pins(path)
    installed: dict[str, str] = {}
    for name, expected in pins.items():
        try:
            actual = metadata.version(name)
        except metadata.PackageNotFoundError as exc:
            raise PythonCodegenError(f"missing {name}=={expected}") from exc
        if actual != expected:
            raise PythonCodegenError(f"{name} installed {actual}, required {expected}")
        installed[name] = actual
    return installed


def grpc_include() -> Path:
    try:
        distribution = metadata.distribution("grpcio-tools")
    except metadata.PackageNotFoundError as exc:
        raise PythonCodegenError("missing grpcio-tools") from exc
    include = Path(distribution.locate_file("grpc_tools/_proto")).resolve()
    if not include.is_dir():
        raise PythonCodegenError(f"grpcio-tools include is missing: {include}")
    return include


def generate_python_outputs(
    destination: Path,
    protos: Sequence[Path],
    run_command: RunCommand,
) -> None:
    validate_python_pins()
    include = grpc_include()
    for surface in ("python", "sdk-python"):
        output = destination / surface
        output.mkdir(parents=True, exist_ok=True)
        command = [
            sys.executable,
            "-m",
            "grpc_tools.protoc",
            f"-I{PROTO_INCLUDE}",
            f"-I{include}",
            f"--python_out={output}",
            f"--grpc_python_out={output}",
            *(str(path) for path in protos),
        ]
        run_command(command, REPO_ROOT)
