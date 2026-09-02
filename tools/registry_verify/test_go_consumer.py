from __future__ import annotations

import contextlib
import io
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from typing import Mapping, Sequence
from unittest import mock

from tools.registry_verify import go_consumer

EXPECTED_HELPER = Path(__file__).with_name("go_consumer.py").resolve()
if Path(go_consumer.__file__).resolve() != EXPECTED_HELPER:
    raise ImportError(f"loaded non-local go_consumer helper: {go_consumer.__file__}")

MODULE = "github.com/cordum-io/cap/v2"
VERSION = "2.16.1"
def manifest_data() -> dict[str, object]:
    return {
        "schemaVersion": "1.2.0",
        "release": {"version": VERSION, "tag": f"v{VERSION}", "channel": "stable"},
        "components": [
            {
                "name": "cap-go",
                "kind": "sdk",
                "tier": "stable",
                "language": "go",
                "package": MODULE,
                "registry": "proxy.golang.org",
                "version": VERSION,
                "publication": "published",
            }
        ],
    }
def write_fixture(root: Path) -> tuple[Path, Path]:
    (root / "release").mkdir()
    (root / "examples").mkdir()
    manifest = root / "release" / "manifest.json"
    example = root / "examples" / "quickstart example.go"
    manifest.write_text(json.dumps(manifest_data()), encoding="utf-8")
    example.write_text("package main\nfunc main() {}\n", encoding="utf-8")
    return manifest, example
class ImplementationRequiredTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self.assertEqual(Path(go_consumer.__file__).resolve(), EXPECTED_HELPER)
class ReleaseParsingTests(ImplementationRequiredTestCase):
    def test_accepts_exact_published_stable_go_component(self) -> None:
        with tempfile.TemporaryDirectory(prefix="cap registry ") as temporary:
            manifest, _ = write_fixture(Path(temporary))
            release = go_consumer.load_release_config(manifest)
        self.assertEqual((release.module, release.version), (MODULE, VERSION))
    def test_rejects_malformed_or_missing_manifest(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            malformed = root / "bad.json"
            malformed.write_text("{", encoding="utf-8")
            for path in (malformed, root / "missing.json"):
                with self.subTest(path=path.name):
                    with self.assertRaises(go_consumer.ManifestError):
                        go_consumer.load_release_config(path)
    def test_rejects_duplicate_manifest_keys(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "manifest.json"
            path.write_text('{"schemaVersion":"1.2.0","schemaVersion":"1.1.0"}', encoding="utf-8")
            with self.assertRaisesRegex(go_consumer.ManifestError, "duplicate"):
                go_consumer.load_release_config(path)
    def test_rejects_noncanonical_release_or_go_component(self) -> None:
        mutations = {
            "prerelease": ("release", "version", "2.16.1-rc.1"),
            "tag": ("release", "tag", "v2.16.0"),
            "channel": ("release", "channel", "candidate"),
            "module": ("components", 0, "package", "example.com/not-cap"),
            "registry": ("components", 0, "registry", "proxy.example.test"),
            "version": ("components", 0, "version", "2.16.0"),
            "publication": ("components", 0, "publication", "prepared"),
        }
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "manifest.json"
            for name, mutation in mutations.items():
                data = manifest_data()
                target: object = data
                for key in mutation[:-2]:
                    target = target[key]  # type: ignore[index]
                target[mutation[-2]] = mutation[-1]  # type: ignore[index]
                path.write_text(json.dumps(data), encoding="utf-8")
                with self.subTest(name=name):
                    with self.assertRaises(go_consumer.ManifestError):
                        go_consumer.load_release_config(path)
class EnvironmentAndCommandTests(ImplementationRequiredTestCase):
    def test_environment_is_official_and_clears_bypass_inputs(self) -> None:
        inherited = {
            "PATH": os.environ.get("PATH", ""),
            "GOFLAGS": "-mod=mod",
            "GOPRIVATE": "corp.test",
            "GONOPROXY": "*",
            "GONOSUMDB": "*",
            "GOINSECURE": "*",
            "GOAUTH": "helper-command",
            "GOCACHEPROG": "cache-command",
            "CC": "compiler-command",
        }
        env = go_consumer.build_go_environment(Path("C:/temp/root"), inherited)
        self.assertEqual(env["GOPROXY"], "https://proxy.golang.org")
        self.assertEqual(env["GOSUMDB"], "sum.golang.org")
        self.assertEqual(env["GOWORK"], "off")
        self.assertEqual(env["GOTOOLCHAIN"], "local")
        for name in go_consumer.CLEARED_GO_ENV:
            self.assertEqual(env[name], "")
        self.assertEqual(env["GOENV"], "off")
        self.assertEqual(env["GOAUTH"], "off")
        self.assertEqual(env["GOTELEMETRY"], "off")
        self.assertEqual(env["CGO_ENABLED"], "0")
        self.assertEqual(env["GOVCS"], "*:off")
        self.assertEqual(env["GO111MODULE"], "on")
        self.assertNotEqual(Path(env["GOMODCACHE"]), Path(env["GOCACHE"]))
        self.assertNotEqual(Path(env["GOPATH"]), Path(env["GOMODCACHE"]))

    def test_go_resolution_uses_only_path_and_rejects_source(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source, trusted = root / "source", root / "trusted"
            source.mkdir()
            trusted.mkdir()
            name = "go.exe" if os.name == "nt" else "go"
            for directory in (source, trusted):
                executable = directory / name
                executable.write_bytes(b"fixture")
                executable.chmod(0o755)
            path = os.pathsep.join(("", str(source), str(trusted)))
            self.assertEqual(
                go_consumer._resolve_go(None, (source,), path), str((trusted / name).resolve())
            )
            with self.assertRaises(go_consumer.VerificationError):
                go_consumer._resolve_go(None, (source,), str(source))

class MutationTests(ImplementationRequiredTestCase):
    def test_missing_grpc_metadata_is_rejected_then_restored(self) -> None:
        original = b"module.test v1.0.0 h1:x\ngoogle.golang.org/grpc v1.2.3 h1:y\ngoogle.golang.org/grpc v1.2.3/go.mod h1:z\n"
        with tempfile.TemporaryDirectory() as temporary:
            sum_path = Path(temporary) / "go.sum"
            sum_path.write_bytes(original)

            def failing_build() -> go_consumer.RunResult:
                self.assertNotIn(b"google.golang.org/grpc ", sum_path.read_bytes())
                return go_consumer.RunResult(("go", "build"), 1, "missing go.sum entry for google.golang.org/grpc")

            go_consumer.prove_missing_grpc_sums(sum_path, failing_build)
            self.assertEqual(sum_path.read_bytes(), original)

    def test_mutation_requires_targets_and_exact_failure_signature(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            sum_path = Path(temporary) / "go.sum"
            for content, output in (
                (b"other v1 h1:x\n", "missing go.sum entry for grpc"),
                (b"google.golang.org/grpc v1 h1:x\ngoogle.golang.org/grpc v1/go.mod h1:y\n", "network failed"),
            ):
                sum_path.write_bytes(content)
                with self.subTest(content=content):
                    with self.assertRaises(go_consumer.VerificationError):
                        go_consumer.prove_missing_grpc_sums(
                            sum_path,
                            lambda: go_consumer.RunResult(("go", "build"), 1, output),
                        )
                    self.assertEqual(sum_path.read_bytes(), content)

    def test_restores_sum_bytes_when_build_raises(self) -> None:
        original = (
            b"google.golang.org/grpc v1 h1:x\n"
            b"google.golang.org/grpc v1/go.mod h1:y\n"
        )
        with tempfile.TemporaryDirectory() as temporary:
            sum_path = Path(temporary) / "go.sum"
            sum_path.write_bytes(original)

            def explode() -> go_consumer.RunResult:
                raise RuntimeError("runner crashed")

            with self.assertRaisesRegex(RuntimeError, "runner crashed"):
                go_consumer.prove_missing_grpc_sums(sum_path, explode)
            self.assertEqual(sum_path.read_bytes(), original)

class ModuleGraphTests(ImplementationRequiredTestCase):
    def test_rejects_replace_wrong_version_and_source_checkout(self) -> None:
        release = go_consumer.ReleaseConfig(MODULE, VERSION)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            cache = root / "cache"
            source = root / "source"
            valid = json.dumps({"Path": MODULE, "Version": f"v{VERSION}", "Dir": str(cache / "cap")})
            edit = json.dumps({"Replace": None})
            go_consumer.validate_module_graph(edit, valid, release, source, cache)
            invalid = {
                "replace": (json.dumps({"Replace": [{}]}), valid),
                "version": (edit, valid.replace(VERSION, "2.16.0")),
                "source": (
                    edit,
                    json.dumps({"Path": MODULE, "Version": f"v{VERSION}", "Dir": str(source / "sdk")}),
                ),
            }
            for name, values in invalid.items():
                with self.subTest(name=name):
                    with self.assertRaises(go_consumer.VerificationError):
                        go_consumer.validate_module_graph(*values, release, source, cache)
            with self.assertRaises(go_consumer.VerificationError):
                go_consumer.validate_module_graph(edit, valid, release, root, cache)

class VerificationFlowTests(ImplementationRequiredTestCase):
    def test_requires_existing_example_before_running_commands(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifest, _ = write_fixture(root)
            runner = mock.Mock()
            with self.assertRaises(go_consumer.VerificationError):
                go_consumer.verify_consumer(manifest, root / "missing.go", False, runner=runner)
            runner.assert_not_called()

    def test_flow_uses_exact_cap_requirement_and_order(self) -> None:
        with tempfile.TemporaryDirectory(prefix="cap source with spaces ") as temporary:
            manifest, example = write_fixture(Path(temporary))
            runner = FakeRunner()
            with contextlib.redirect_stdout(io.StringIO()):
                go_consumer.verify_consumer(
                    manifest, example, False, runner=runner, go_executable=sys.executable
                )
        commands = [call[0][1:] for call in runner.calls]
        self.assertEqual(
            commands,
            [
                ("mod", "init", "example.com/cap-registry-verify"),
                ("mod", "edit", f"-require={MODULE}@v{VERSION}"),
                ("mod", "tidy"),
                ("mod", "verify"),
                ("mod", "edit", "-json"),
                ("list", "-m", "-json", "all"),
                ("build", "-mod=readonly", "."),
                ("build", "-mod=readonly", "."),
                ("mod", "verify"),
                ("build", "-mod=readonly", "."),
            ],
        )
        self.assertFalse(any("nats.go@" in part or "protobuf@" in part for command in commands for part in command))
        cwd, env = runner.calls[0][1:3]
        self.assertIn(" ", str(cwd))
        self.assertEqual(Path(env["GOMODCACHE"]).parent, cwd.parent)
        self.assertFalse(Path(env["GOMODCACHE"]).is_relative_to(cwd))

    def test_run_mode_executes_readonly_quickstart_last(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            manifest, example = write_fixture(Path(temporary))
            runner = FakeRunner()
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                go_consumer.verify_consumer(
                    manifest, example, True, runner=runner, go_executable=sys.executable
                )
        self.assertEqual(runner.calls[-1][0][1:], ("run", "-mod=readonly", "."))
        for evidence in ("module-graph=OK", "go-mod-verify=OK", "grpc-sum-mutation=REJECTED_AND_RESTORED", "readonly-build=OK", "quickstart-echo: OK"):
            self.assertIn(evidence, output.getvalue())

    def test_cli_requires_exactly_one_mode(self) -> None:
        base = ["--manifest", "m.json", "--example", "main.go"]
        for argv in (base, base + ["--build-only", "--run"]):
            with self.subTest(argv=argv):
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
                    go_consumer.parse_args(argv)

class FakeRunner:
    def __init__(self) -> None:
        self.calls: list[tuple[tuple[str, ...], Path, Mapping[str, str], int]] = []

    def __call__(
        self, argv: Sequence[str], cwd: Path, env: Mapping[str, str], timeout: int
    ) -> "go_consumer.RunResult":
        command = tuple(argv)
        self.calls.append((command, cwd, dict(env), timeout))
        args = command[1:]
        if args[:2] == ("mod", "init"):
            (cwd / "go.mod").write_text("module example.com/cap-registry-verify\n", encoding="utf-8")
        if args == ("mod", "tidy"):
            (cwd / "go.sum").write_bytes(
                b"google.golang.org/grpc v1.79.3 h1:x\n"
                b"google.golang.org/grpc v1.79.3/go.mod h1:y\n"
            )
        if args == ("mod", "edit", "-json"):
            return go_consumer.RunResult(command, 0, json.dumps({"Replace": None}))
        if args == ("list", "-m", "-json", "all"):
            module = {"Path": MODULE, "Version": f"v{VERSION}", "Dir": str(Path(env["GOMODCACHE"]) / "cap")}
            return go_consumer.RunResult(command, 0, json.dumps(module))
        if args[:1] == ("build",) and b"google.golang.org/grpc " not in (cwd / "go.sum").read_bytes():
            return go_consumer.RunResult(command, 1, "missing go.sum entry for google.golang.org/grpc")
        return go_consumer.RunResult(command, 0, "quickstart-echo: OK")

if __name__ == "__main__":
    unittest.main()
