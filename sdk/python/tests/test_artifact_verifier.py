"""Focused tests for the standalone exact-artifact verifier."""

from __future__ import annotations

import importlib.util
import io
import json
import sys
import tarfile
import zipfile
from pathlib import Path
from types import ModuleType
from typing import Dict, Mapping, Optional, Tuple
from unittest.mock import patch

import pytest

SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "verify_artifacts.py"
SDK_ROOT = SCRIPT.parents[1]
def _project_version() -> str:
    if sys.version_info >= (3, 11):
        import tomllib as _toml
    else:
        import tomli as _toml
    project = _toml.loads((SDK_ROOT / "pyproject.toml").read_text(encoding="utf-8"))["project"]
    assert isinstance(project, dict)
    return str(project["version"])


PROJECT_VERSION = _project_version()


def _load_verifier() -> ModuleType:
    spec = importlib.util.spec_from_file_location("cap_artifact_verifier", SCRIPT)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


verifier = _load_verifier()

def _metadata(version: str, readme: bytes) -> bytes:
    requirements = (
        "protobuf<7,>=6.31.1", "grpcio<2,>=1.76.0", "nats-py>=2.6.0",
        "cryptography>=41.0.0", "pydantic>=2.6.0", "redis>=5.0.0",
        'build<2,>=1.2; extra == "dev"', 'mypy<2,>=1.8; extra == "dev"',
        'packaging>=23.2; extra == "dev"', 'pdoc>=14.0; extra == "dev"',
        'PyYAML<7,>=6.0; extra == "dev"',
        'pytest<9,>=8.0; extra == "dev"',
        'tomli>=2.0; python_version < "3.11" and extra == "dev"',
        'twine<7,>=5.0; extra == "dev"',
    )
    header = (
        "Metadata-Version: 2.4\n"
        "Name: cap-sdk-python\n"
        f"Version: {version}\n"
        "Requires-Python: >=3.9\n"
    )
    body = (
        "Description-Content-Type: text/markdown\n"
        "License-Expression: Apache-2.0\n"
        "License-File: LICENSE\n\n"
        f"{readme.decode('utf-8')}\n"
    )
    return (header + "".join(f"Requires-Dist: {value}\n" for value in requirements) + body).encode()

def _source_bytes(relative: str) -> bytes:
    return (SDK_ROOT / relative).read_bytes()

def _entries(
    version: str,
    omit: Optional[str] = None,
    replacements: Optional[Mapping[str, bytes]] = None,
) -> Tuple[Dict[str, bytes], Dict[str, bytes]]:
    changed = dict(replacements or {})
    readme = changed.get("README.md", _source_bytes("README.md"))
    license_text = changed.get("LICENSE", _source_bytes("LICENSE"))
    package = {
        name: changed.get(name, _source_bytes(name))
        for name in (*verifier.EXPECTED_PACKAGE_FILES, "cap/py.typed")
    }
    metadata = changed.get("METADATA", _metadata(version, readme))
    dist_info = f"cap_sdk_python-{version}.dist-info"
    wheel = {
        f"{dist_info}/METADATA": metadata,
        f"{dist_info}/RECORD": b"",
        f"{dist_info}/WHEEL": b"Wheel-Version: 1.0\n",
        f"{dist_info}/licenses/LICENSE": license_text,
        f"{dist_info}/top_level.txt": b"cap\n",
        **package,
    }
    sdist = {
        "LICENSE": license_text,
        "PKG-INFO": metadata,
        "README.md": readme,
        "pyproject.toml": changed.get("pyproject.toml", _source_bytes("pyproject.toml")),
        "setup.cfg": changed.get("setup.cfg", b"[egg_info]\ntag_build = \ntag_date = 0\n\n"),
        **package,
    }
    for name in ("PKG-INFO", "SOURCES.txt", "dependency_links.txt", "requires.txt", "top_level.txt"):
        sdist[f"cap_sdk_python.egg-info/{name}"] = metadata if name == "PKG-INFO" else b""
    for path in sorted((SDK_ROOT / "tests").glob("test_*.py")):
        relative = path.relative_to(SDK_ROOT).as_posix()
        sdist[relative] = path.read_bytes()
    if omit is not None:
        wheel.pop(omit, None)
        sdist.pop(omit, None)
    return wheel, sdist

