"""Helpers for clean Python SDK distribution tests."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tarfile
import zipfile
from dataclasses import dataclass
from pathlib import Path, PurePosixPath
from typing import Dict, Iterable, Mapping, Optional, Sequence, Set


SDK_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = Path(__file__).resolve().parents[3]
PROTO_ROOT = REPO_ROOT / "proto" / "cordum" / "agent" / "v1"
GENERATED_ROOT = PurePosixPath("cap/pb/cordum/agent/v1")

_COPY_IGNORE = shutil.ignore_patterns(
    ".git",
    ".mypy_cache",
    ".pytest_cache",
    ".venv",
    "__pycache__",
    "*.egg-info",
    "*.pyc",
    "build",
    "dist",
)

_IMPORT_SMOKE = r"""
import importlib
import json
from importlib import metadata
from pathlib import Path
import sys

expected = json.loads(sys.argv[1])
repo_root = Path(sys.argv[2]).resolve()
import cap

origin = Path(cap.__file__).resolve()
assert origin != repo_root and repo_root not in origin.parents, origin
for module_name in expected:
    importlib.import_module(module_name)

from cap.pb.cordum.agent.v1 import job_pb2
payload = job_pb2.JobRequest(job_id="artifact-smoke").SerializeToString()
for entry in filter(None, sys.path):
    resolved = Path(entry).resolve()
    assert resolved != repo_root and repo_root not in resolved.parents, resolved
print(json.dumps({
    "bytes": len(payload),
    "cap_file": str(origin),
    "grpcio_version": metadata.version("grpcio"),
    "imports": len(expected),
    "protobuf_version": metadata.version("protobuf"),
}, sort_keys=True))
"""

DEPENDENCY_FLOORS = {"protobuf": "6.31.1", "grpcio": "1.76.0"}


@dataclass(frozen=True)
class BuiltArtifacts:
    wheel: Path
    sdist: Path


def canonical_proto_stems() -> Sequence[str]:
    """Return the canonical proto stems in deterministic order."""
    return tuple(path.stem for path in sorted(PROTO_ROOT.glob("*.proto")))


def expected_generated_paths() -> Set[str]:
    """Return normalized archive paths for every canonical Python stub."""
    expected: Set[str] = set()
    for stem in canonical_proto_stems():
        expected.add(str(GENERATED_ROOT / f"{stem}_pb2.py"))
        expected.add(str(GENERATED_ROOT / f"{stem}_pb2_grpc.py"))
    return expected


def expected_import_names() -> Sequence[str]:
    """Return import names corresponding to the canonical generated paths."""
    return tuple(
        path[:-3].replace("/", ".")
        for path in sorted(expected_generated_paths())
    )


def clean_environment() -> Dict[str, str]:
    """Return an environment that cannot import from a caller's source path."""
    env = os.environ.copy()
    env.pop("PYTHONHOME", None)
    env.pop("PYTHONPATH", None)
    env["PIP_DISABLE_PIP_VERSION_CHECK"] = "1"
    env["PYTHONNOUSERSITE"] = "1"
    return env


def _command_failure(args: Sequence[str], result: subprocess.CompletedProcess[str]) -> str:
    command = " ".join(args)
    return f"command failed ({result.returncode}): {command}\n{result.stdout}\n{result.stderr}"


def run_checked(args: Sequence[str], cwd: Path) -> subprocess.CompletedProcess[str]:
    """Run a command with captured output and a source-clean environment."""
    result = subprocess.run(
        list(args),
        cwd=str(cwd),
        env=clean_environment(),
        text=True,
        capture_output=True,
        check=False,
    )
    assert result.returncode == 0, _command_failure(args, result)
    return result


