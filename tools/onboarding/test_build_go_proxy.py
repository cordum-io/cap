"""Contract tests for the deterministic local Go module proxy builder."""

from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path
from typing import Sequence


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
BUILDER = REPOSITORY_ROOT / "tools" / "onboarding" / "build_go_proxy.py"
MODULE = "github.com/cordum-io/cap/v2"
VERSION = "v2.5.3-onboarding.0.g0123456789ab"
PREFIX = f"{MODULE}@{VERSION}/"


def _run(command: Sequence[str], *, cwd: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        cwd=cwd,
        capture_output=True,
        text=True,
        timeout=15,
        check=False,
    )


def _snapshot(root: Path) -> dict[str, str]:
    return {
        path.relative_to(root).as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
        for path in sorted(root.rglob("*"))
        if path.is_file()
    }


class GoProxyBuilderTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory(prefix="cap-go-proxy-")
        self.root = Path(self.temporary.name) / "source"
        self.root.mkdir()
        self._initialize_repository()

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def _write(self, relative: str, data: bytes) -> None:
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

    def _initialize_repository(self) -> None:
        result = _run(["git", "init", "-q"], cwd=self.root)
        self.assertEqual(result.returncode, 0, result.stderr)
        for key, value in (("user.name", "CAP Test"), ("user.email", "cap@example.invalid"), ("core.autocrlf", "true")):
            configured = _run(["git", "config", key, value], cwd=self.root)
            self.assertEqual(configured.returncode, 0, configured.stderr)
        tracked = {
            ".gitignore": b".cache/\nsdk/go/worker/ignored.go\n",
            "go.mod": f"module {MODULE}\n\ngo 1.25.12\n".encode(),
            "go.sum": b"example.invalid/module v1.0.0 h1:test\n",
            "LICENSE": b"fixture license\n",
            "cordum/agent/v1/job.pb.go": b"package agentv1\n",
            "sdk/go/worker/worker.go": b"package worker\n",
            "sdk/go/worker/worker_test.go": b"package worker\n",
            "docs/private.md": b"must not ship\n",
        }
        for relative, data in tracked.items():
            self._write(relative, data)
        added = _run(["git", "add", "--", *tracked], cwd=self.root)
        self.assertEqual(added.returncode, 0, added.stderr)
        committed = _run(["git", "commit", "-qm", "fixture"], cwd=self.root)
        self.assertEqual(committed.returncode, 0, committed.stderr)
        self._write("sdk/go/worker/untracked.go", b"package worker\n")
        self._write("sdk/go/worker/ignored.go", b"package worker\n")
        self._write(".cache/ignored.go", b"secret\n")

    def _build(
        self, output: Path, version: str = VERSION
    ) -> subprocess.CompletedProcess[str]:
        self.assertTrue(BUILDER.is_file(), f"Go proxy builder is missing: {BUILDER}")
        return _run(
            [
                sys.executable,
                str(BUILDER),
                "--root",
                str(self.root),
                "--output",
                str(output),
                "--version",
                version,
            ],
            cwd=REPOSITORY_ROOT,
        )

    def _version_dir(self, output: Path) -> Path:
        return output / MODULE / "@v"

    def test_builds_exact_proxy_metadata_and_allowlisted_zip(self) -> None:
        output = Path(self.temporary.name) / "proxy"
        result = self._build(output)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        version_dir = self._version_dir(output)
        self.assertEqual((version_dir / "list").read_text(), VERSION + "\n")
        self.assertEqual(
            (version_dir / f"{VERSION}.mod").read_bytes(),
            (self.root / "go.mod").read_bytes(),
        )
        self.assertEqual(
            json.loads((version_dir / f"{VERSION}.info").read_text()),
            {"Version": VERSION},
        )
        with zipfile.ZipFile(version_dir / f"{VERSION}.zip") as archive:
            names = archive.namelist()
            expected = [
                PREFIX + "LICENSE",
                PREFIX + "cordum/agent/v1/job.pb.go",
                PREFIX + "go.mod",
                PREFIX + "go.sum",
                PREFIX + "sdk/go/worker/worker.go",
            ]
            self.assertEqual(names, expected)
            self.assertNotIn(PREFIX + "sdk/go/worker/worker_test.go", names)
            self.assertNotIn(PREFIX + "sdk/go/worker/ignored.go", names)
            self.assertNotIn(PREFIX + "sdk/go/worker/untracked.go", names)
            self.assertTrue(
                all(
                    info.date_time == (1980, 1, 1, 0, 0, 0)
                    for info in archive.infolist()
                )
            )
            self.assertEqual(archive.read(PREFIX + "go.mod"), (self.root / "go.mod").read_bytes())

    def test_build_is_byte_deterministic(self) -> None:
        first = Path(self.temporary.name) / "first"
        second = Path(self.temporary.name) / "second"
        self.assertEqual(self._build(first).returncode, 0)
        self._write("go.mod", (self.root / "go.mod").read_bytes().replace(b"\n", b"\r\n"))
        self.assertEqual(self._build(second).returncode, 0)
        self.assertEqual(_snapshot(first), _snapshot(second))
        with zipfile.ZipFile(self._version_dir(second) / f"{VERSION}.zip") as archive:
            self.assertNotIn(b"\r\n", archive.read(PREFIX + "go.mod"))

    def test_tracked_source_drift_is_rejected(self) -> None:
        self._write("sdk/go/worker/worker.go", b"package changed\n")
        output = Path(self.temporary.name) / "dirty"
        result = self._build(output)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("differs", result.stdout + result.stderr)
        self.assertFalse(output.exists())

    def test_existing_empty_output_is_supported(self) -> None:
        output = Path(self.temporary.name) / "empty"
        output.mkdir()
        result = self._build(output)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue((self._version_dir(output) / f"{VERSION}.zip").is_file())

    def test_nonempty_output_is_rejected_without_mutation(self) -> None:
        output = Path(self.temporary.name) / "occupied"
        output.mkdir()
        sentinel = output / "keep.txt"
        sentinel.write_bytes(b"keep\n")
        before = _snapshot(output)
        result = self._build(output)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("output", result.stdout + result.stderr)
        self.assertEqual(_snapshot(output), before)

    def test_symlink_output_is_rejected_without_touching_target(self) -> None:
        target = Path(self.temporary.name) / "target"
        target.mkdir()
        output = Path(self.temporary.name) / "proxy-link"
        try:
            output.symlink_to(target, target_is_directory=True)
        except OSError as error:
            self.skipTest(f"directory symlinks unavailable: {error}")
        result = self._build(output)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("symlink", (result.stdout + result.stderr).lower())
        self.assertTrue(output.is_symlink())
        self.assertEqual(list(target.iterdir()), [])

    def test_git_symlink_mode_is_rejected_even_if_checkout_is_regular(self) -> None:
        relative = "sdk/go/worker/link.go"
        self._write(relative, b"worker.go")
        hashed = _run(["git", "hash-object", "-w", relative], cwd=self.root)
        self.assertEqual(hashed.returncode, 0, hashed.stderr)
        cache = f"120000,{hashed.stdout.strip()},{relative}"
        indexed = _run(["git", "update-index", "--add", "--cacheinfo", cache], cwd=self.root)
        self.assertEqual(indexed.returncode, 0, indexed.stderr)
        self.assertFalse((self.root / relative).is_symlink())
        output = Path(self.temporary.name) / "git-symlink"
        result = self._build(output)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("differs", result.stdout + result.stderr)
        self.assertFalse(output.exists())

    def test_source_parent_link_cannot_redirect_reads_outside_root(self) -> None:
        worker = self.root / "sdk" / "go" / "worker"
        original = worker.with_name("worker-original")
        outside = Path(self.temporary.name) / "outside-worker"
        worker.rename(original)
        outside.mkdir()
        (outside / "worker.go").write_bytes(b"package outside\n")
        try:
            self._link_directory(worker, outside)
            self.assertFalse((worker / "worker.go").is_symlink())
            output = Path(self.temporary.name) / "redirected"
            result = self._build(output)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("differs", (result.stdout + result.stderr).lower())
            self.assertFalse(output.exists())
        finally:
            if worker.exists():
                if os.name == "nt":
                    os.rmdir(worker)
                else:
                    worker.unlink()
            original.rename(worker)

    def _link_directory(self, link: Path, target: Path) -> None:
        try:
            link.symlink_to(target, target_is_directory=True)
            return
        except OSError:
            if os.name != "nt":
                raise
        result = _run(
            ["cmd.exe", "/c", "mklink", "/J", str(link), str(target)],
            cwd=Path(self.temporary.name),
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_missing_tracked_source_fails_before_output(self) -> None:
        (self.root / "sdk/go/worker/worker.go").unlink()
        output = Path(self.temporary.name) / "missing"
        result = self._build(output)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("worker.go", result.stdout + result.stderr)
        self.assertFalse(output.exists())

    def test_invalid_module_declaration_is_rejected(self) -> None:
        self._write("go.mod", b"module example.com/wrong/v2\n\ngo 1.25.12\n")
        added = _run(["git", "add", "go.mod"], cwd=self.root)
        self.assertEqual(added.returncode, 0, added.stderr)
        committed = _run(["git", "commit", "-qm", "invalid module"], cwd=self.root)
        self.assertEqual(committed.returncode, 0, committed.stderr)
        result = self._build(Path(self.temporary.name) / "wrong")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(MODULE, result.stdout + result.stderr)

    def test_invalid_versions_are_rejected(self) -> None:
        invalid = (
            "2.5.3-onboarding.0",
            "v1.5.3-onboarding.0",
            "v2.5.3",
            "v2.05.3-onboarding.0",
            "v2.5.3-onboarding.00",
            "v2.5.3-bad+metadata",
            "v2.5.3-bad/path",
            "v2.5.3-RC.1",
        )
        for version in invalid:
            with self.subTest(version=version):
                result = self._build(Path(self.temporary.name) / version.replace("/", "_"), version)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("version", (result.stdout + result.stderr).lower())

    def test_documented_artifact_directory_is_git_ignored(self) -> None:
        documented_paths = (
            ".artifacts/onboarding-probe",
            ".artifact-venv/onboarding-probe",
        )
        for path in documented_paths:
            with self.subTest(path=path):
                result = _run(
                    ["git", "check-ignore", "--quiet", path],
                    cwd=REPOSITORY_ROOT,
                )
                self.assertEqual(
                    result.returncode,
                    0,
                    "documented in-clone artifacts must not dirty git status",
                )


if __name__ == "__main__":
    unittest.main()