def _write_pair(
    root: Path,
    omit: Optional[str] = None,
    wheel_version: str = PROJECT_VERSION,
    sdist_version: str = PROJECT_VERSION,
    replacements: Optional[Mapping[str, bytes]] = None,
    extra_wheel: Optional[Mapping[str, bytes]] = None,
    extra_sdist: Optional[Mapping[str, bytes]] = None,
) -> Tuple[Path, Path]:
    wheel_entries, _ = _entries(wheel_version, omit, replacements)
    _, sdist_entries = _entries(sdist_version, omit, replacements)
    wheel_entries.update(extra_wheel or {})
    sdist_entries.update(extra_sdist or {})
    wheel = root / f"cap_sdk_python-{PROJECT_VERSION}-py3-none-any.whl"
    sdist = root / f"cap_sdk_python-{PROJECT_VERSION}.tar.gz"
    with zipfile.ZipFile(wheel, "w") as archive:
        for name, content in sorted(wheel_entries.items()):
            archive.writestr(name, content)
    with tarfile.open(sdist, "w:gz") as archive:
        for name, content in sorted(sdist_entries.items()):
            info = tarfile.TarInfo(f"cap_sdk_python-{PROJECT_VERSION}/{name}")
            info.size = len(content)
            archive.addfile(info, io.BytesIO(content))
    return wheel, sdist

def test_complete_pair_has_exact_deterministic_inventory(tmp_path: Path) -> None:
    wheel, sdist = _write_pair(tmp_path)
    first = verifier.inspect_artifacts(wheel, sdist)
    second = verifier.inspect_artifacts(wheel, sdist)
    assert first == second
    assert (first.name, first.version) == ("cap-sdk-python", PROJECT_VERSION)
    assert len(verifier.EXPECTED_GENERATED) == 14
    assert first.wheel["entries"] == sorted(first.wheel["entries"])
    assert first.sdist["entries"] == sorted(first.sdist["entries"])
    assert len(first.readme_sha256) == len(first.license_sha256) == 64

@pytest.mark.parametrize(
    ("omitted", "message"),
    (
        ("cap/py.typed", "missing cap/py.typed"),
        ("README.md", "missing README.md"),
        ("cap/worker.py", "is missing cap/worker.py"),
        (verifier.EXPECTED_GENERATED[-1], "generated modules missing="),
    ),
)
def test_required_contents_fail_closed(tmp_path: Path, omitted: str, message: str) -> None:
    wheel, sdist = _write_pair(tmp_path, omit=omitted)
    with pytest.raises(verifier.VerificationError, match=message):
        verifier.inspect_artifacts(wheel, sdist)

def test_mismatched_distribution_versions_fail_closed(tmp_path: Path) -> None:
    wheel, sdist = _write_pair(tmp_path, sdist_version="9.9.9")
    with pytest.raises(verifier.VerificationError, match="Name or Version mismatch"):
        verifier.inspect_artifacts(wheel, sdist)

@pytest.mark.parametrize("kind", ("wheel", "sdist"))
def test_noncanonical_distribution_filename_fails_closed(
    tmp_path: Path, kind: str
) -> None:
    wheel, sdist = _write_pair(tmp_path)
    original = wheel if kind == "wheel" else sdist
    suffix = ".whl" if kind == "wheel" else ".tar.gz"
    renamed = tmp_path / ("renamed" + suffix)
    renamed.write_bytes(original.read_bytes())
    checked_wheel = renamed if kind == "wheel" else wheel
    checked_sdist = renamed if kind == "sdist" else sdist
    with pytest.raises(verifier.VerificationError, match="canonical filename"):
        verifier.inspect_artifacts(checked_wheel, checked_sdist)


def test_wheel_rejects_unexpected_top_level_python(tmp_path: Path) -> None:
    wheel, sdist = _write_pair(tmp_path)
    with zipfile.ZipFile(wheel, "a") as archive:
        archive.writestr("shadow_cap.py", b"raise RuntimeError('unexpected')\n")
    with pytest.raises(verifier.VerificationError, match="wheel inventory"):
        verifier.inspect_artifacts(wheel, sdist)

@pytest.mark.parametrize(
    "relative",
    ("cap/worker.py", "cap/py.typed", "README.md", "LICENSE", "pyproject.toml"),
)
def test_source_backed_bytes_fail_closed(tmp_path: Path, relative: str) -> None:
    wheel, sdist = _write_pair(tmp_path, replacements={relative: b"tampered\n"})
    with pytest.raises(verifier.VerificationError, match="differs from immutable SDK source"):
        verifier.inspect_artifacts(wheel, sdist)

@pytest.mark.parametrize(
    ("kind", "relative"),
    (
        ("wheel", "cap/payload.bin"),
        ("wheel", f"cap_sdk_python-{PROJECT_VERSION}.dist-info/extra.json"),
        ("sdist", "scripts/backdoor.sh"),
        ("sdist", "tests/test_backdoor.py"),
    ),
)
def test_unexpected_inventory_fails_closed(tmp_path: Path, kind: str, relative: str) -> None:
    extras = {relative: b"unexpected\n"}
    wheel, sdist = _write_pair(
        tmp_path,
        extra_wheel=extras if kind == "wheel" else None,
        extra_sdist=extras if kind == "sdist" else None,
    )
    with pytest.raises(verifier.VerificationError, match=f"{kind} inventory"):
        verifier.inspect_artifacts(wheel, sdist)