def build_distributions(workspace: Path) -> BuiltArtifacts:
    """Build wheel and sdist from an external copy of the SDK source."""
    source = workspace / "source"
    output = workspace / "dist"
    shutil.copytree(SDK_ROOT, source, ignore=_COPY_IGNORE)
    output.mkdir()
    run_checked(
        (sys.executable, "-m", "build", "--outdir", str(output), str(source)),
        workspace,
    )
    wheels = sorted(output.glob("*.whl"))
    sdists = sorted(output.glob("*.tar.gz"))
    assert len(wheels) == 1, f"expected one wheel, found: {wheels}"
    assert len(sdists) == 1, f"expected one sdist, found: {sdists}"
    return BuiltArtifacts(wheel=wheels[0], sdist=sdists[0])


def _strip_sdist_root(name: str) -> str:
    parts = PurePosixPath(name).parts
    assert len(parts) > 1, f"sdist member has no package root: {name}"
    return str(PurePosixPath(*parts[1:]))


def archive_entries(path: Path) -> Mapping[str, bytes]:
    """Read file entries from a wheel or sdist using normalized paths."""
    if path.suffix == ".whl":
        with zipfile.ZipFile(path) as archive:
            return {
                name: archive.read(name)
                for name in archive.namelist()
                if not name.endswith("/")
            }
    with tarfile.open(path, "r:gz") as archive:
        entries: Dict[str, bytes] = {}
        for member in archive.getmembers():
            if not member.isfile():
                continue
            handle = archive.extractfile(member)
            assert handle is not None, member.name
            entries[_strip_sdist_root(member.name)] = handle.read()
        return entries


def wheel_metadata(entries: Mapping[str, bytes]) -> str:
    """Return the wheel's unique core metadata document."""
    matches = [name for name in entries if name.endswith(".dist-info/METADATA")]
    assert len(matches) == 1, f"expected one METADATA file, found: {matches}"
    return entries[matches[0]].decode("utf-8")


def _resolve_python(candidate: str) -> Path:
    direct = Path(candidate)
    if direct.is_file():
        return direct.resolve()
    resolved = shutil.which(candidate)
    assert resolved is not None, f"Python executable not found: {candidate}"
    return Path(resolved).resolve()


def consumer_python() -> Path:
    """Select the interpreter used for installed-artifact compatibility smoke."""
    return _resolve_python(os.environ.get("CAP_TEST_PYTHON", sys.executable))


def _created_venv_python(venv: Path) -> Path:
    candidates = (venv / "Scripts" / "python.exe", venv / "bin" / "python")
    matches = [path for path in candidates if path.is_file()]
    assert len(matches) == 1, f"cannot locate venv Python under {venv}"
    return matches[0]


def install_artifact(artifact: Path, workspace: Path) -> Path:
    """Install one exact artifact into a fresh venv and run pip check."""
    venv = workspace / "venv"
    run_checked((str(consumer_python()), "-m", "venv", str(venv)), workspace)
    python = _created_venv_python(venv)
    pins = tuple(f"{name}=={version}" for name, version in DEPENDENCY_FLOORS.items())
    run_checked(
        (str(python), "-m", "pip", "install", "--no-cache-dir", *pins),
        workspace,
    )
    run_checked(
        (str(python), "-m", "pip", "install", "--no-cache-dir", str(artifact)),
        workspace,
    )
    run_checked((str(python), "-m", "pip", "check"), workspace)
    return python


def smoke_installed_artifact(
    artifact: Path,
    workspace: Path,
    import_names: Optional[Iterable[str]] = None,
) -> subprocess.CompletedProcess[str]:
    """Install and import an artifact from an isolated external consumer cwd."""
    workspace.mkdir()
    python = install_artifact(artifact, workspace)
    consumer = workspace / "consumer"
    consumer.mkdir()
    names = tuple(import_names or expected_import_names())
    return subprocess.run(
        (str(python), "-I", "-c", _IMPORT_SMOKE, json.dumps(names), str(REPO_ROOT)),
        cwd=str(consumer),
        env=clean_environment(),
        text=True,
        capture_output=True,
        check=False,
    )
