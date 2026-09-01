"""Verify CAP's released Go quickstart from an isolated module consumer."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Mapping, Optional, Sequence

if __package__:
    from .process_runner import CommandFailure, RunResult, run_command
else:
    from process_runner import CommandFailure, RunResult, run_command

CAP_MODULE = "github.com/cordum-io/cap/v2"
OFFICIAL_PROXY = "https://proxy.golang.org"
OFFICIAL_SUMDB = "sum.golang.org"
COMMAND_TIMEOUT_SECONDS = 180
RUN_TIMEOUT_SECONDS = 45
STABLE_V2 = re.compile(r"^2\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$")
CLEARED_GO_ENV = (
    "AR", "CC", "CXX", "GOCACHEPROG", "GODEBUG", "GOEXPERIMENT", "GOFLAGS",
    "GOOS", "GOARCH", "GOINSECURE", "GONOPROXY", "GONOSUMDB", "GOPRIVATE",
    "GOROOT", "PKG_CONFIG",
)

class ManifestError(ValueError):
    """The release manifest cannot authorize this verification."""
class VerificationError(RuntimeError):
    """The installed-artifact proof failed closed."""
@dataclass(frozen=True)
class ReleaseConfig:
    module: str
    version: str

Runner = Callable[[Sequence[str], Path, Mapping[str, str], int], RunResult]

def _mapping(value: object, label: str) -> Mapping[str, object]:
    if not isinstance(value, dict):
        raise ManifestError(f"{label} must be an object")
    return value

def _text_field(value: Mapping[str, object], field: str, label: str) -> str:
    result = value.get(field)
    if not isinstance(result, str) or not result:
        raise ManifestError(f"{label}.{field} must be a non-empty string")
    return result

def _unique_object(pairs: list[tuple[str, object]]) -> dict[str, object]:
    result: dict[str, object] = {}
    for key, value in pairs:
        if key in result:
            raise ManifestError(f"duplicate manifest key {key!r}")
        result[key] = value
    return result

def load_release_config(path: Path) -> ReleaseConfig:
    try:
        document = json.loads(path.read_text(encoding="utf-8"), object_pairs_hook=_unique_object)
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise ManifestError(f"cannot read release manifest {path}: {error}") from error
    root = _mapping(document, "manifest")
    schema = _text_field(root, "schemaVersion", "manifest")
    if schema not in {"1.0.0", "1.1.0", "1.2.0"}:
        raise ManifestError(f"unsupported manifest schemaVersion {schema!r}")
    release = _mapping(root.get("release"), "manifest.release")
    version = _text_field(release, "version", "manifest.release")
    if not STABLE_V2.fullmatch(version):
        raise ManifestError(f"release version {version!r} is not stable v2")
    if _text_field(release, "tag", "manifest.release") != f"v{version}":
        raise ManifestError("manifest.release.tag must equal v + release.version")
    if _text_field(release, "channel", "manifest.release") != "stable":
        raise ManifestError("manifest.release.channel must be stable")
    component = _published_go_component(root, version)
    return ReleaseConfig(_text_field(component, "package", "cap-go"), version)

def _published_go_component(root: Mapping[str, object], version: str) -> Mapping[str, object]:
    raw = root.get("components")
    if not isinstance(raw, list):
        raise ManifestError("manifest.components must be an array")
    matches = [item for item in raw if isinstance(item, dict) and item.get("name") == "cap-go"]
    if len(matches) != 1:
        raise ManifestError("manifest must contain exactly one cap-go component")
    component = matches[0]
    expected = {
        "kind": "sdk",
        "tier": "stable",
        "language": "go",
        "package": CAP_MODULE,
        "registry": "proxy.golang.org",
        "version": version,
        "publication": "published",
    }
    for field, wanted in expected.items():
        if component.get(field) != wanted:
            raise ManifestError(f"cap-go.{field} must equal {wanted!r}")
    return component

def build_go_environment(root: Path, inherited: Optional[Mapping[str, str]] = None) -> dict[str, str]:
    source = os.environ if inherited is None else inherited
    environment = dict(source)
    environment.update(
        {
            "GOMODCACHE": str(root / "module-cache"),
            "GOCACHE": str(root / "build-cache"),
            "GOPATH": str(root / "gopath"),
            "GOTMPDIR": str(root / "go-tmp"),
            "GOPROXY": OFFICIAL_PROXY,
            "GOSUMDB": OFFICIAL_SUMDB,
            "GOWORK": "off",
            "GOTOOLCHAIN": "local",
            "GOENV": "off",
            "GOAUTH": "off",
            "GOTELEMETRY": "off",
            "GOVCS": "*:off",
            "CGO_ENABLED": "0",
            "GO111MODULE": "on",
        }
    )
    for name in CLEARED_GO_ENV:
        environment[name] = ""
    return environment

def _command_label(argv: Sequence[str]) -> str:
    return " ".join(repr(part) for part in argv)


def _require_success(result: RunResult) -> RunResult:
    if result.returncode != 0:
        output = result.output.strip().replace("\r", "").replace("\n", " | ")
        raise CommandFailure(f"command exit {result.returncode}: {_command_label(result.argv)}: {output}")
    return result

def run_checked(argv: Sequence[str], cwd: Path, env: Mapping[str, str], timeout: int) -> RunResult:
    return _require_success(run_command(argv, cwd, env, timeout))

def _json_objects(text: str) -> list[Mapping[str, object]]:
    decoder = json.JSONDecoder()
    offset = 0
    objects: list[Mapping[str, object]] = []
    while offset < len(text):
        while offset < len(text) and text[offset].isspace():
            offset += 1
        if offset == len(text):
            break
        try:
            value, offset = decoder.raw_decode(text, offset)
        except json.JSONDecodeError as error:
            raise VerificationError(f"malformed Go module JSON: {error}") from error
        if not isinstance(value, dict):
            raise VerificationError("Go module JSON record must be an object")
        objects.append(value)
    return objects

def _within(path: Path, root: Path) -> bool:
    try:
        path.resolve().relative_to(root.resolve())
        return True
    except ValueError:
        return False

def validate_module_graph(
    edit_output: str, graph_output: str, release: ReleaseConfig,
    source_root: Path, module_cache: Path,
) -> None:
    edit_records = _json_objects(edit_output)
    if len(edit_records) != 1 or edit_records[0].get("Replace"):
        raise VerificationError("consumer go.mod must not contain Replace directives")
    matches = [record for record in _json_objects(graph_output) if record.get("Path") == release.module]
    if len(matches) != 1:
        raise VerificationError("module graph must contain exactly one CAP module")
    cap = matches[0]
    if cap.get("Version") != f"v{release.version}":
        raise VerificationError("CAP module graph version differs from release manifest")
    if cap.get("Replace"):
        raise VerificationError("CAP module graph must not contain a replacement")
    directory = cap.get("Dir")
    if not isinstance(directory, str) or not directory:
        raise VerificationError("CAP module graph has no resolved module directory")
    resolved = Path(directory)
    if not _within(resolved, module_cache) or _within(resolved, source_root):
        raise VerificationError("CAP module resolved outside the isolated module cache")

def prove_missing_grpc_sums(sum_path: Path, build: Callable[[], RunResult]) -> None:
    original = sum_path.read_bytes()
    lines = original.splitlines(keepends=True)
    targets = [line for line in lines if line.startswith(b"google.golang.org/grpc ")]
    if len(targets) < 2 or not any(b"/go.mod " in line for line in targets):
        raise VerificationError("go.sum lacks grpc module and go.mod mutation targets")
    sum_path.write_bytes(b"".join(line for line in lines if line not in targets))
    try:
        result = build()
        output = result.output.lower()
        if result.returncode == 0:
            raise VerificationError("readonly build unexpectedly passed without grpc sums")
        if "missing go.sum entry" not in output or "google.golang.org/grpc" not in output:
            raise VerificationError("mutation failed for a reason other than missing grpc sums")
    finally:
        sum_path.write_bytes(original)
    if sum_path.read_bytes() != original:
        raise VerificationError("go.sum was not restored byte-for-byte")

def _resolve_go(
    explicit: Optional[str], forbidden_roots: Sequence[Path], path_value: Optional[str] = None,
) -> str:
    if explicit:
        candidates = [Path(explicit)]
    else:
        name = "go.exe" if os.name == "nt" else "go"
        search = os.environ.get("PATH", "") if path_value is None else path_value
        candidates = [Path(entry) / name for entry in search.split(os.pathsep) if entry]
    for candidate in candidates:
        resolved = candidate.resolve()
        executable = resolved.is_file() and (os.name == "nt" or os.access(resolved, os.X_OK))
        if executable and not any(_within(resolved, root) for root in forbidden_roots):
            return str(resolved)
    raise VerificationError("go executable is unavailable outside verification source and temp roots")

def _source_root(manifest: Path) -> Path:
    parent = manifest.resolve().parent
    return parent.parent if parent.name == "release" else parent

def verify_consumer(
    manifest: Path, example: Path, run_mode: bool, *, runner: Optional[Runner] = None,
    go_executable: Optional[str] = None,
) -> None:
    release = load_release_config(manifest)
    if not example.is_file() or example.is_symlink():
        raise VerificationError(f"Go quickstart example is not a regular file: {example}")
    execute = run_command if runner is None else runner
    with tempfile.TemporaryDirectory(prefix="cap-registry-verify-") as temporary:
        root = Path(temporary)
        source_root = _source_root(manifest)
        go = _resolve_go(go_executable, (source_root, root))
        consumer = root / "consumer with spaces"
        consumer.mkdir()
        shutil.copyfile(example, consumer / "main.go")
        environment = build_go_environment(root)
        Path(environment["GOTMPDIR"]).mkdir()

        def checked(*args: str, timeout: int = COMMAND_TIMEOUT_SECONDS) -> RunResult:
            return _require_success(execute((go, *args), consumer, environment, timeout))

        checked("mod", "init", "example.com/cap-registry-verify")
        checked("mod", "edit", f"-require={release.module}@v{release.version}")
        checked("mod", "tidy")
        checked("mod", "verify")
        edit = checked("mod", "edit", "-json").output
        graph = checked("list", "-m", "-json", "all").output
        validate_module_graph(edit, graph, release, source_root, Path(environment["GOMODCACHE"]))
        print(f"registry-go-consumer: module-graph=OK module={release.module}@v{release.version} replace=none source=isolated-cache")
        checked("build", "-mod=readonly", ".")
        mutation_environment = dict(environment)
        mutation_environment["GOCACHE"] = str(root / "mutation-build-cache")
        prove_missing_grpc_sums(
            consumer / "go.sum",
            lambda: execute((go, "build", "-mod=readonly", "."), consumer, mutation_environment, COMMAND_TIMEOUT_SECONDS),
        )
        print("registry-go-consumer: grpc-sum-mutation=REJECTED_AND_RESTORED")
        checked("mod", "verify")
        print("registry-go-consumer: go-mod-verify=OK")
        checked("build", "-mod=readonly", ".")
        print("registry-go-consumer: readonly-build=OK")
        if run_mode:
            result = checked("run", "-mod=readonly", ".", timeout=RUN_TIMEOUT_SECONDS)
            if "quickstart-echo: OK" not in result.output.splitlines():
                raise VerificationError("quickstart run omitted its success marker")
            print("quickstart-echo: OK")
        print(f"registry-go-consumer: OK module={release.module}@v{release.version} mode={'run' if run_mode else 'build'}")

def parse_args(argv: Optional[Sequence[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--example", required=True, type=Path)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--build-only", action="store_true")
    mode.add_argument("--run", action="store_true")
    return parser.parse_args(argv)

def main(argv: Optional[Sequence[str]] = None) -> int:
    arguments = parse_args(argv)
    try:
        verify_consumer(arguments.manifest, arguments.example, bool(arguments.run))
    except (ManifestError, VerificationError, CommandFailure, OSError) as error:
        print(f"registry-go-consumer: FAIL: {error}", file=sys.stderr)
        return 1
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
