"""Synchronize canonical simple-echo sources into the Getting Started guide."""

from __future__ import annotations

import argparse
import os
import stat
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Final, Optional, Sequence, cast


GUIDE_PATH: Final = Path("docs/getting-started.md")
DEFAULT_ROOT: Final = Path(__file__).resolve().parents[2]


@dataclass(frozen=True)
class Snippet:
    label: str
    language: str
    source: Path


SNIPPETS: Final[tuple[Snippet, ...]] = (
    Snippet("go-worker", "go", Path("examples/simple-echo/go-worker/main.go")),
    Snippet("go-client", "go", Path("examples/simple-echo/go-client/main.go")),
    Snippet(
        "python-worker",
        "python",
        Path("examples/simple-echo/python-worker/main.py"),
    ),
    Snippet(
        "python-client",
        "python",
        Path("examples/simple-echo/python-client/main.py"),
    ),
    Snippet(
        "node-worker",
        "javascript",
        Path("examples/simple-echo/node-worker/main.js"),
    ),
    Snippet(
        "node-client",
        "javascript",
        Path("examples/simple-echo/node-client/main.js"),
    ),
)


class SyncError(Exception):
    """A deterministic, user-actionable synchronization failure."""


@dataclass(frozen=True)
class Region:
    snippet: Snippet
    marker_start: int
    content_start: int
    content_end: int
    marker_end: int


@dataclass(frozen=True)
class Fence:
    character: int
    length: int


def _marker(label: str, boundary: str) -> bytes:
    return f"<!-- cap-snippet:{label}:{boundary} -->".encode("utf-8")


def _normalize_lf(data: bytes) -> bytes:
    return data.replace(b"\r\n", b"\n")


def _guide_eol(guide: bytes) -> bytes:
    crlf_count = guide.count(b"\r\n")
    bare_lf_count = guide.count(b"\n") - crlf_count
    if crlf_count and bare_lf_count:
        raise SyncError("guide: mixed LF and CRLF line endings")
    return b"\r\n" if crlf_count else b"\n"


def _read_utf8(path: Path, owner: str) -> bytes:
    try:
        data = path.read_bytes()
    except FileNotFoundError as exc:
        raise SyncError(f"{owner}: missing file: {path}") from exc
    except OSError as exc:
        raise SyncError(f"{owner}: cannot read {path}: {exc}") from exc
    try:
        data.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise SyncError(f"{owner}: {path} is not valid UTF-8: {exc}") from exc
    return data


def _marker_line(guide: bytes, snippet: Snippet, boundary: str) -> tuple[int, int]:
    marker = _marker(snippet.label, boundary)
    count = guide.count(marker)
    if count != 1:
        qualifier = "missing" if count == 0 else f"duplicated {count} times"
        raise SyncError(f"{snippet.label}: {boundary} marker is {qualifier}")
    position = guide.index(marker)
    line_start = guide.rfind(b"\n", 0, position) + 1
    newline = guide.find(b"\n", position)
    line_end = len(guide) if newline < 0 else newline
    line = guide[line_start:line_end].removesuffix(b"\r")
    if position != line_start or line != marker:
        raise SyncError(
            f"{snippet.label}: {boundary} marker must be an exact standalone line"
        )
    after_line = len(guide) if newline < 0 else newline + 1
    return position, after_line


def _advance_fence(active: Optional[Fence], line: bytes) -> Optional[Fence]:
    content = line.removesuffix(b"\r")
    stripped = content.lstrip(b" ")
    if len(content) - len(stripped) > 3 or not stripped:
        return active
    character = stripped[0]
    if character not in (ord("`"), ord("~")):
        return active
    run = len(stripped) - len(stripped.lstrip(bytes((character,))))
    if active is None:
        return Fence(character, run) if run >= 3 else None
    remainder = stripped[run:]
    if character == active.character and run >= active.length and not remainder.strip():
        return None
    return active


def _outside_fence(guide: bytes, position: int) -> bool:
    active: Optional[Fence] = None
    offset = 0
    for line in guide.splitlines(keepends=True):
        if offset >= position:
            break
        active = _advance_fence(active, line.rstrip(b"\n"))
        offset += len(line)
    return active is None


