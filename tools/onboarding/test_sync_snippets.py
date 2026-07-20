"""Contract tests for the Getting Started snippet synchronizer."""

from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from dataclasses import dataclass
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
SYNC_TOOL = REPOSITORY_ROOT / "tools" / "onboarding" / "sync_snippets.py"
GUIDE_PATH = Path("docs/getting-started.md")


@dataclass(frozen=True)
class Snippet:
    label: str
    language: str
    source: Path


SNIPPETS = (
    Snippet("go-worker", "go", Path("examples/simple-echo/go-worker/main.go")),
    Snippet("go-client", "go", Path("examples/simple-echo/go-client/main.go")),
    Snippet("python-worker", "python", Path("examples/simple-echo/python-worker/main.py")),
    Snippet("python-client", "python", Path("examples/simple-echo/python-client/main.py")),
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


def _marker(label: str, boundary: str) -> bytes:
    return f"<!-- cap-snippet:{label}:{boundary} -->".encode()


def _normalize_lf(data: bytes) -> bytes:
    """Normalize CRLF only; do not strip or otherwise rewrite source bytes."""
    return data.replace(b"\r\n", b"\n")


def _expected_block(root: Path, snippet: Snippet) -> bytes:
    source = _normalize_lf((root / snippet.source).read_bytes())
    return b"```" + snippet.language.encode() + b"\n" + source + b"```\n"


def _extract_block(guide: bytes, snippet: Snippet) -> bytes:
    guide = _normalize_lf(guide)
    start = _marker(snippet.label, "start") + b"\n"
    end = _marker(snippet.label, "end")
    start_index = guide.index(start) + len(start)
    end_index = guide.index(end, start_index)
    return guide[start_index:end_index]


def _render_block(snippet: Snippet, body: bytes) -> bytes:
    return (
        _marker(snippet.label, "start")
        + b"\n```"
        + snippet.language.encode()
        + b"\n"
        + body
        + b"```\n"
        + _marker(snippet.label, "end")
        + b"\n"
    )


def _tree_snapshot(root: Path) -> dict[str, bytes]:
    return {
        path.relative_to(root).as_posix(): path.read_bytes()
        for path in sorted(root.rglob("*"))
        if path.is_file()
    }


class RepositorySnippetContractTests(unittest.TestCase):
    def test_repository_has_each_required_marker_exactly_once(self) -> None:
        guide = (REPOSITORY_ROOT / GUIDE_PATH).read_bytes()

        for snippet in SNIPPETS:
            with self.subTest(snippet=snippet.label):
                for boundary in ("start", "end"):
                    marker = _marker(snippet.label, boundary)
                    self.assertEqual(
                        guide.count(marker),
                        1,
                        f"required marker must occur once: {marker.decode()}",
                    )

    def test_repository_fences_match_canonical_bytes_with_lf_normalization(self) -> None:
        guide = (REPOSITORY_ROOT / GUIDE_PATH).read_bytes()

        for snippet in SNIPPETS:
            with self.subTest(snippet=snippet.label):
                start = _marker(snippet.label, "start")
                end = _marker(snippet.label, "end")
                self.assertEqual(
                    (guide.count(start), guide.count(end)),
                    (1, 1),
                    f"cannot compare {snippet.label}: expected one marker pair",
                )
                self.assertEqual(
                    _extract_block(guide, snippet),
                    _expected_block(REPOSITORY_ROOT, snippet),
                    f"{snippet.label} must exactly mirror {snippet.source.as_posix()}",
                )


class SynchronizerCliTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory(prefix="cap-snippets-")
        self.root = Path(self.temporary.name)
        self._write_sources()

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def _write_sources(self) -> None:
        for snippet in SNIPPETS:
            path = self.root / snippet.source
            path.parent.mkdir(parents=True, exist_ok=True)
            content = (
                f"{snippet.label} first  \r\n"
                f"\t{snippet.label} indented\r\n"
                f"{snippet.label} last\t\r\n"
            )
            path.write_bytes(content.encode())

    def _write_guide(self, *, canonical: bool) -> None:
        guide = bytearray(b"# Fixture\n\n")
        for snippet in SNIPPETS:
            if canonical:
                body = _normalize_lf((self.root / snippet.source).read_bytes())
            else:
                body = f"stale {snippet.label}\n".encode()
            guide.extend(_render_block(snippet, body))
            guide.extend(b"\n")
        path = self.root / GUIDE_PATH
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(bytes(guide))

    def _run_tool(self, *, check: bool) -> subprocess.CompletedProcess[str]:
        self.assertTrue(
            SYNC_TOOL.is_file(),
            f"snippet synchronizer is missing: {SYNC_TOOL}",
        )
        command = [sys.executable, str(SYNC_TOOL), "--root", str(self.root)]
        if check:
            command.append("--check")
        return subprocess.run(
            command,
            cwd=REPOSITORY_ROOT,
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
        )

    def _assert_synced(self) -> None:
        guide = (self.root / GUIDE_PATH).read_bytes()
        for snippet in SNIPPETS:
            with self.subTest(snippet=snippet.label):
                self.assertEqual(guide.count(_marker(snippet.label, "start")), 1)
                self.assertEqual(guide.count(_marker(snippet.label, "end")), 1)
                self.assertEqual(
                    _extract_block(guide, snippet),
                    _expected_block(self.root, snippet),
                )

    def _assert_rejected(
        self, result: subprocess.CompletedProcess[str], label: str
    ) -> None:
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(label, result.stdout + result.stderr)

    def test_sync_uses_all_six_fixed_file_and_language_mappings(self) -> None:
        self._write_guide(canonical=False)
        sources_before = {
            snippet.source: (self.root / snippet.source).read_bytes()
            for snippet in SNIPPETS
        }

        result = self._run_tool(check=False)

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self._assert_synced()
        self.assertEqual(
            {path: (self.root / path).read_bytes() for path in sources_before},
            sources_before,
        )

    def test_sync_preserves_source_whitespace_except_crlf(self) -> None:
        self._write_guide(canonical=False)

        result = self._run_tool(check=False)

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        guide = (self.root / GUIDE_PATH).read_bytes()
        self.assertIn(b"go-worker first  \n\tgo-worker indented\n", guide)
        self.assertIn(b"go-worker last\t\n```\n", guide)
        self.assertNotIn(b"\r\n", guide)

    def test_check_mode_does_not_mutate_drifted_files(self) -> None:
        self._write_guide(canonical=False)
        before = _tree_snapshot(self.root)

        result = self._run_tool(check=True)

        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(_tree_snapshot(self.root), before)

    def test_sync_is_byte_deterministic(self) -> None:
        self._write_guide(canonical=False)
        guide_path = self.root / GUIDE_PATH
        guide_path.write_bytes(guide_path.read_bytes().replace(b"\n", b"\r\n"))

        first = self._run_tool(check=False)
        first_snapshot = _tree_snapshot(self.root)
        second = self._run_tool(check=False)

        self.assertEqual(first.returncode, 0, first.stdout + first.stderr)
        self.assertEqual(second.returncode, 0, second.stdout + second.stderr)
        self.assertEqual(_tree_snapshot(self.root), first_snapshot)
        self.assertNotIn(b"\n", guide_path.read_bytes().replace(b"\r\n", b""))

    def test_check_accepts_exact_content_after_crlf_normalization(self) -> None:
        self._write_guide(canonical=True)
        path = self.root / GUIDE_PATH
        path.write_bytes(path.read_bytes().replace(b"\n", b"\r\n"))

        result = self._run_tool(check=True)

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_check_rejects_missing_marker(self) -> None:
        self._write_guide(canonical=True)
        path = self.root / GUIDE_PATH
        path.write_bytes(path.read_bytes().replace(_marker("go-worker", "start"), b"", 1))

        self._assert_rejected(self._run_tool(check=True), "go-worker")

    def test_check_rejects_duplicate_marker(self) -> None:
        self._write_guide(canonical=True)
        path = self.root / GUIDE_PATH
        marker = _marker("go-worker", "start")
        path.write_bytes(path.read_bytes().replace(marker, marker + b"\n" + marker, 1))

        self._assert_rejected(self._run_tool(check=True), "go-worker")

    def test_check_rejects_unterminated_marker(self) -> None:
        self._write_guide(canonical=True)
        path = self.root / GUIDE_PATH
        path.write_bytes(path.read_bytes().replace(_marker("go-worker", "end"), b"", 1))

        self._assert_rejected(self._run_tool(check=True), "go-worker")

    def test_sync_rejects_structural_corruption_without_mutation(self) -> None:
        start = _marker("go-worker", "start")
        end = _marker("go-worker", "end")
        corruptions = (
            ("missing", start, b""),
            ("duplicate", start, start + b"\n" + start),
            ("unterminated", end, b""),
        )
        for name, old, replacement in corruptions:
            with self.subTest(corruption=name):
                self._write_guide(canonical=True)
                path = self.root / GUIDE_PATH
                path.write_bytes(path.read_bytes().replace(old, replacement, 1))
                before = _tree_snapshot(self.root)

                result = self._run_tool(check=False)

                self._assert_rejected(result, "go-worker")
                self.assertEqual(_tree_snapshot(self.root), before)

    def test_check_rejects_one_byte_drift(self) -> None:
        self._write_guide(canonical=True)
        path = self.root / GUIDE_PATH
        path.write_bytes(path.read_bytes().replace(b"go-worker first", b"go-worker First", 1))

        self._assert_rejected(self._run_tool(check=True), "go-worker")


if __name__ == "__main__":
    unittest.main()
