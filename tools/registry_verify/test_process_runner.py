from __future__ import annotations

import io
import os
import sys
import tempfile
import time
import unittest
from pathlib import Path
from unittest import mock

from tools.registry_verify import go_consumer
from tools.registry_verify import process_runner

EXPECTED_HELPER = Path(__file__).with_name("process_runner.py").resolve()
if Path(process_runner.__file__).resolve() != EXPECTED_HELPER:
    raise ImportError(f"loaded non-local process_runner helper: {process_runner.__file__}")


class ProcessRunnerTests(unittest.TestCase):
    @mock.patch("tools.registry_verify.process_runner.subprocess.Popen")
    def test_runner_uses_argv_without_a_shell(self, popen: mock.Mock) -> None:
        process = popen.return_value
        process.pid = 2_147_483_647
        process.stdout = io.BytesIO(b"ok\n")
        process.wait.return_value = 0
        process.returncode = 0
        with mock.patch.object(process_runner.windows_job, "assign_kill_on_close", return_value=None):
            result = process_runner.run_command(
                [sys.executable, "-c", "print('ok')"], Path.cwd(), {}, 5
            )
        self.assertEqual(result.output, "ok\n")
        self.assertIs(popen.call_args.kwargs["shell"], False)
        self.assertNotIn("cmd", popen.call_args.args[0][0].lower())

    @mock.patch(
        "tools.registry_verify.process_runner.subprocess.Popen",
        side_effect=OSError("spawn denied"),
    )
    def test_spawn_error_has_command_provenance(self, _popen: mock.Mock) -> None:
        with self.assertRaisesRegex(process_runner.CommandFailure, "cannot start.*probe"):
            process_runner.run_command(["missing-go", "probe"], Path.cwd(), {}, 5)

    def test_timeout_and_failure_include_bounded_provenance(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            cwd = Path(temporary)
            with self.assertRaisesRegex(process_runner.CommandFailure, "timed out"):
                process_runner.run_command(
                    [sys.executable, "-c", "import time; time.sleep(2)"], cwd, {}, 1
                )
            failed = process_runner.run_command(
                [sys.executable, "-c", "import sys; print('boom'); sys.exit(7)"],
                cwd,
                {},
                5,
            )
        self.assertEqual(failed.returncode, 7)
        self.assertIn("boom", failed.output)

    def test_checked_failure_includes_exit_and_output(self) -> None:
        result = process_runner.RunResult(("go", "probe"), 7, "boom\n")
        with self.assertRaisesRegex(process_runner.CommandFailure, "exit 7.*boom"):
            go_consumer._require_success(result)

    def test_runner_preserves_normal_output(self) -> None:
        payload = "x" * 20_000
        with tempfile.TemporaryDirectory() as temporary:
            result = process_runner.run_command(
                [sys.executable, "-c", f"print({payload!r})"], Path(temporary), {}, 5
            )
        self.assertNotIn("[output truncated]", result.output)
        self.assertEqual(result.output.strip(), payload)

    @mock.patch("tempfile.TemporaryFile", side_effect=AssertionError("unbounded spool"))
    def test_large_output_uses_bounded_memory_not_a_temp_spool(self, _temporary: mock.Mock) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            result = process_runner.run_command(
                [sys.executable, "-c", "import sys; sys.stdout.write('x' * 400000)"],
                Path(temporary),
                {},
                5,
            )
        self.assertTrue(result.output.startswith("[output truncated]"))
        self.assertLessEqual(
            len(result.output.encode()), process_runner.MAX_OUTPUT_BYTES + 32
        )

    def test_timeout_kills_descendant_processes(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            marker = root / "descendant-survived"
            started = root / "descendant-started"
            grandchild = (
                "import pathlib,time; time.sleep(1.5); "
                f"pathlib.Path({str(marker)!r}).write_text('alive')"
            )
            parent = (
                "import pathlib,subprocess,sys,time; "
                f"subprocess.Popen([sys.executable,'-c',{grandchild!r}]); "
                f"pathlib.Path({str(started)!r}).write_text('started'); time.sleep(30)"
            )
            with self.assertRaises(process_runner.CommandFailure):
                process_runner.run_command([sys.executable, "-c", parent], root, os.environ, 1)
            self.assertTrue(started.exists(), "test never reached descendant spawn")
            time.sleep(1)
            self.assertFalse(marker.exists(), "timed-out descendant remained alive")

    def test_exited_parent_cannot_leave_pipe_holding_descendant(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            started, marker = root / "child-started", root / "child-survived"
            child = (
                "import pathlib,time; time.sleep(3); "
                f"pathlib.Path({str(marker)!r}).write_text('alive')"
            )
            parent = (
                "import pathlib,subprocess,sys; "
                f"subprocess.Popen([sys.executable,'-c',{child!r}]); "
                f"pathlib.Path({str(started)!r}).write_text('started')"
            )
            began = time.monotonic()
            with self.assertRaises(process_runner.CommandFailure):
                process_runner.run_command([sys.executable, "-c", parent], root, os.environ, 1)
            self.assertLess(time.monotonic() - began, 2.5)
            self.assertTrue(started.exists(), "test never reached descendant spawn")
            time.sleep(2.5)
            self.assertFalse(marker.exists(), "exited parent left a surviving descendant")

    @mock.patch("tools.registry_verify.process_runner.subprocess.Popen")
    def test_output_read_error_fails_closed(self, popen: mock.Mock) -> None:
        class BrokenPipe(io.BytesIO):
            def read(self, size: int = -1) -> bytes:
                raise OSError("capture denied")

        process = popen.return_value
        process.pid = 2_147_483_647
        process.stdout = BrokenPipe()
        process.wait.return_value = 0
        process.returncode = 0
        with mock.patch.object(process_runner.windows_job, "assign_kill_on_close", return_value=None):
            with self.assertRaisesRegex(process_runner.CommandFailure, "capture denied"):
                process_runner.run_command([sys.executable, "-V"], Path.cwd(), {}, 5)


if __name__ == "__main__":
    unittest.main()