def _validate_region(guide: bytes, snippet: Snippet) -> Region:
    start, content_start = _marker_line(guide, snippet, "start")
    end, marker_end = _marker_line(guide, snippet, "end")
    if start >= end:
        raise SyncError(f"{snippet.label}: start and end markers are misordered")
    if not _outside_fence(guide, start) or not _outside_fence(guide, end):
        raise SyncError(f"{snippet.label}: marker must be outside fenced code")
    return Region(snippet, start, content_start, end, marker_end)


def _validate_regions(guide: bytes) -> tuple[Region, ...]:
    regions: list[Region] = []
    errors: list[str] = []
    for snippet in SNIPPETS:
        try:
            regions.append(_validate_region(guide, snippet))
        except SyncError as exc:
            errors.append(str(exc))
    if errors:
        raise SyncError("\n".join(errors))
    ordered = sorted(regions, key=lambda region: region.marker_start)
    actual = tuple(region.snippet.label for region in ordered)
    expected = tuple(snippet.label for snippet in SNIPPETS)
    if actual != expected:
        mismatch = next(label for label, wanted in zip(actual, expected) if label != wanted)
        raise SyncError(f"{mismatch}: snippet blocks are misordered")
    for previous, current in zip(ordered, ordered[1:]):
        if current.marker_start < previous.marker_end:
            raise SyncError(
                f"{current.snippet.label}: marker block overlaps {previous.snippet.label}"
            )
    return tuple(ordered)


def _load_blocks(root: Path) -> dict[str, bytes]:
    blocks: dict[str, bytes] = {}
    errors: list[str] = []
    for snippet in SNIPPETS:
        try:
            source = _read_utf8(root / snippet.source, snippet.label)
        except SyncError as exc:
            errors.append(str(exc))
            continue
        normalized = _normalize_lf(source)
        blocks[snippet.label] = (
            b"```" + snippet.language.encode("ascii") + b"\n" + normalized + b"```\n"
        )
    if errors:
        raise SyncError("\n".join(errors))
    return blocks


def _render(
    guide: bytes,
    regions: tuple[Region, ...],
    blocks: dict[str, bytes],
    eol: bytes,
) -> bytes:
    output = bytearray()
    cursor = 0
    for region in regions:
        output.extend(guide[cursor : region.content_start])
        output.extend(blocks[region.snippet.label].replace(b"\n", eol))
        cursor = region.content_end
    output.extend(guide[cursor:])
    return bytes(output)


def _atomic_write(path: Path, data: bytes, original_mode: int) -> None:
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=path.parent
    )
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(temporary, stat.S_IMODE(original_mode))
        os.replace(temporary, path)
    except OSError as exc:
        temporary.unlink(missing_ok=True)
        raise SyncError(f"guide: cannot atomically replace {path}: {exc}") from exc


def synchronize(root: Path, check: bool) -> tuple[bool, tuple[str, ...]]:
    guide_path = root / GUIDE_PATH
    guide = _read_utf8(guide_path, "guide")
    eol = _guide_eol(guide)
    regions = _validate_regions(guide)
    blocks = _load_blocks(root)
    drifted = tuple(
        region.snippet.label
        for region in regions
        if _normalize_lf(guide[region.content_start : region.content_end])
        != blocks[region.snippet.label]
    )
    if check or not drifted:
        return False, drifted
    rendered = _render(guide, regions, blocks, eol)
    try:
        current = guide_path.read_bytes()
        mode = guide_path.stat().st_mode
    except OSError as exc:
        raise SyncError(f"guide: cannot recheck {guide_path}: {exc}") from exc
    if current != guide:
        raise SyncError("guide: changed during synchronization; refusing to overwrite")
    _atomic_write(guide_path, rendered, mode)
    return True, drifted


def _parse_args(argv: Optional[Sequence[str]]) -> tuple[Path, bool]:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=DEFAULT_ROOT)
    parser.add_argument(
        "--check",
        action="store_true",
        help="report drift without modifying the guide",
    )
    namespace = parser.parse_args(argv)
    return cast(Path, namespace.root).resolve(), cast(bool, namespace.check)


def main(argv: Optional[Sequence[str]] = None) -> int:
    root, check = _parse_args(argv)
    try:
        changed, drifted = synchronize(root, check)
    except SyncError as exc:
        print(f"snippet sync failed:\n{exc}", file=sys.stderr)
        return 1
    if check and drifted:
        print(f"snippet drift: {', '.join(drifted)}", file=sys.stderr)
        return 1
    status = "updated" if changed else "verified"
    print(f"snippet sync {status}: {GUIDE_PATH.as_posix()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
