"""Bounded, shell-free subprocess execution for registry verification."""

from __future__ import annotations

import os
import signal
import subprocess
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import BinaryIO, Mapping, Optional, Sequence

if __package__:
    from . import windows_job
else:
    import windows_job

MAX_OUTPUT_BYTES = 262_144


class CommandFailure(RuntimeError):
    """A bounded child process failed, timed out, or could not be observed."""


@dataclass(frozen=True)
class RunResult:
    argv: tuple[str, ...]
    returncode: int
    output: str


def _command_label(argv: Sequence[str]) -> str:
    return " ".join(repr(part) for part in argv)


def _drain_output(
    pipe: BinaryIO,
    tail: bytearray,
    truncated: list[bool],
    errors: list[Exception],
) -> None:
    try:
        while chunk := pipe.read(65_536):
            overflow = len(tail) + len(chunk) - MAX_OUTPUT_BYTES
            if overflow > 0:
                del tail[:overflow]
                truncated[0] = True
            tail.extend(chunk[-MAX_OUTPUT_BYTES:])
    except (OSError, ValueError) as error:
        errors.append(error)
    finally:
        try:
            pipe.close()
        except (OSError, ValueError) as error:
            errors.append(error)


def _captured_output(tail: bytearray, truncated: bool) -> str:
    output = bytes(tail).decode("utf-8", errors="replace")
    return ("[output truncated]\n" if truncated else "") + output


def _terminate_process_tree(
    process: subprocess.Popen[bytes], job_handle: Optional[int],
) -> str:
    tree_error = ""
    if job_handle is not None:
        try:
            windows_job.terminate(job_handle)
        except OSError as error:
            tree_error = f"job termination failed: {error}"
    elif os.name == "nt":
        system_root = Path(os.environ.get("SystemRoot", r"C:\Windows"))
        taskkill = system_root / "System32" / "taskkill.exe"
        try:
            result = subprocess.run(
                [str(taskkill), "/PID", str(process.pid), "/T", "/F"],
                stdin=subprocess.DEVNULL,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                timeout=10,
                check=False,
                shell=False,
            )
            if result.returncode != 0:
                tree_error = f"taskkill exited {result.returncode}"
        except (OSError, subprocess.TimeoutExpired) as error:
            tree_error = f"taskkill failed: {error}"
    else:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        except OSError as error:
            tree_error = f"killpg failed: {error}"
    if process.poll() is None:
        process.kill()
    try:
        process.wait(timeout=10)
    except subprocess.TimeoutExpired:
        tree_error = f"{tree_error}; process did not exit".strip("; ")
    return tree_error


def _start_process(
    command: tuple[str, ...], cwd: Path, env: Mapping[str, str]
) -> tuple[subprocess.Popen[bytes], Optional[int]]:
    try:
        process = subprocess.Popen(
            list(command),
            cwd=str(cwd),
            env=dict(env),
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            shell=False,
            creationflags=subprocess.CREATE_NEW_PROCESS_GROUP if os.name == "nt" else 0,
            start_new_session=os.name != "nt",
        )
    except OSError as error:
        raise CommandFailure(f"cannot start command {_command_label(command)}: {error}") from error
    try:
        return process, windows_job.assign_kill_on_close(process.pid)
    except OSError as error:
        detail = _terminate_process_tree(process, None)
        suffix = f"; {detail}" if detail else ""
        raise CommandFailure(f"cannot contain command {_command_label(command)}: {error}{suffix}") from error


def run_command(
    argv: Sequence[str], cwd: Path, env: Mapping[str, str], timeout: int
) -> RunResult:
    command = tuple(str(part) for part in argv)
    process, job_handle = _start_process(command, cwd, env)
    if process.stdout is None:
        _terminate_process_tree(process, job_handle)
        windows_job.close(job_handle)
        raise CommandFailure(f"cannot capture command output: {_command_label(command)}")
    tail, truncated, errors = bytearray(), [False], []
    reader = threading.Thread(
        target=_drain_output, args=(process.stdout, tail, truncated, errors), daemon=True
    )
    reader.start()
    deadline = time.monotonic() + timeout
    process_timeout, pipe_timeout, cleanup_error = False, False, ""
    try:
        process.wait(timeout=max(0.0, deadline - time.monotonic()))
    except subprocess.TimeoutExpired:
        process_timeout = True
        cleanup_error = _terminate_process_tree(process, job_handle)
    reader.join(timeout=max(0.0, deadline - time.monotonic()))
    if reader.is_alive():
        pipe_timeout = True
        detail = _terminate_process_tree(process, job_handle)
        cleanup_error = cleanup_error or detail
    try:
        windows_job.close(job_handle)
    except OSError as error:
        cleanup_error = cleanup_error or f"job close failed: {error}"
    if os.name != "nt" and not process_timeout and not pipe_timeout:
        cleanup_error = _terminate_process_tree(process, None)
    reader.join(timeout=5)
    if reader.is_alive():
        errors.append(OSError("output reader did not terminate after tree cleanup"))
    output = _captured_output(tail, truncated[0])
    if errors:
        raise CommandFailure(
            f"cannot capture command output for {_command_label(command)}: {errors[0]}"
        )
    if process_timeout or pipe_timeout:
        summary = output.strip().replace("\n", " | ")
        detail = f"; {cleanup_error}" if cleanup_error else ""
        reason = "command timed out" if process_timeout else "command output remained open"
        raise CommandFailure(
            f"{reason} after {timeout}s: {_command_label(command)}: {summary}{detail}"
        )
    if cleanup_error:
        raise CommandFailure(f"command cleanup failed for {_command_label(command)}: {cleanup_error}")
    return RunResult(command, int(process.returncode), output)
