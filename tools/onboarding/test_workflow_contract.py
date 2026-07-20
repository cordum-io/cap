"""Static contracts for the installed-artifact onboarding CI gate."""

from __future__ import annotations

import os
import re
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
CI = ROOT / ".github" / "workflows" / "ci.yml"
WORKFLOW = ROOT / ".github" / "workflows" / "onboarding.yml"
GUIDE = ROOT / "docs" / "getting-started.md"
TROUBLESHOOTING = ROOT / "docs" / "troubleshooting.md"
CLEANUP = ROOT / "tools" / "onboarding" / "cleanup_runtime.sh"


def bash_executable() -> str:
    git_bash = Path(os.environ.get("ProgramFiles", "")) / "Git" / "bin" / "bash.exe"
    if git_bash.is_file():
        return str(git_bash)
    executable = shutil.which("bash")
    if executable is None:
        raise RuntimeError("bash is required for onboarding contract tests")
    return executable


def python_artifact_selector(guide: str) -> str:
    lines = guide.splitlines()
    start = next(
        index for index, line in enumerate(lines) if "CAP_PYTHON_ARTIFACT=" in line
    )
    command: list[str] = []
    for line in lines[start:]:
        command.append(line)
        if line.rstrip().endswith(("\\", "&&")):
            continue
        next_index = start + len(command)
        if next_index < len(lines) and lines[next_index].startswith(
            ("test -n ", "export CAP_PYTHON_ARTIFACT")
        ):
            continue
        break
    return "\n".join(command)


