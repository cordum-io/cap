"""Build a deterministic, allowlisted local Go module proxy for CAP onboarding."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path, PurePosixPath
from typing import NamedTuple, Sequence


MODULE = "github.com/cordum-io/cap/v2"
ZIP_TIME = (1980, 1, 1, 0, 0, 0)
REQUIRED_FILES = frozenset({"LICENSE", "go.mod", "go.sum"})
SOURCE_PREFIXES = ("cordum/agent/v1/", "sdk/go/")
VERSION_PATTERN = re.compile(
    r"^v2\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-"
    r"([0-9a-z-]+(?:\.[0-9a-z-]+)*)$"
)


class BuildError(RuntimeError):
    """An expected, user-actionable proxy build failure."""


class GitEntry(NamedTuple):
    mode: str
    kind: str
    object_id: str
    path: PurePosixPath


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path.cwd())
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--version", required=True)
    return parser.parse_args()


def validate_version(version: str) -> None:
    match = VERSION_PATTERN.fullmatch(version)
    if match is None:
        raise BuildError("version must be a valid v2 prerelease without build metadata")
    prerelease = match.group(3).split(".")
    if any(part.isdigit() and len(part) > 1 and part.startswith("0") for part in prerelease):
        raise BuildError("version prerelease numeric identifiers cannot have leading zeros")


def git_tree_entries(root: Path) -> list[GitEntry]:
    try:
        result = subprocess.run(
            ["git", "ls-tree", "-r", "-z", "--full-tree", "HEAD", "--"],
            cwd=root,
            capture_output=True,
            timeout=15,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise BuildError(f"cannot list tracked files: {error}") from error
    if result.returncode != 0:
        message = result.stderr.decode("utf-8", errors="replace").strip()
        raise BuildError(f"cannot list tracked files: {message or 'git failed'}")
    try:
        records = result.stdout.split(b"\0")
        entries = []
        for record in records:
            if not record:
                continue
            header, raw_path = record.split(b"\t", 1)
            mode, kind, object_id = header.decode("ascii").split()
            entries.append(
                GitEntry(mode, kind, object_id, PurePosixPath(raw_path.decode("utf-8")))
            )
    except (UnicodeDecodeError, ValueError) as error:
        raise BuildError("git index entries are malformed or not valid UTF-8") from error
    return entries


def is_allowed(relative: PurePosixPath) -> bool:
    name = relative.as_posix()
    if name in REQUIRED_FILES:
        return True
    return (
        name.endswith(".go")
        and not name.endswith("_test.go")
        and name.startswith(SOURCE_PREFIXES)
    )


def validate_sources(root: Path) -> list[GitEntry]:
    entries = sorted(
        (entry for entry in git_tree_entries(root) if is_allowed(entry.path)),
        key=lambda entry: entry.path.as_posix(),
    )
    for entry in entries:
        if entry.kind != "blob" or entry.mode not in {"100644", "100755"}:
            raise BuildError(
                f"version source has unsupported Git type/mode: "
                f"{entry.kind}/{entry.mode} {entry.path}"
            )
    names = [entry.path.as_posix() for entry in entries]
    missing_required = sorted(REQUIRED_FILES.difference(names))
    if missing_required:
        raise BuildError(f"missing tracked source: {', '.join(missing_required)}")
    if not any(name.startswith("cordum/agent/v1/") for name in names):
        raise BuildError("missing tracked source under cordum/agent/v1")
    if not any(name.startswith("sdk/go/") for name in names):
        raise BuildError("missing tracked source under sdk/go")
    if len({name.casefold() for name in names}) != len(names):
        raise BuildError("tracked source paths collide when compared case-insensitively")
    return entries


def validate_clean_sources(root: Path) -> None:
    command = [
        "git", "diff", "--name-only", "-z", "HEAD", "--",
        *sorted(REQUIRED_FILES), *[prefix.rstrip("/") for prefix in SOURCE_PREFIXES],
    ]
    try:
        result = subprocess.run(command, cwd=root, capture_output=True, timeout=15, check=False)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise BuildError(f"cannot compare version sources: {error}") from error
    if result.returncode != 0:
        message = result.stderr.decode("utf-8", errors="replace").strip()
        raise BuildError(f"cannot compare version sources: {message or 'git failed'}")
    changed = [
        PurePosixPath(raw.decode("utf-8"))
        for raw in result.stdout.split(b"\0") if raw
    ]
    packaged_changes = sorted(path.as_posix() for path in changed if is_allowed(path))
    if packaged_changes:
        raise BuildError("tracked source differs from HEAD: " + ", ".join(packaged_changes))


def git_blob(root: Path, object_id: str) -> bytes:
    try:
        result = subprocess.run(
            ["git", "cat-file", "blob", object_id], cwd=root,
            capture_output=True, timeout=15, check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise BuildError(f"cannot read version source: {error}") from error
    if result.returncode != 0:
        message = result.stderr.decode("utf-8", errors="replace").strip()
        raise BuildError(f"cannot read version source: {message or 'git failed'}")
    return result.stdout


def validate_module(module_bytes: bytes) -> bytes:
    try:
        module_text = module_bytes.decode("utf-8")
    except UnicodeDecodeError as error:
        raise BuildError("go.mod must be valid UTF-8") from error
    declarations = re.findall(r"^module\s+(\S+)\s*$", module_text, re.MULTILINE)
    if declarations != [MODULE]:
        raise BuildError(f"go.mod must declare exactly: module {MODULE}")
    return module_bytes


def write_metadata(version_dir: Path, version: str, module_bytes: bytes) -> None:
    version_dir.mkdir(parents=True)
    (version_dir / "list").write_bytes((version + "\n").encode("utf-8"))
    info = json.dumps(
        {"Version": version},
        separators=(",", ":"),
        sort_keys=True,
    )
    (version_dir / f"{version}.info").write_bytes((info + "\n").encode("utf-8"))
    (version_dir / f"{version}.mod").write_bytes(module_bytes)


def write_module_zip(
    destination: Path, root: Path, version: str, sources: Sequence[GitEntry]
) -> None:
    prefix = f"{MODULE}@{version}/"
    with zipfile.ZipFile(destination, "w", compression=zipfile.ZIP_STORED) as archive:
        for entry in sources:
            relative = entry.path
            info = zipfile.ZipInfo(prefix + relative.as_posix(), ZIP_TIME)
            info.create_system = 3
            info.external_attr = 0o100644 << 16
            info.compress_type = zipfile.ZIP_STORED
            archive.writestr(info, git_blob(root, entry.object_id))


def ensure_available_output(output: Path) -> bool:
    if output.is_symlink():
        raise BuildError(f"output must not be a symlink: {output}")
    if not output.exists():
        return False
    if not output.is_dir():
        raise BuildError(f"output must be an absent or empty directory: {output}")
    if any(output.iterdir()):
        raise BuildError(f"output directory must be empty: {output}")
    return True


def publish_staged(staged: Path, output: Path, output_existed: bool) -> None:
    if output_existed:
        output.rmdir()
    try:
        os.replace(staged, output)
    except OSError:
        if output_existed and not output.exists():
            output.mkdir()
        raise


def build_proxy(root: Path, output: Path, version: str) -> int:
    validate_version(version)
    root = root.resolve(strict=True)
    sources = validate_sources(root)
    validate_clean_sources(root)
    module_entry = next(entry for entry in sources if entry.path.as_posix() == "go.mod")
    module_bytes = validate_module(git_blob(root, module_entry.object_id))
    output = Path(os.path.abspath(output))
    output_existed = ensure_available_output(output)
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".cap-go-proxy-", dir=output.parent) as temporary:
        staged = Path(temporary) / "proxy"
        version_dir = staged / MODULE / "@v"
        write_metadata(version_dir, version, module_bytes)
        write_module_zip(version_dir / f"{version}.zip", root, version, sources)
        publish_staged(staged, output, output_existed)
    return len(sources)


def main() -> int:
    arguments = parse_args()
    try:
        count = build_proxy(arguments.root, arguments.output, arguments.version)
    except (BuildError, OSError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"built {arguments.version} with {count} tracked files at {arguments.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
