"""Bounded child process for one CrewAI job (docs/adoption/framework-adapters-design.md section 3.6).

The parent never imports crewai. Each job runs in a fresh child: an explicit argv with no shell,
an allowlisted environment, a new temp dir as its cwd and TMP/TEMP/TMPDIR, and one JSON job line
on stdin, written only after the child's process tree is contained (a new session on POSIX, a
kill-on-close Job Object on Windows). The child answers with JSONL events on stdout. Every exit
path kills the whole tree first and then removes the temp dir.
"""

from __future__ import annotations

import asyncio
import json
import os
import shutil
import signal
import subprocess
import tempfile
import time
from dataclasses import dataclass
from typing import Any, Awaitable, Callable, Dict, Mapping, Optional, Sequence

from cap import JobFailure

CHILD_CRASHED = "CHILD_CRASHED"
LINE_LIMIT = 1 << 20
EVENT_TYPES = ("progress", "artifact", "result", "error")
FORCED_ENV = {
    "OTEL_SDK_DISABLED": "true",
    "CREWAI_DISABLE_TELEMETRY": "true",
    "CREWAI_DISABLE_TRACKING": "true",
    "CREWAI_TRACING_ENABLED": "false",
    "CREWAI_DISABLE_VERSION_CHECK": "true",
}
_PASSED = ("PATH", "SYSTEMROOT", "PYTHONPATH") + (
    ("APPDATA", "LOCALAPPDATA", "USERPROFILE") if os.name == "nt" else ())
_REAP_SECONDS = 5.0
_PIPE_SECONDS = 1.0


@dataclass(frozen=True)
class ChildLimits:
    """A workload's "child" block: graceMs, maxStdoutBytes and maxStderrBytes."""

    grace_ms: int = 2000
    max_stdout_bytes: int = 1 << 20
    max_stderr_bytes: int = 64 << 10


def child_env(tmp: str) -> Dict[str, str]:
    """The child's whole environment: the allowlist, the job temp dir and the forced switches."""
    env = {name: os.environ[name] for name in _PASSED if name in os.environ}
    env.update(TMP=tmp, TEMP=tmp, TMPDIR=tmp)
    if os.name != "nt":  # Windows storage stays under APPDATA: overriding it breaks import crewai
        env["XDG_DATA_HOME"] = tmp
    env.update(FORCED_ENV)
    return env


if os.name == "nt":
    import ctypes
    from ctypes import wintypes

    class _BasicLimits(ctypes.Structure):
        _fields_ = [("PerProcessUserTimeLimit", ctypes.c_int64), ("PerJobUserTimeLimit", ctypes.c_int64),
                    ("LimitFlags", wintypes.DWORD), ("MinimumWorkingSetSize", ctypes.c_size_t),
                    ("MaximumWorkingSetSize", ctypes.c_size_t), ("ActiveProcessLimit", wintypes.DWORD),
                    ("Affinity", ctypes.c_size_t), ("PriorityClass", wintypes.DWORD),
                    ("SchedulingClass", wintypes.DWORD)]

    class _ExtendedLimits(ctypes.Structure):
        _fields_ = [("BasicLimitInformation", _BasicLimits), ("IoInfo", ctypes.c_uint64 * 6),
                    ("ProcessMemoryLimit", ctypes.c_size_t), ("JobMemoryLimit", ctypes.c_size_t),
                    ("PeakProcessMemoryUsed", ctypes.c_size_t), ("PeakJobMemoryUsed", ctypes.c_size_t)]

    class _Accounting(ctypes.Structure):
        _fields_ = [("Times", ctypes.c_int64 * 4), ("TotalPageFaultCount", wintypes.DWORD),
                    ("TotalProcesses", wintypes.DWORD), ("ActiveProcesses", wintypes.DWORD),
                    ("TotalTerminatedProcesses", wintypes.DWORD)]

    _K32 = ctypes.WinDLL("kernel32", use_last_error=True)
    _K32.CreateJobObjectW.argtypes = (ctypes.c_void_p, wintypes.LPCWSTR)
    _K32.CreateJobObjectW.restype = wintypes.HANDLE
    _K32.SetInformationJobObject.argtypes = (wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p, wintypes.DWORD)
    _K32.QueryInformationJobObject.argtypes = (wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p,
                                               wintypes.DWORD, ctypes.c_void_p)
    _K32.OpenProcess.argtypes = (wintypes.DWORD, wintypes.BOOL, wintypes.DWORD)
    _K32.OpenProcess.restype = wintypes.HANDLE
    _K32.AssignProcessToJobObject.argtypes = (wintypes.HANDLE, wintypes.HANDLE)
    _K32.TerminateJobObject.argtypes = (wintypes.HANDLE, wintypes.UINT)
    _K32.CloseHandle.argtypes = (wintypes.HANDLE,)
    _SPAWN: Dict[str, Any] = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP}
else:
    _SPAWN = {"start_new_session": True}


class _Tree:
    """Containment for one child's whole process tree, set up before the job line is written."""

    def __init__(self, proc: asyncio.subprocess.Process) -> None:
        self.proc = proc
        self.job: Optional[int] = _job_for(proc.pid) if os.name == "nt" else None

    async def kill(self, grace: float) -> None:
        if os.name == "nt":
            _K32.TerminateJobObject(self.job, 1)
            await _until(lambda: _active(self.job) == 0, _REAP_SECONDS)
            _K32.CloseHandle(self.job)
        else:
            _killpg(self.proc.pid, signal.SIGTERM)
            await _until(lambda: self.proc.returncode is not None, grace)
            _killpg(self.proc.pid, signal.SIGKILL)
        await _until(lambda: self.proc.returncode is not None, _REAP_SECONDS)