@pytest.mark.parametrize(
    ("relative", "replacement", "message"),
    (
        ("setup.cfg", b"[metadata]\nname = injected\n", "setup.cfg"),
        ("METADATA", _metadata(PROJECT_VERSION, _source_bytes("README.md")).replace(
            b"Requires-Python: >=3.9", b"Requires-Python: >=3.8"), "Requires-Python"),
        ("METADATA", _metadata(PROJECT_VERSION, _source_bytes("README.md")).replace(
            b"protobuf<7,>=6.31.1", b"protobuf>=1"), "Requires-Dist"),
    ),
)
def test_build_metadata_is_bound_to_pyproject(
    tmp_path: Path, relative: str, replacement: bytes, message: str
) -> None:
    wheel, sdist = _write_pair(tmp_path, replacements={relative: replacement})
    with pytest.raises(verifier.VerificationError, match=message):
        verifier.inspect_artifacts(wheel, sdist)


def _completed(args: Tuple[str, ...], stdout: str = "") -> object:
    import subprocess

    return subprocess.CompletedProcess(args, 0, stdout=stdout, stderr="")


def test_consumer_installs_and_reports_exact_generated_code_floors(tmp_path: Path) -> None:
    calls = []

    def fake_run(args: Tuple[str, ...], cwd: Path, timeout: int) -> object:
        del cwd, timeout
        calls.append(tuple(args))
        evidence = {
            "imports": len(verifier.EXPECTED_IMPORTS),
            "protobuf_version": "6.31.1",
            "grpcio_version": "1.76.0",
            "worker_trust": {
                "challenge_bytes": 321,
                "mode": "enforce",
                "protocol_version": 1,
                "sender_id": "artifact-worker",
                "timeout_operational": True,
                "legacy_versions_rejected": True,
                "static_token_compatibility": True,
            },
        }
        stdout = json.dumps(evidence) if str(verifier.SMOKE_SCRIPT) in args else ""
        return _completed(tuple(args), stdout)

    with patch.object(verifier, "_run", side_effect=fake_run), patch.object(
        verifier, "_venv_python", return_value=Path(sys.executable)
    ):
        report = verifier._consumer_check(tmp_path / "artifact.whl", Path(sys.executable), 10)
    floor_command = ("protobuf==6.31.1", "grpcio==1.76.0")
    assert any(call[-2:] == floor_command for call in calls)
    assert report["dependency_versions"] == {"grpcio": "1.76.0", "protobuf": "6.31.1"}


def test_consumer_rejects_resolved_version_above_floor(tmp_path: Path) -> None:
    def fake_run(args: Tuple[str, ...], cwd: Path, timeout: int) -> object:
        del cwd, timeout
        evidence = {
            "imports": len(verifier.EXPECTED_IMPORTS),
            "protobuf_version": "6.31.2",
            "grpcio_version": "1.76.0",
        }
        stdout = json.dumps(evidence) if str(verifier.SMOKE_SCRIPT) in args else ""
        return _completed(tuple(args), stdout)

    with patch.object(verifier, "_run", side_effect=fake_run), patch.object(
        verifier, "_venv_python", return_value=Path(sys.executable)
    ), pytest.raises(verifier.VerificationError, match="protobuf exact floor"):
        verifier._consumer_check(tmp_path / "artifact.whl", Path(sys.executable), 10)


def test_cli_emits_stable_json_for_exact_paths(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    wheel, sdist = _write_pair(tmp_path)
    evidence = {"imports": 14, "pip_check": "ok", "source_shadow": "absent"}
    arguments = ("--wheel", str(wheel), "--sdist", str(sdist), "--python", sys.executable)
    with patch.object(verifier, "_consumer_check", return_value=evidence):
        assert verifier.main(arguments) == 0
        first = capsys.readouterr().out
        assert verifier.main(arguments) == 0
        second = capsys.readouterr().out
    assert first == second
    report = json.loads(first)
    assert report["schema"] == "cap-python-artifact-verification/v1"
    assert report["generated_modules"] == list(verifier.GENERATED_IMPORTS)
    assert report["imported_modules"] == list(verifier.EXPECTED_IMPORTS)
    assert report["consumer_checks"] == {"sdist": evidence, "wheel": evidence}


def test_cli_rejects_non_exact_artifact_path(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    wheel, sdist = _write_pair(tmp_path)
    wrong = tmp_path / "renamed.zip"
    wrong.write_bytes(wheel.read_bytes())
    assert verifier.main(("--wheel", str(wrong), "--sdist", str(sdist))) == 1
    error = capsys.readouterr().err
    assert "artifact verification failed" in error
    assert "exact .whl file" in error


def test_command_failure_reports_exit_and_command(tmp_path: Path) -> None:
    command = (sys.executable, "-c", "raise SystemExit(7)")
    with pytest.raises(verifier.VerificationError, match=r"command failed \(7\)"):
        verifier._run(command, tmp_path, 10)
