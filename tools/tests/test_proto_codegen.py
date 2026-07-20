from __future__ import annotations

import json
import re
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from tools import codegen_toolchain, proto_codegen


class ProtoCodegenTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory(prefix="cap-codegen-test-")
        self.root = Path(self.temporary.name)
        self.proto_root = self.root / "proto" / "cordum" / "agent" / "v1"
        self.proto_root.mkdir(parents=True)
        (self.proto_root / "base.proto").write_text(
            'syntax = "proto3";\npackage cordum.agent.v1;\n', encoding="utf-8"
        )
        (self.proto_root / "service.proto").write_text(
            'syntax = "proto3";\npackage cordum.agent.v1;\n'
            'import "cordum/agent/v1/base.proto";\n'
            "service ExampleService {}\n",
            encoding="utf-8",
        )

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def test_expected_outputs_cover_every_tracked_language(self) -> None:
        protos = proto_codegen.proto_sources(self.proto_root)
        expected = proto_codegen.expected_outputs(protos)

        self.assertEqual(set(expected), set(proto_codegen.LANGUAGES))
        self.assertIn(Path("service_grpc.pb.go"), expected["go"])
        self.assertNotIn(Path("base_grpc.pb.go"), expected["go"])
        self.assertIn(Path("cordum/agent/v1/base_pb.d.ts"), expected["node"])
        self.assertIn(Path("cap_pb.js"), expected["node"])
        self.assertIn(Path("policy_pb.rb"), proto_codegen.ruby_outputs(("policy",)))

    def test_aggregate_roots_exclude_imported_protos(self) -> None:
        protos = proto_codegen.proto_sources(self.proto_root)

        roots = proto_codegen.aggregate_roots(protos)

        self.assertEqual([path.name for path in roots], ["service.proto"])

    def test_missing_generated_language_fails_closed(self) -> None:
        expected = proto_codegen.expected_outputs(
            proto_codegen.proto_sources(self.proto_root)
        )
        generated = self.root / "missing"
        for name, outputs in expected.items():
            if name == "ruby":
                continue
            base = generated / proto_codegen.LANGUAGES[name].generated_subdir
            for output in outputs:
                (base / output).parent.mkdir(parents=True, exist_ok=True)
                (base / output).write_text("generated", encoding="utf-8")

        with self.assertRaisesRegex(
            proto_codegen.CodegenError, "language ruby was skipped"
        ):
            proto_codegen.validate_generated(generated, expected)

    def test_inventory_rejects_missing_extra_and_empty_outputs(self) -> None:
        root = self.root / "generated"
        root.mkdir()
        (root / "base.pb.go").write_text("generated", encoding="utf-8")
        (root / "extra.pb.go").write_text("generated", encoding="utf-8")

        with self.assertRaisesRegex(proto_codegen.CodegenError, "missing=.*service"):
            proto_codegen.validate_inventory(
                root,
                {Path("base.pb.go"), Path("service.pb.go")},
                proto_codegen.LANGUAGES["go"],
                "generated",
            )

        (root / "service.pb.go").touch()
        with self.assertRaisesRegex(proto_codegen.CodegenError, "extra=.*extra"):
            proto_codegen.validate_inventory(
                root,
                {Path("base.pb.go"), Path("service.pb.go")},
                proto_codegen.LANGUAGES["go"],
                "generated",
            )
        (root / "extra.pb.go").unlink()
        with self.assertRaisesRegex(proto_codegen.CodegenError, "outputs are empty"):
            proto_codegen.validate_inventory(
                root,
                {Path("base.pb.go"), Path("service.pb.go")},
                proto_codegen.LANGUAGES["go"],
                "generated",
            )

    def test_snapshot_detects_non_idempotent_generation(self) -> None:
        expected = {"go": {Path("base.pb.go")}}
        generated = self.root / "generated" / "go" / "cordum" / "agent" / "v1"
        generated.mkdir(parents=True)
        (generated / "base.pb.go").write_text("first", encoding="utf-8")
        before = proto_codegen.snapshot_generated(self.root / "generated", expected)
        (generated / "base.pb.go").write_text("second", encoding="utf-8")

        with self.assertRaisesRegex(proto_codegen.CodegenError, "not idempotent"):
            proto_codegen.assert_idempotent(
                before,
                proto_codegen.snapshot_generated(self.root / "generated", expected),
            )

    def test_compare_tracked_detects_drift_per_output(self) -> None:
        # compare_tracked used to shell out to `git diff --no-index`, which fails
        # inside the hermetic container (/src/.git is a worktree pointer to a
        # host path that does not exist there), so it now compares content
        # directly. Exercise the real comparison rather than mocking it: a
        # mocked-out differ would keep passing even if the comparison were
        # dropped entirely.
        expected = {"go": {Path("base.pb.go")}}
        tracked = self.root / "cordum" / "agent" / "v1"
        generated = self.root / "temp" / "go" / "cordum" / "agent" / "v1"
        tracked.mkdir(parents=True)
        generated.mkdir(parents=True)
        (tracked / "base.pb.go").write_text("same", encoding="utf-8")
        (generated / "base.pb.go").write_text("same", encoding="utf-8")

        # Identical content: no drift.
        proto_codegen.compare_tracked(self.root / "temp", expected, self.root)

        # Any difference in a declared output must be reported as drift.
        (generated / "base.pb.go").write_text("different", encoding="utf-8")
        with self.assertRaisesRegex(proto_codegen.CodegenError, "generated protobuf drift"):
            proto_codegen.compare_tracked(self.root / "temp", expected, self.root)

    def test_compare_tracked_reports_a_missing_tracked_output(self) -> None:
        expected = {"go": {Path("base.pb.go")}}
        generated = self.root / "temp" / "go" / "cordum" / "agent" / "v1"
        generated.mkdir(parents=True)
        (generated / "base.pb.go").write_text("emitted", encoding="utf-8")

        # The tracked file does not exist at all; that is drift, not a crash.
        with self.assertRaises(proto_codegen.CodegenError):
            proto_codegen.compare_tracked(self.root / "temp", expected, self.root)

    def test_generator_config_requires_exact_remote_revisions(self) -> None:
        valid = self.root / "buf.gen.yaml"
        valid.write_text(
            "version: v2\nplugins:\n"
            "  - remote: buf.build/protocolbuffers/go:v1.36.11\n"
            "    revision: 1\n    out: go\n",
            encoding="utf-8",
        )
        proto_codegen.validate_remote_pins(
            valid, ["buf.build/protocolbuffers/go:v1.36.11"]
        )

        valid.write_text(
            "version: v2\nplugins:\n"
            "  - remote: buf.build/protocolbuffers/go:v1.36.11\n"
            "    out: go\n",
            encoding="utf-8",
        )
        with self.assertRaisesRegex(proto_codegen.CodegenError, "revision"):
            proto_codegen.validate_remote_pins(
                valid, ["buf.build/protocolbuffers/go:v1.36.11"]
            )

        valid.write_text(
            "version: v2\nplugins:\n"
            "  - remote: buf.build/protocolbuffers/go:v1.36.11\n"
            "    out: go\n"
            "  - remote: buf.build/grpc/go:v1.6.0\n"
            "    revision: 1\n    out: go\n",
            encoding="utf-8",
        )
        with self.assertRaisesRegex(proto_codegen.CodegenError, "protocolbuffers/go.*revision"):
            proto_codegen.validate_remote_pins(
                valid,
                [
                    "buf.build/protocolbuffers/go:v1.36.11",
                    "buf.build/grpc/go:v1.6.0",
                ],
            )

    def test_node_manifest_and_lock_are_exactly_pinned(self) -> None:
        tools = self.root / "tools"
        tools.mkdir()
        dependencies = dict(proto_codegen.NODE_DEPENDENCIES)
        (tools / "package.json").write_text(
            json.dumps({"devDependencies": dependencies}), encoding="utf-8"
        )
        (tools / "package-lock.json").write_text(
            json.dumps({"packages": {"": {"devDependencies": dependencies}}}),
            encoding="utf-8",
        )
        proto_codegen.validate_node_pins(tools)

        dependencies["protobufjs"] = "^7.6.3"
        (tools / "package.json").write_text(
            json.dumps({"devDependencies": dependencies}), encoding="utf-8"
        )
        with self.assertRaisesRegex(proto_codegen.CodegenError, "Node codegen pins"):
            proto_codegen.validate_node_pins(tools)

    @mock.patch("tools.proto_codegen.subprocess.run")
    @mock.patch("tools.proto_codegen.shutil.which", return_value="C:/bin/npm.cmd")
    def test_run_resolves_platform_command_shims(
        self, which: mock.Mock, run: mock.Mock
    ) -> None:
        run.return_value.returncode = 0
        run.return_value.stdout = ""
        run.return_value.stderr = ""

        proto_codegen._run(["npm", "ci"], self.root)

        which.assert_called_once_with("npm")
        self.assertEqual(run.call_args.args[0][0], "C:/bin/npm.cmd")

    @mock.patch("tools.proto_codegen._run")
    def test_schema_checks_include_origin_main_breaking(self, run: mock.Mock) -> None:
        # run_schema_checks now takes the toolchain, because which invariants are
        # checkable depends on the plugin-resolution mode.
        proto_codegen.run_schema_checks(codegen_toolchain.networked())

        commands = [call.args[0] for call in run.call_args_list]
        self.assertTrue(any("lint" in command for command in commands))
        self.assertTrue(any("build" in command for command in commands))
        self.assertTrue(
            any("breaking" in command and ".git#branch=origin/main" in command for command in commands)
        )

    @mock.patch("tools.proto_codegen._run")
    def test_schema_checks_skip_breaking_on_the_hermetic_path(self, run: mock.Mock) -> None:
        # `buf breaking --against origin/main` needs remote git history that a
        # --network=none container does not have. The hermetic path must still
        # lint and build, and must skip only the breaking check -- pin that so
        # the skip cannot silently widen to the other two.
        proto_codegen.run_schema_checks(codegen_toolchain.hermetic())

        commands = [call.args[0] for call in run.call_args_list]
        self.assertTrue(any("lint" in command for command in commands))
        self.assertTrue(any("build" in command for command in commands))
        self.assertFalse(any("breaking" in command for command in commands))

    def test_write_reconciles_only_generated_artifacts(self) -> None:
        expected = {"go": {Path("base.pb.go")}}
        generated = self.root / "temp" / "go" / "cordum" / "agent" / "v1"
        tracked = self.root / "cordum" / "agent" / "v1"
        generated.mkdir(parents=True)
        tracked.mkdir(parents=True)
        (generated / "base.pb.go").write_text("new", encoding="utf-8")
        (tracked / "old.pb.go").write_text("old", encoding="utf-8")
        (tracked / "README.md").write_text("keep", encoding="utf-8")

        proto_codegen.write_tracked(self.root / "temp", expected, self.root)

        self.assertEqual((tracked / "base.pb.go").read_text(), "new")
        self.assertFalse((tracked / "old.pb.go").exists())
        self.assertEqual((tracked / "README.md").read_text(), "keep")

    def test_repository_codegen_manifests_are_fully_pinned(self) -> None:
        proto_codegen.validate_remote_pins(proto_codegen.BUF_TEMPLATE)
        proto_codegen.validate_node_pins(proto_codegen.NODE_TOOLS)
        wrapper = (proto_codegen.REPO_ROOT / "tools/make_protos.sh").read_text()
        self.assertIn("proto_codegen.py", wrapper)
        self.assertNotIn("CAP_RUN_", wrapper)
        attributes = (proto_codegen.REPO_ROOT / ".gitattributes").read_text()
        self.assertIn("linguist-generated=true", attributes)
        self.assertIn("whitespace=-trailing-space", attributes)

    def test_codegen_ci_has_breaking_idempotence_and_compile_gates(self) -> None:
        workflow = (
            proto_codegen.REPO_ROOT / ".github/workflows/proto-codegen.yml"
        ).read_text(encoding="utf-8")
        for token in (
            "fetch-depth: 0",
            "test_*proto_codegen.py",
            "tools/make_protos.sh --check",
            "go test ./cordum/agent/v1",
            "cmake --build",
            "node --check",
            "readdirSync('./node/cordum/agent/v1')",
            "tsc --project tools/codegen/tsconfig.generated.json",
            "compileall",
            "PYTHONPATH=python",
            "PYTHONPATH=sdk/python/cap/pb",
            "ruby -c",
            "google-protobuf --version 4.33.2",
            "require 'cordum/agent/v1/buspacket_pb'",
            "require 'cordum/agent/v1/policy_pb'",
            "mvn --batch-mode",
            "cargo test",
        ):
            self.assertIn(token, workflow)
        self.assertNotIn("${{ runner.temp }}", workflow)
        self.assertIn(
            'echo "GEM_HOME=$RUNNER_TEMP/cap-ruby-gems" >> "$GITHUB_ENV"',
            workflow,
        )
        self.assertIn(
            'echo "GEM_PATH=$RUNNER_TEMP/cap-ruby-gems" >> "$GITHUB_ENV"',
            workflow,
        )

    def test_codegen_ci_rust_toolchain_matches_manifest_msrv(self) -> None:
        manifest = (proto_codegen.REPO_ROOT / "sdk/rust/Cargo.toml").read_text(
            encoding="utf-8"
        )
        workflow = (
            proto_codegen.REPO_ROOT / ".github/workflows/proto-codegen.yml"
        ).read_text(encoding="utf-8")

        match = re.search(r'^rust-version = "(\d+\.\d+)"$', manifest, re.MULTILINE)
        self.assertIsNotNone(match)
        assert match is not None
        msrv = match.group(1)
        self.assertEqual(msrv, "1.88")
        self.assertIn(f"dtolnay/rust-toolchain@{msrv}.0", workflow)


if __name__ == "__main__":
    unittest.main()