def _job_for(pid: int) -> int:
    job = _K32.CreateJobObjectW(None, None)
    if not job:
        raise ctypes.WinError(ctypes.get_last_error())
    limits = _ExtendedLimits()
    limits.BasicLimitInformation.LimitFlags = 0x2000  # JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
    process = _K32.OpenProcess(0x0101, False, pid)  # PROCESS_TERMINATE | PROCESS_SET_QUOTA
    ok = bool(process) and bool(_K32.SetInformationJobObject(job, 9, ctypes.byref(limits), ctypes.sizeof(limits))) \
        and bool(_K32.AssignProcessToJobObject(job, process))
    error = ctypes.get_last_error()
    if process:
        _K32.CloseHandle(process)
    if not ok:
        _K32.CloseHandle(job)
        raise ctypes.WinError(error)
    return job


def _active(job: int) -> int:
    info = _Accounting()
    if not _K32.QueryInformationJobObject(job, 1, ctypes.byref(info), ctypes.sizeof(info), None):
        return 0
    return info.ActiveProcesses


def _killpg(pgid: int, sig: int) -> None:
    try:
        os.killpg(pgid, sig)
    except (ProcessLookupError, PermissionError):  # macOS answers EPERM when no live member is left
        pass


async def _until(done: Callable[[], bool], seconds: float) -> bool:
    end = time.monotonic() + seconds
    while not done():
        if time.monotonic() >= end:
            return False
        await asyncio.sleep(0.01)
    return True


async def _drain(stderr: asyncio.StreamReader, cap: int) -> bytes:
    """Keep a bounded prefix of stderr, never parsed, logged or published; discard the rest."""
    kept = bytearray()
    while True:
        chunk = await stderr.read(65536)
        if not chunk:
            return bytes(kept)
        kept += chunk[: max(0, cap - len(kept))]


def _event(line: bytes) -> Dict[str, Any]:
    try:
        event = json.loads(line)
    except (ValueError, RecursionError):
        event = None
    if not isinstance(event, dict) or event.get("type") not in EVENT_TYPES:
        raise JobFailure(CHILD_CRASHED)
    return event


async def _events(stdout: asyncio.StreamReader, budget: int,
                  on_event: Callable[[Dict[str, Any]], Awaitable[None]]) -> Dict[str, Any]:
    total = 0
    while True:
        try:
            line = await stdout.readline()
        except ValueError:  # a line over LINE_LIMIT
            raise JobFailure(CHILD_CRASHED) from None
        total += len(line)
        if not line or total > budget:
            raise JobFailure(CHILD_CRASHED)
        if not line.strip():
            continue
        event = _event(line)
        if event["type"] in ("result", "error"):
            return event
        await on_event(event)


async def run_child(argv: Sequence[str], job: Mapping[str, Any], limits: ChildLimits,
                    on_event: Callable[[Dict[str, Any]], Awaitable[None]]) -> Dict[str, Any]:
    """Run one job in a fresh contained child and return its terminal (result or error) event.

    progress and artifact events go to on_event in order, each awaited before the next line is
    read. After its terminal event the child must exit 0 within graceMs. A spawn or containment
    failure, an exit without a terminal event, a line over 1 MiB, stdout over maxStdoutBytes or
    a line that is not a known event raises JobFailure("CHILD_CRASHED").
    """
    tmp = tempfile.mkdtemp(prefix="cap-crewai-")
    proc: Optional[asyncio.subprocess.Process] = None
    tree: Optional[_Tree] = None
    drain: Optional[asyncio.Future] = None
    try:
        try:
            proc = await asyncio.create_subprocess_exec(
                *argv, stdin=asyncio.subprocess.PIPE, stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE, cwd=tmp, env=child_env(tmp), limit=LINE_LIMIT, **_SPAWN)
            tree = _Tree(proc)
        except OSError:
            raise JobFailure(CHILD_CRASHED) from None
        drain = asyncio.ensure_future(_drain(proc.stderr, limits.max_stderr_bytes))
        try:
            proc.stdin.write(json.dumps(job).encode("ascii") + b"\n")
            await proc.stdin.drain()
            proc.stdin.close()
        except OSError:
            raise JobFailure(CHILD_CRASHED) from None
        event = await _events(proc.stdout, limits.max_stdout_bytes, on_event)
        exited = await _until(lambda: proc.returncode is not None, limits.grace_ms / 1000)
        if not exited or proc.returncode != 0:
            raise JobFailure(CHILD_CRASHED)
        return event
    finally:
        await asyncio.shield(_cleanup(proc, tree, drain, tmp, limits.grace_ms / 1000))


async def _cleanup(proc: Optional[asyncio.subprocess.Process], tree: Optional[_Tree],
                   drain: Optional[asyncio.Future], tmp: str, grace: float) -> None:
    try:
        if tree is not None:
            await tree.kill(grace)
        elif proc is not None:  # containment failed: the child alone exists, kill it
            try:
                proc.kill()
            except ProcessLookupError:
                pass
            await _until(lambda: proc.returncode is not None, _REAP_SECONDS)
        if proc is not None:
            # Python 3.11+ wait() also waits for every pipe holder; a lingering one must not hang teardown.
            try:
                await asyncio.wait_for(proc.wait(), _PIPE_SECONDS)
            except asyncio.TimeoutError:
                pass
        if drain is not None:
            drain.cancel()
            await asyncio.gather(drain, return_exceptions=True)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