class OnboardingWorkflowContractTests(unittest.TestCase):
    ci: str
    guide: str
    troubleshooting: str
    cleanup: str
    workflow: str

    @classmethod
    def setUpClass(cls) -> None:
        cls.ci = CI.read_text(encoding="utf-8")
        cls.guide = GUIDE.read_text(encoding="utf-8")
        cls.troubleshooting = TROUBLESHOOTING.read_text(encoding="utf-8")
        cls.cleanup = CLEANUP.read_text(encoding="utf-8") if CLEANUP.is_file() else ""
        cls.workflow = (
            WORKFLOW.read_text(encoding="utf-8") if WORKFLOW.is_file() else ""
        )

    def test_required_ci_calls_reusable_gate(self) -> None:
        self.assertIn("uses: ./.github/workflows/onboarding.yml", self.ci)
        self.assertNotIn("continue-on-error", self.workflow)

    def test_three_language_matrix_has_hard_budget(self) -> None:
        self.assertRegex(
            self.workflow,
            r"language:\s*\[go, python, node\]",
        )
        self.assertIn("fail-fast: false", self.workflow)
        self.assertIn("timeout-minutes: 10", self.workflow)
        self.assertIn("workflow_call:", self.workflow)

    def test_bootstrap_actions_are_immutable(self) -> None:
        pins = (
            "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683",
            "actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065",
            "actions/setup-go@d35c59abb061a4a6fb18e82ac0862c26744d6ab5",
            "actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020",
        )
        for pin in pins:
            with self.subTest(pin=pin):
                self.assertIn(pin, self.workflow)
        self.assertNotRegex(self.workflow, r"actions/[a-z-]+@v\d")

    def test_public_broker_command_uses_the_ci_digest(self) -> None:
        digest = "nats:2.12.6-alpine@sha256:1cfc36e2e5e638243d8c722f72c954cd0ec4b15ee82fadbc718ce12e2b3c1652"
        self.assertIn(digest, self.guide)
        self.assertIn(digest, self.troubleshooting)
        self.assertIn(digest, self.workflow)

    def test_cancellation_cleanup_and_diagnostics_are_durable(self) -> None:
        self.assertRegex(
            self.workflow,
            r"(?m)^  smoke:\n    if: \$\{\{ always\(\) && needs\.contracts\.result == 'success' \}\}",
        )
        self.assertRegex(
            self.ci,
            r"(?m)^  onboarding:\n    name:.*\n    if: \$\{\{ always\(\) \}\}",
        )
        self.assertIn("Always capture onboarding state and clean up", self.workflow)
        self.assertIn('echo "$worker_pid" > "$logs/worker.pid"', self.workflow)
        self.assertIn("tools/onboarding/cleanup_runtime.sh", self.workflow)
        self.assertIn("github.run_attempt", self.workflow)

    def test_sync_and_exact_canonical_files_are_used(self) -> None:
        required = (
            "tools/onboarding/sync_snippets.py --check",
            "examples/simple-echo/go-worker/main.go",
            "examples/simple-echo/go-client/main.go",
            "examples/simple-echo/python-worker/main.py",
            "examples/simple-echo/python-client/main.py",
            "examples/simple-echo/node-worker/main.js",
            "examples/simple-echo/node-client/main.js",
        )
        for value in required:
            with self.subTest(value=value):
                self.assertIn(value, self.workflow)
        self.assertIn("$RUNNER_TEMP/cap-onboarding-consumer", self.workflow)

    def test_go_uses_only_local_proxy_candidate_for_cap(self) -> None:
        required = (
            "tools/onboarding/build_go_proxy.py",
            "GOWORK=off",
            "GOMODCACHE=",
            "GOPROXY=",
            "GONOSUMDB=github.com/cordum-io/cap/v2",
            "go list -m",
            "go mod tidy",
            'GOPROXY="$proxy_url"',
            "go mod download -json",
            "hashlib.sha256",
        )
        for value in required:
            with self.subTest(value=value):
                self.assertIn(value, self.workflow)
        self.assertNotRegex(self.workflow, r"(?m)^\s*replace\s+")

    def test_go_dependencies_are_resolved_after_both_sources_exist(self) -> None:
        client_end = self.guide.find("<!-- cap-snippet:go-client:end -->")
        dependency_install = self.guide.find(
            'go get "github.com/cordum-io/cap/v2@$CAP_GO_VERSION"'
        )
        local_download = self.guide.find(
            'go mod download "github.com/cordum-io/cap/v2@$CAP_GO_VERSION"'
        )
        self.assertGreater(client_end, 0)
        self.assertGreater(local_download, client_end)
        self.assertLess(local_download, dependency_install)
        self.assertGreater(dependency_install, client_end)
        self.assertIn("go mod tidy", self.guide[client_end:dependency_install + 1000])

    def test_python_builds_both_artifacts_and_isolates_imports(self) -> None:
        required = (
            "python -m build",
            "*.whl",
            "*.tar.gz",
            "--no-cache-dir",
            "pip check",
            "env -u PYTHONPATH",
            " -I ",
        )
        for value in required:
            with self.subTest(value=value):
                self.assertIn(value, self.workflow)

    def test_python_guide_rejects_stale_or_ambiguous_wheels(self) -> None:
        self.assertIn("rm -rf .artifacts/python", self.guide)
        self.assertIn('(artifact,) = sorted(Path(".artifacts/python").glob("*.whl"))', self.guide)

    def test_python_artifact_selection_failure_reaches_the_shell(self) -> None:
        selector = python_artifact_selector(self.guide)
        for wheel_count in (0, 2):
            with self.subTest(wheel_count=wheel_count):
                with tempfile.TemporaryDirectory() as temporary:
                    root = Path(temporary)
                    artifacts = root / ".artifacts" / "python"
                    artifacts.mkdir(parents=True)
                    for index in range(wheel_count):
                        (artifacts / f"candidate-{index}.whl").touch()
                    result = subprocess.run(
                        [bash_executable(), "-c", selector],
                        cwd=root,
                        check=False,
                        capture_output=True,
                        text=True,
                    )
                    self.assertNotEqual(result.returncode, 0)

    def test_node_installs_tarball_without_source_path(self) -> None:
        required = (
            "npm ci",
            "npm run build",
            "npm pack",
            "*.tgz",
            "npm install",
            "npm ls --omit=dev",
            "env -u NODE_PATH",
        )
        for value in required:
            with self.subTest(value=value):
                self.assertIn(value, self.workflow)

    def test_broker_and_worker_readiness_are_bounded_and_exact(self) -> None:
        required = (
            "nats:2.12.6-alpine@sha256:",
            "-m 8222",
            "/healthz",
            "/subsz?subs=1",
            'entry.get("subject") == "job.echo"',
            "CAP_NATS_URL: nats://127.0.0.1:4222",
        )
        for value in required:
            with self.subTest(value=value):
                self.assertIn(value, self.workflow)
        self.assertRegex(self.workflow, r"for attempt in \{1\.\.[0-9]+\}")

    def test_no_worker_must_fail_before_success_can_pass(self) -> None:
        negative = self.workflow.find("no_worker_status")
        worker = self.workflow.find("run_worker >")
        self.assertGreaterEqual(negative, 0)
        self.assertGreater(worker, negative)
        self.assertIn('test "$no_worker_status" -ne 0', self.workflow)
        self.assertIn('! grep -q "CAP_SIMPLE_ECHO_SUCCESS"', self.workflow)
        self.assertIn('test "$success_count" -eq 1', self.workflow)

    def test_processes_are_cleaned_and_failure_logs_are_archived(self) -> None:
        required = (
            "trap",
            "actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02",
            "if: ${{ failure() || cancelled() }}",
        )
        for value in required:
            with self.subTest(value=value):
                self.assertIn(value, self.workflow)
        self.assertIn('kill "$worker_pid"', self.cleanup)
        self.assertIn('wait "$worker_pid"', self.workflow)
        self.assertIn("timeout 20s docker logs", self.cleanup)
        self.assertIn("timeout 30s docker rm -f", self.cleanup)

    def test_repeated_cleanup_preserves_first_diagnostics(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bin_dir = root / "bin"
            logs = root / "logs"
            state = root / "removed"
            bin_dir.mkdir()
            docker = bin_dir / "docker"
            docker.write_text(
                "#!/usr/bin/env bash\n"
                "if [[ $1 == logs ]]; then\n"
                "  [[ -e $FAKE_DOCKER_STATE ]] && echo SECOND_NATS || echo FIRST_NATS\n"
                "else\n"
                "  [[ -e $FAKE_DOCKER_STATE ]] && echo SECOND_RM || echo FIRST_RM\n"
                "  touch \"$FAKE_DOCKER_STATE\"\n"
                "fi\n",
                encoding="utf-8",
            )
            docker.chmod(0o755)
            environment = os.environ.copy()
            environment["PATH"] = f"{bin_dir}{os.pathsep}{environment['PATH']}"
            environment["FAKE_DOCKER_STATE"] = state.as_posix()
            command = [bash_executable(), CLEANUP.as_posix(), logs.as_posix(), "cap-test"]
            subprocess.run(command, check=True, env=environment)
            first = (
                (logs / "final-nats.log").read_text(encoding="utf-8"),
                (logs / "final-rm.log").read_text(encoding="utf-8"),
            )
            subprocess.run(command, check=True, env=environment)
            self.assertEqual(first[0], "FIRST_NATS\n")
            self.assertEqual(first[1], "FIRST_RM\n")
            self.assertEqual((logs / "final-nats.log").read_text(), first[0])
            self.assertEqual((logs / "final-rm.log").read_text(), first[1])


if __name__ == "__main__":
    unittest.main()
