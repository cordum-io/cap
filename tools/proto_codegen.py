#!/usr/bin/env python
"""Pinned, fail-closed regeneration for every checked-in CAP protobuf surface."""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
import tempfile
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Mapping, Sequence

try:
    from tools.python_proto_codegen import PythonCodegenError, generate_python_outputs, validate_python_pins
except ModuleNotFoundError:  # Direct execution: python tools/proto_codegen.py
    from python_proto_codegen import PythonCodegenError, generate_python_outputs, validate_python_pins

REPO_ROOT = Path(__file__).resolve().parents[1]
PROTO_ROOT = REPO_ROOT / "proto" / "cordum" / "agent" / "v1"
BUF_TEMPLATE = REPO_ROOT / "buf.gen.yaml"
BUF_CONFIG = REPO_ROOT / "buf.yaml"
NODE_TOOLS = REPO_ROOT / "tools" / "codegen"
BUF_TOOL = "github.com/bufbuild/buf/cmd/buf@v1.71.0"
TIMEOUT_SECONDS = 600
NODE_DEPENDENCIES = {
    "@types/google-protobuf": "3.15.12",
    "google-protobuf": "3.21.4",
    "protobufjs": "7.6.3",
    "protobufjs-cli": "1.3.3",
    "ts-protoc-gen": "0.15.0",
    "typescript": "5.9.3",
}
REMOTE_PLUGINS = (
    "buf.build/protocolbuffers/go:v1.36.11",
    "buf.build/grpc/go:v1.6.0",
    "buf.build/protocolbuffers/cpp:v21.12",
    "buf.build/protocolbuffers/js:v3.21.4",
    "buf.build/protocolbuffers/ruby:v33.2",
)

class CodegenError(RuntimeError):
    """A reproducibility or generated-artifact contract violation."""
@dataclass(frozen=True)
class Language:
    name: str
    generated_subdir: Path
    tracked_subdir: Path
    suffixes: tuple[str, ...]
LANGUAGES = {
    "go": Language("go", Path("go/cordum/agent/v1"), Path("cordum/agent/v1"), (".pb.go",)),
    "cpp": Language("cpp", Path("cpp/cordum/agent/v1"), Path("cpp/cordum/agent/v1"), (".pb.cc", ".pb.h")),
    "node": Language("node", Path("node"), Path("node"), ("_pb.js", "_pb.d.ts")),
    "python": Language("python", Path("python/cordum/agent/v1"), Path("python/cordum/agent/v1"), ("_pb2.py", "_pb2_grpc.py")),
    "sdk-python": Language("sdk-python", Path("sdk-python/cordum/agent/v1"), Path("sdk/python/cap/pb/cordum/agent/v1"), ("_pb2.py", "_pb2_grpc.py")),
    "ruby": Language("ruby", Path("ruby/cordum/agent/v1"), Path("sdk/ruby/proto/cordum/agent/v1"), ("_pb.rb",)),
}
def require(condition: bool, message: str) -> None:
    if not condition:
        raise CodegenError(message)

def proto_sources(root: Path = PROTO_ROOT) -> tuple[Path, ...]:
    require(root.is_dir(), f"canonical proto directory is missing: {root}")
    protos = tuple(sorted(root.glob("*.proto"), key=lambda path: path.name))
    require(bool(protos), f"no canonical protos found under {root}")
    return protos

def _has_service(proto: Path) -> bool:
    return bool(re.search(r"^\s*service\s+\w+", proto.read_text(encoding="utf-8"), re.MULTILINE))

def ruby_outputs(stems: Sequence[str]) -> set[Path]:
    return {Path(f"{stem}_pb.rb") for stem in stems}

def expected_outputs(protos: Sequence[Path]) -> dict[str, set[Path]]:
    stems = tuple(proto.stem for proto in protos)
    services = {proto.stem for proto in protos if _has_service(proto)}
    python = {Path(f"{stem}_{kind}.py") for stem in stems for kind in ("pb2", "pb2_grpc")}
    node = {Path("cordum/agent/v1") / f"{stem}_pb{ext}" for stem in stems for ext in (".js", ".d.ts")}
    node.update({Path("cap_pb.js"), Path("cap_pb.d.ts")})
    go = {Path(f"{stem}.pb.go") for stem in stems}
    go.update(Path(f"{stem}_grpc.pb.go") for stem in services)
    return {
        "go": go,
        "cpp": {Path(f"{stem}.pb{ext}") for stem in stems for ext in (".cc", ".h")},
        "node": node,
        "python": set(python),
        "sdk-python": set(python),
        "ruby": ruby_outputs(stems),
    }


