#!/usr/bin/env python
"""Pinned, fail-closed regeneration for every checked-in CAP protobuf surface."""

from __future__ import annotations

import argparse
import difflib
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
    from tools.codegen_toolchain import Toolchain, ToolchainError, describe, select
    from tools.python_proto_codegen import PythonCodegenError, generate_python_outputs, validate_python_pins
except ModuleNotFoundError:  # Direct execution: python tools/proto_codegen.py
    from codegen_toolchain import Toolchain, ToolchainError, describe, select
    from python_proto_codegen import PythonCodegenError, generate_python_outputs, validate_python_pins

REPO_ROOT = Path(__file__).resolve().parents[1]
PROTO_ROOT = REPO_ROOT / "proto" / "cordum" / "agent" / "v1"
BUF_TEMPLATE = REPO_ROOT / "buf.gen.yaml"
BUF_CONFIG = REPO_ROOT / "buf.yaml"
NODE_TOOLS = REPO_ROOT / "tools" / "codegen"
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


def _normalized_lines(path: Path) -> list[str]:
    """Line-ending-independent view of a generated file.

    Windows checkouts store these with CRLF; the container generates LF. Only
    the content difference is drift.
    """
    text = path.read_bytes().replace(b"\r\n", b"\n").decode("utf-8", errors="replace")
    return text.splitlines(keepends=True)


def diff_outputs(tracked: Path, generated: Path) -> str:
    """Empty when the two files agree, otherwise a readable unified diff.

    Deliberately does not shell out to git: this runs inside the hermetic
    container, where /src/.git is a worktree pointer to a host path that does
    not exist, and `git diff --no-index` aborts before comparing anything.
    """
    if not tracked.is_file():
        return f"missing tracked output: {tracked}\n"
    before, after = _normalized_lines(tracked), _normalized_lines(generated)
    if before == after:
        return ""
    diff = difflib.unified_diff(before, after, fromfile=str(tracked), tofile=str(generated), n=3)
    return "".join(diff)[:8000]


def compare_tracked(generated: Path, expected: Mapping[str, set[Path]], tracked: Path = REPO_ROOT) -> None:
    drifts: list[str] = []
    for name, outputs in expected.items():
        language = LANGUAGES[name]
        source = generated / language.generated_subdir
        target = tracked / language.tracked_subdir
        validate_inventory(target, outputs, language, "tracked")
        for output in sorted(outputs, key=str):
            diff = diff_outputs(target / output, source / output)
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

def run_schema_checks(tools: Toolchain) -> None:
    _run(tools.buf("lint"))
    _run(tools.buf("build"))
    if not tools.checks_breaking:
        return
    _run(["git", "rev-parse", "--verify", "origin/main^{commit}"])
    _run(tools.buf("breaking", "--against", ".git#branch=origin/main", "--against-config", str(BUF_CONFIG)))

def ensure_node_tools(tools: Toolchain) -> None:
    validate_node_pins()
    if tools.installs_node_modules:
        _run(["npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund"], NODE_TOOLS)


def run_buf_generate(destination: Path, tools: Toolchain) -> None:
    _run(tools.buf("generate", "--template", str(tools.template), "--output", str(destination)))


def run_node_bundle(destination: Path, protos: Sequence[Path], tools: Toolchain) -> None:
    binary = tools.protobufjs_bin
    output = destination / "node"
    output.mkdir(parents=True, exist_ok=True)
    roots = [str(path) for path in aggregate_roots(protos)]
    _run(["node", str(binary / "pbjs"), "-p", str(PROTO_ROOT.parents[2]), "-t", "static-module", "-w", "commonjs", "-r", "cordum.agent.v1", "-o", str(output / "cap_pb.js"), *roots])
    _run(["node", str(binary / "pbts"), "-o", str(output / "cap_pb.d.ts"), str(output / "cap_pb.js")])


def generate_once(destination: Path, protos: Sequence[Path], expected: Mapping[str, set[Path]], tools: Toolchain) -> None:
    if destination.exists():
        shutil.rmtree(destination)
    run_buf_generate(destination, tools)
    generate_python_outputs(destination, protos, _run)
    run_node_bundle(destination, protos, tools)
    validate_generated(destination, expected)


def execute(mode: str, tools: Toolchain) -> dict[str, object]:
    # The canonical remote pins are validated on both paths: the hermetic
    # template mirrors them, so a drifted //buf.gen.yaml must fail either way.
    validate_remote_pins(BUF_TEMPLATE)
    validate_node_pins()
    validate_python_pins()
    run_schema_checks(tools)
    ensure_node_tools(tools)
    protos = proto_sources()
    expected = expected_outputs(protos)
    with tempfile.TemporaryDirectory(prefix="cap-codegen-") as temporary:
        destination = Path(temporary).resolve() / "generated"
        generate_once(destination, protos, expected, tools)
        before = snapshot_generated(destination, expected)
        generate_once(destination, protos, expected, tools)
        assert_idempotent(before, snapshot_generated(destination, expected))
        compare_tracked(destination, expected) if mode == "check" else write_tracked(destination, expected)
    report = {"mode": mode, "languages": sorted(expected), "outputs": sum(map(len, expected.values()))}
    report.update(describe(tools))
    return report


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument("--check", action="store_const", const="check", dest="mode")
    modes.add_argument("--write", action="store_const", const="write", dest="mode")
    parser.add_argument(
        "--offline",
        action="store_true",
        help="use the hermetic toolchain (local buf plugins, preinstalled Node modules)",
    )
    arguments = parser.parse_args(argv)
    try:
        report = execute(arguments.mode, select(arguments.offline))
    except (CodegenError, PythonCodegenError, ToolchainError, json.JSONDecodeError, UnicodeError) as exc:
        print(f"CAP proto codegen failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
