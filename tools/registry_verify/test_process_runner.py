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
    @mock.patch("tools.registry_verify.process_runner._start_process")
    @mock.patch(
        "tools.registry_verify.process_runner._terminate_process_tree", return_value=""
    )
    @mock.patch("tools.registry_verify.process_runner.windows_job.close")
    @mock.patch(
        "tools.registry_verify.process_runner.threading.Thread.start",
        side_effect=RuntimeError("thread unavailable"),
    )
    def test_reader_start_failure_cleans_process_job_and_pipe(
        self,
        _start_reader: mock.Mock,
        close_job: mock.Mock,
        terminate: mock.Mock,
        start_process: mock.Mock,
    ) -> None:
        process = mock.Mock()
        process.stdout = io.BytesIO(b"")
        start_process.return_value = (process, 123)

        with self.assertRaisesRegex(
            process_runner.CommandFailure, "output reader.*thread unavailable"
        ):
            process_runner.run_command(["go", "version"], Path.cwd(), {}, 5)

        terminate.assert_called_once_with(process, 123)
        close_job.assert_called_once_with(123)
        self.assertTrue(process.stdout.closed)

    @mock.patch("tools.registry_verify.process_runner.subprocess.Popen")
    def test_runner_uses_argv_without_a_shell(self, popen: mock.Mock) -> None:
        process = popen.return_value
        process.pid = 2_147_483_647
        process.stdout = io.BytesIO(b"ok\n")
        process.wait.return_value = 0
        process.returncode = 0
        command = [sys.executable, "-c", "print('path with spaces @v2.16.1')"]
        with mock.patch.object(
            process_runner.windows_job, "assign_kill_on_close", return_value=None
        ):
            result = process_runner.run_command(
                command, Path.cwd(), {}, 5
            )
        self.assertEqual(result.output, "ok\n")
        self.assertIs(popen.call_args.kwargs["shell"], False)
        self.assertNotIn("cmd", popen.call_args.args[0][0].lower())
        launched = popen.call_args.args[0]
        if os.name == "nt":
            self.assertEqual(
                launched,
                [
                    sys.executable,
                    "-I",
                    "-S",
                    str(Path(process_runner.windows_job.__file__).resolve()),
                    *command,
                ],
            )
            process.stdin.write.assert_called_once_with(
                process_runner.windows_job.START_BARRIER
            )
        else:
            self.assertEqual(launched, command)

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

    @unittest.skipUnless(os.name == "nt", "Windows Job Object containment")
    def test_windows_contains_child_spawned_during_assignment_delay(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            started, marker = root / "child-started", root / "child-escaped"
            preassignment_started: list[bool] = []
            child = (
                "import pathlib,time; time.sleep(2); "
                f"pathlib.Path({str(marker)!r}).write_text('escaped')"
            )
            parent = (
                "import pathlib,subprocess,sys,time; "
                f"subprocess.Popen([sys.executable,'-c',{child!r}],"
                "stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL); "
                f"pathlib.Path({str(started)!r}).write_text('started'); time.sleep(1)"
            )
            assign = process_runner.windows_job.assign_kill_on_close

            def delayed_assignment(pid: int) -> int | None:
                deadline = time.monotonic() + 2
                while not started.exists() and time.monotonic() < deadline:
                    time.sleep(0.02)
                preassignment_started.append(started.exists())
                return assign(pid)

            with mock.patch.object(
                process_runner.windows_job,
                "assign_kill_on_close",
                side_effect=delayed_assignment,
            ):
                result = process_runner.run_command(
                    [sys.executable, "-c", parent], root, os.environ, 5
                )

            self.assertEqual(result.returncode, 0)
            self.assertEqual(preassignment_started, [False])
            self.assertTrue(started.exists(), "test never reached child spawn")
            time.sleep(1.5)
            self.assertFalse(marker.exists(), "pre-assignment child escaped the Job Object")

    @unittest.skipUnless(os.name == "nt", "Windows Job Object containment")
    def test_windows_assignment_failure_never_releases_target(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            marker = root / "target-ran"
            command = [
                sys.executable,
                "-c",
                f"import pathlib; pathlib.Path({str(marker)!r}).write_text('ran')",
            ]
            with mock.patch.object(
                process_runner.windows_job,
                "assign_kill_on_close",
                side_effect=OSError("assignment denied"),
            ):
                with self.assertRaisesRegex(
                    process_runner.CommandFailure, "cannot contain.*assignment denied"
                ):
                    process_runner.run_command(command, root, os.environ, 5)
            time.sleep(0.25)
            self.assertFalse(marker.exists(), "target ran without Job assignment")

    @unittest.skipUnless(os.name == "nt", "Windows contained launcher")
    def test_windows_launcher_preserves_exact_argv_without_shell(self) -> None:
        command = [r"C:\Program Files\Go\bin\go.exe", "get", "cap@v2.16.1"]
        stdin = mock.Mock()
        stdin.buffer = io.BytesIO(process_runner.windows_job.START_BARRIER)
        completed = mock.Mock(returncode=23)
        with (
            mock.patch.object(process_runner.windows_job.sys, "stdin", stdin),
            mock.patch.object(
                process_runner.windows_job.subprocess,
                "run",
                return_value=completed,
            ) as run,
        ):
            result = process_runner.windows_job.run_after_barrier(command)
        self.assertEqual(result, 23)
        run.assert_called_once_with(
            command,
            stdin=process_runner.windows_job.subprocess.DEVNULL,
            check=False,
            shell=False,
        )

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