def aggregate_roots(protos: Sequence[Path]) -> tuple[Path, ...]:
    imported: set[str] = set()
    pattern = re.compile(r'import\s+"cordum/agent/v1/([^"/]+)\.proto"')
    for proto in protos:
        imported.update(pattern.findall(proto.read_text(encoding="utf-8")))
    roots = tuple(proto for proto in protos if proto.stem not in imported)
    require(bool(roots), "cannot determine Node aggregate roots (import cycle?)")
    return roots


def _inventory(root: Path, language: Language) -> set[Path]:
    if not root.is_dir():
        return set()
    return {
        path.relative_to(root)
        for path in root.rglob("*")
        if path.is_file() and path.name.endswith(language.suffixes)
    }

def validate_inventory(root: Path, expected: set[Path], language: Language, kind: str) -> None:
    require(root.is_dir(), f"language {language.name} was skipped: missing {kind} root {root}")
    actual = _inventory(root, language)
    missing = sorted(str(path) for path in expected - actual)
    extra = sorted(str(path) for path in actual - expected)
    require(not missing and not extra, f"{kind} {language.name} inventory mismatch: missing={missing} extra={extra}")
    empty = sorted(str(path) for path in expected if not (root / path).read_bytes())
    require(not empty, f"{kind} {language.name} outputs are empty: {empty}")

def validate_generated(root: Path, expected: Mapping[str, set[Path]]) -> None:
    for name, outputs in expected.items():
        language = LANGUAGES[name]
        validate_inventory(root / language.generated_subdir, outputs, language, "generated")


def snapshot_generated(root: Path, expected: Mapping[str, set[Path]]) -> dict[str, dict[str, bytes]]:
    validate_generated(root, expected)
    snapshot: dict[str, dict[str, bytes]] = {}
    for name, outputs in expected.items():
        base = root / LANGUAGES[name].generated_subdir
        snapshot[name] = {str(path): (base / path).read_bytes() for path in outputs}
    return snapshot


def assert_idempotent(before: Mapping[str, Mapping[str, bytes]], after: Mapping[str, Mapping[str, bytes]]) -> None:
    require(before == after, "protobuf generation is not idempotent across clean runs")


def git_diff(tracked: Path, generated: Path) -> str:
    command = ["git", "diff", "--no-index", "--ignore-cr-at-eol", "--", str(tracked), str(generated)]
    result = subprocess.run(command, cwd=REPO_ROOT, text=True, capture_output=True, check=False)
    require(result.returncode in (0, 1), f"git diff failed ({result.returncode}): {result.stderr[-2000:]}")
    return (result.stdout + result.stderr)[-8000:] if result.returncode else ""


def compare_tracked(generated: Path, expected: Mapping[str, set[Path]], tracked: Path = REPO_ROOT) -> None:
    drifts: list[str] = []
    for name, outputs in expected.items():
        language = LANGUAGES[name]
        source = generated / language.generated_subdir
        target = tracked / language.tracked_subdir
        validate_inventory(target, outputs, language, "tracked")
        for output in sorted(outputs, key=str):
            diff = git_diff(target / output, source / output)
            if diff:
                drifts.append(diff)
    require(not drifts, "generated protobuf drift:\n" + "\n".join(drifts[:6]))


def write_tracked(generated: Path, expected: Mapping[str, set[Path]], tracked: Path = REPO_ROOT) -> None:
    validate_generated(generated, expected)
    for name, outputs in expected.items():
        language = LANGUAGES[name]
        source = generated / language.generated_subdir
        target = tracked / language.tracked_subdir
        target.mkdir(parents=True, exist_ok=True)
        for extra in _inventory(target, language) - outputs:
            (target / extra).unlink()
        for output in outputs:
            destination = target / output
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source / output, destination)


def validate_remote_pins(path: Path, expected: Sequence[str] = REMOTE_PLUGINS) -> None:
    require(path.is_file(), f"missing Buf generation template: {path}")
    lines = path.read_text(encoding="utf-8").splitlines()
    found: list[str] = []
    for index, line in enumerate(lines):
        match = re.match(r"\s*-\s+remote:\s+(\S+)\s*$", line)
        if not match:
            continue
        remote = match.group(1)
        require(bool(re.fullmatch(r"buf\.build/[\w.-]+/[\w.-]+:v[\w.-]+", remote)), f"remote plugin is not version-pinned: {remote}")
        indent = len(line) - len(line.lstrip())
        following: list[str] = []
        for candidate in lines[index + 1 :]:
            if re.match(rf"^\s{{{indent}}}-\s+", candidate):
                break
            following.append(candidate)
        block = "\n".join(following)
        require(bool(re.search(r"^\s+revision:\s+[1-9]\d*\s*$", block, re.MULTILINE)), f"remote plugin {remote} has no pinned revision")
        found.append(remote)
    require(Counter(found) == Counter(expected), f"unexpected remote plugin set: {found}")


