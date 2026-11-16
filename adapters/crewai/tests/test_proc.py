"""Child lifecycle tests for cap_adapter_crewai._proc; CI runs them on Linux, macOS and Windows."""
import asyncio
import os
import sys
import tempfile
import time
from pathlib import Path

import pytest
from cap import JobFailure

from cap_adapter_crewai import _proc

CHILD = Path(__file__).parent / "fixtures" / "lifecycle_child.py"
TRAP = Path(__file__).resolve().parents[3] / "testdata" / "adapters" / "nettrap"
FAST = _proc.ChildLimits(grace_ms=300)

if os.name == "nt":
    import ctypes

    _K32 = ctypes.WinDLL("kernel32", use_last_error=True)
    _K32.OpenProcess.argtypes = (ctypes.c_uint32, ctypes.c_int, ctypes.c_uint32)
    _K32.OpenProcess.restype = ctypes.c_void_p
    _K32.WaitForSingleObject.argtypes = (ctypes.c_void_p, ctypes.c_uint32)
    _K32.CloseHandle.argtypes = (ctypes.c_void_p,)


@pytest.fixture(autouse=True)
def temp_dirs(monkeypatch):
    """Every run makes exactly one temp dir, and it is gone when run_child returns or raises."""
    made = []
    real = tempfile.mkdtemp

    def mkdtemp(*args, **kwargs):
        made.append(real(*args, **kwargs))
        return made[-1]

    monkeypatch.setattr(_proc.tempfile, "mkdtemp", mkdtemp)
    yield made
    assert made and [path for path in made if os.path.exists(path)] == []


def argv(mode):
    return [sys.executable, str(CHILD), mode]


def run(mode, limits=FAST, seen=None):
    seen = [] if seen is None else seen

    async def on_event(event):
        seen.append(event)

    return asyncio.run(_proc.run_child(argv(mode), {"job": 1}, limits, on_event))


def alive(pid):
    if os.name == "nt":  # never os.kill(pid, 0) here: on Windows it terminates the process
        handle = _K32.OpenProcess(0x00100000, False, pid)  # SYNCHRONIZE
        if not handle:
            return False
        try:
            return _K32.WaitForSingleObject(handle, 0) != 0
        finally:
            _K32.CloseHandle(handle)
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    try:  # a zombie has exited
        with open("/proc/%d/stat" % pid) as stat:
            return stat.read().rsplit(")", 1)[1].split()[0] != "Z"
    except OSError:
        return True


def gone(pid, seconds=10.0):
    end = time.monotonic() + seconds
    while alive(pid):
        if time.monotonic() >= end:
            return False
        time.sleep(0.05)
    return True


def test_result_progress_and_env(monkeypatch):
    monkeypatch.setenv("CAP_TEST_SECRET", "never-in-the-child")
    for name in _proc.FORCED_ENV:
        monkeypatch.setenv(name, "operator-value")
    seen = []
    result = run("echo", seen=seen)
    assert seen == [{"type": "progress", "marker": "one"}]
    assert result["type"] == "result" and result["job"] == {"job": 1} and result["cwd_is_tmp"] is True
    env = result["env"]
    assert {name: env.get(name) for name in _proc.FORCED_ENV} == _proc.FORCED_ENV
    assert "CAP_TEST_SECRET" not in env
    assert env["TMP"] == env["TEMP"] == env["TMPDIR"] == result["tmp"]
    if os.name != "nt":
        assert env["XDG_DATA_HOME"] == result["tmp"]


def test_timeout_kills_the_tree():
    seen = []

    async def main():
        async def on_event(event):
            seen.append(event)

        await asyncio.wait_for(_proc.run_child(argv("hang"), {}, FAST, on_event), timeout=5)

    with pytest.raises(asyncio.TimeoutError):
        asyncio.run(main())
    assert seen and gone(seen[0]["pid"])


def test_cancel_kills_the_tree():
    seen = []

    async def main():
        started = asyncio.Event()

        async def on_event(event):
            seen.append(event)
            started.set()

        task = asyncio.ensure_future(_proc.run_child(argv("hang"), {}, FAST, on_event))
        await asyncio.wait_for(started.wait(), 30)
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await task

    asyncio.run(main())
    assert gone(seen[0]["pid"])


def test_on_event_error_propagates_and_kills_the_tree():
    seen = []

    async def main():
        async def on_event(event):
            seen.append(event)
            raise ValueError("handler failed")

        await _proc.run_child(argv("hang"), {}, FAST, on_event)

    with pytest.raises(ValueError, match="handler failed"):
        asyncio.run(main())
    assert gone(seen[0]["pid"])


@pytest.mark.parametrize("mode, limits", [
    ("crash", FAST),
    ("long-line", _proc.ChildLimits(grace_ms=300, max_stdout_bytes=8 << 20)),
    ("flood", _proc.ChildLimits(grace_ms=300, max_stdout_bytes=4096)),
    ("unknown-type", FAST),
    ("not-json", FAST),
    ("exit-after-result", FAST),
])
def test_child_crashed(mode, limits):
    with pytest.raises(JobFailure) as failure:
        run(mode, limits)
    assert failure.value.code == failure.value.error_code == _proc.CHILD_CRASHED


def test_stderr_flood_is_drained():
    assert run("stderr") == {"type": "result"}


def test_containment_failure_fails_closed(monkeypatch):
    pids = []

    def refuse(proc):
        pids.append(proc.pid)
        raise OSError("containment refused")

    monkeypatch.setattr(_proc, "_Tree", refuse)
    with pytest.raises(JobFailure) as failure:
        run("hang")
    assert failure.value.code == _proc.CHILD_CRASHED and gone(pids[0])


def test_trap_reaches_the_child(monkeypatch):
    monkeypatch.setenv("PYTHONPATH", str(TRAP))
    control = run("echo")
    assert control["trap"] is True and control["env"]["PYTHONPATH"] == str(TRAP)
    with pytest.raises(JobFailure):
        run("connect")