def _read_dependencies(path: Path, nested: bool = False) -> dict[str, str]:
    require(path.is_file(), f"missing Node codegen lock file: {path}")
    data = json.loads(path.read_text(encoding="utf-8"))
    require(isinstance(data, dict), f"invalid JSON object: {path}")
    if nested:
        packages = data.get("packages")
        require(isinstance(packages, dict) and isinstance(packages.get(""), dict), f"invalid npm lock root: {path}")
        data = packages[""]
    dependencies = data.get("devDependencies")
    require(isinstance(dependencies, dict), f"missing devDependencies: {path}")
    return {str(name): str(version) for name, version in dependencies.items()}

def validate_node_pins(root: Path = NODE_TOOLS) -> None:
    manifest = _read_dependencies(root / "package.json")
    lock = _read_dependencies(root / "package-lock.json", nested=True)
    require(manifest == NODE_DEPENDENCIES and lock == NODE_DEPENDENCIES, f"unexpected Node codegen pins: manifest={manifest} lock={lock}")

def _run(command: Sequence[str], cwd: Path = REPO_ROOT) -> None:
    executable = [shutil.which(command[0]) or command[0], *command[1:]]
    try:
        result = subprocess.run(executable, cwd=cwd, text=True, capture_output=True, check=False, timeout=TIMEOUT_SECONDS)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise CodegenError(f"command could not run: {' '.join(command)}: {exc}") from exc
    if result.returncode:
        output = (result.stdout + "\n" + result.stderr).strip()[-8000:]
        raise CodegenError(f"command failed ({result.returncode}): {' '.join(command)}\n{output}")

def _buf(*arguments: str) -> list[str]:
    return ["go", "run", BUF_TOOL, *arguments]


def run_schema_checks() -> None:
    _run(_buf("lint"))
    _run(_buf("build"))
    _run(["git", "rev-parse", "--verify", "origin/main^{commit}"])
    _run(_buf("breaking", "--against", ".git#branch=origin/main", "--against-config", str(BUF_CONFIG)))

def ensure_node_tools() -> None:
    validate_node_pins()
    _run(["npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund"], NODE_TOOLS)


def run_buf_generate(destination: Path) -> None:
    _run(_buf("generate", "--template", str(BUF_TEMPLATE), "--output", str(destination)))


def run_node_bundle(destination: Path, protos: Sequence[Path]) -> None:
    binary = NODE_TOOLS / "node_modules" / "protobufjs-cli" / "bin"
    output = destination / "node"
    output.mkdir(parents=True, exist_ok=True)
    roots = [str(path) for path in aggregate_roots(protos)]
    _run(["node", str(binary / "pbjs"), "-p", str(PROTO_ROOT.parents[2]), "-t", "static-module", "-w", "commonjs", "-r", "cordum.agent.v1", "-o", str(output / "cap_pb.js"), *roots])
    _run(["node", str(binary / "pbts"), "-o", str(output / "cap_pb.d.ts"), str(output / "cap_pb.js")])


def generate_once(destination: Path, protos: Sequence[Path], expected: Mapping[str, set[Path]]) -> None:
    if destination.exists():
        shutil.rmtree(destination)
    run_buf_generate(destination)
    generate_python_outputs(destination, protos, _run)
    run_node_bundle(destination, protos)
    validate_generated(destination, expected)


def execute(mode: str) -> dict[str, object]:
    validate_remote_pins(BUF_TEMPLATE)
    validate_node_pins()
    validate_python_pins()
    run_schema_checks()
    ensure_node_tools()
    protos = proto_sources()
    expected = expected_outputs(protos)
    with tempfile.TemporaryDirectory(prefix="cap-codegen-") as temporary:
        destination = Path(temporary).resolve() / "generated"
        generate_once(destination, protos, expected)
        before = snapshot_generated(destination, expected)
        generate_once(destination, protos, expected)
        assert_idempotent(before, snapshot_generated(destination, expected))
        compare_tracked(destination, expected) if mode == "check" else write_tracked(destination, expected)
    return {"mode": mode, "languages": sorted(expected), "outputs": sum(map(len, expected.values())), "buf": BUF_TOOL}


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument("--check", action="store_const", const="check", dest="mode")
    modes.add_argument("--write", action="store_const", const="write", dest="mode")
    arguments = parser.parse_args(argv)
    try:
        report = execute(arguments.mode)
    except (CodegenError, PythonCodegenError, json.JSONDecodeError, UnicodeError) as exc:
        print(f"CAP proto codegen failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
