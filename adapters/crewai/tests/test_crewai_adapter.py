"""CrewAI adapter boundaries, goldens and installed-child security checks."""
import asyncio
import os
import sys
import tempfile
from pathlib import Path
from types import SimpleNamespace

import cap
import pytest
from cap.pb.cordum.agent.v1 import job_pb2

import cap_adapter_crewai as adapter
from cap_adapter_crewai import _proc

FIXTURES = Path(__file__).parent / "fixtures"
TRAP = Path(__file__).resolve().parents[3] / "testdata" / "adapters" / "nettrap"


@pytest.fixture
def child_env(monkeypatch, tmp_path):
    monkeypatch.setenv("PYTHONPATH", os.pathsep.join([str(FIXTURES), str(TRAP)]))
    monkeypatch.setattr(tempfile, "tempdir", str(tmp_path))
    return tmp_path


def child_run(factory, data, artifacts=None):
    assert Path(adapter.__file__).with_name("_child.py").exists(), "child not implemented"
    events = []

    async def emit(event):
        events.append(event)

    job = {"factory": "golden_crews:" + factory, "inputModel": "golden_crews:TextInput",
           "artifactTasks": artifacts or {}, "input": data}
    result = asyncio.run(_proc.run_child([sys.executable, "-m", "cap_adapter_crewai._child"],
                                         job, _proc.ChildLimits(), emit))
    return result, events


@pytest.mark.parametrize("factory,data,expected,kickoffs", [
    ("echo_crew", {"text": "hello"}, {"type": "result", "output": {"text": "hello"}}, 1),
    ("echo_crew", {"text": 42}, {"type": "error", "code": job_pb2.ERROR_CODE_JOB_INVALID_INPUT, "retryable": None}, 0),
    ("traced_crew", {"text": "hello"}, {"type": "error", "code": "FRAMEWORK_ERROR", "retryable": None}, 0),
    ("hierarchical_crew", {"text": "hello"}, {"type": "error", "code": "FRAMEWORK_ERROR", "retryable": None}, 0),
    ("failure_crew", {"text": "hello"}, {"type": "error", "code": 1001, "retryable": True}, 0),
])
def test_child_outcomes(child_env, factory, data, expected, kickoffs):
    assert child_run(factory, data) == (expected, [])
    marker = child_env / "kickoffs.txt"
    assert (len(marker.read_text().splitlines()) if marker.exists() else 0) == kickoffs


def test_stdout_and_existing_callback_stay_off_ipc(child_env):
    result, events = child_run("noisy_crew", {"text": "hello"}, {"named": "artifact"})
    assert result == {"type": "result", "output": {"text": "hello"}}
    assert events == [{"type": "progress", "marker": "named"},
                      {"type": "artifact", "name": "artifact", "mediaType": "text/plain",
                       "content": '{"text": "hello"}'}]
    assert (child_env / "callbacks.txt").read_text().splitlines() == ["task", "crew", "crew"]


def entry(**extra):
    return {"factory": "operator_module:factory", "inputModel": "operator_module:Input", **extra}


def test_workloads_are_validated_without_importing():
    assert hasattr(adapter, "load_workloads"), "operator registry not implemented"
    workloads = adapter.load_workloads({"echo": entry()})
    assert workloads["echo"].factory == "operator_module:factory"
    assert workloads["echo"].child == _proc.ChildLimits()
    with pytest.raises(TypeError):
        workloads["other"] = workloads["echo"]


@pytest.mark.parametrize("value,key", [
    ({}, "workloads"), ({"BAD": entry()}, "workloads"),
    ({"echo": entry(other=1)}, "workloads.echo"),
    ({"echo": entry(factory="os..path:bad")}, "factory"),
    ({"echo": entry(inputModel="/tmp/x.py")}, "inputModel"),
    ({"echo": entry(artifactTasks={"x": "BAD"})}, "artifactTasks"),
    ({"echo": entry(artifactTasks={"x": "a", "y": "a"})}, "artifactTasks"),
    ({"echo": entry(child={"graceMs": True})}, "child"),
    ({"echo": entry(child={"graceMs": -1, "maxStdoutBytes": 1, "maxStderrBytes": 1})}, "child"),
    ({"echo": entry(child={"graceMs": 1, "maxStdoutBytes": 1 << 30, "maxStderrBytes": 1})}, "child"),
])
def test_bad_workload(value, key):
    assert hasattr(adapter, "load_workloads"), "operator registry not implemented"
    with pytest.raises(adapter.ConfigError, match=key):
        adapter.load_workloads(value)


@pytest.mark.parametrize("event,code", [
    ({"type": "progress", "marker": 3}, "CHILD_CRASHED"),
    ({"type": "result"}, "CHILD_CRASHED"),
    ({"type": "error", "code": 0, "retryable": None}, "CHILD_CRASHED"),
    ({"type": "error", "code": "BUSY", "retryable": 1}, "CHILD_CRASHED"),
    ({"type": "artifact", "name": "x", "mediaType": "text/plain", "content": None}, "CHILD_CRASHED"),
    ({"type": "progress", "marker": ""}, "FRAMEWORK_ERROR"),
])
def test_event_failure_mapping(monkeypatch, event, code):
    assert hasattr(adapter, "make_handler"), "handler not implemented"

    async def report(marker):
        if not marker:
            raise ValueError("secret")

    async def child(argv, job, limits, emit):
        if event["type"] in ("progress", "artifact"):
            await emit(event)
        return event

    monkeypatch.setattr(_proc, "run_child", child)
    ctx = SimpleNamespace(job=SimpleNamespace(meta=SimpleNamespace(capability="echo")), report_progress=report)
    with pytest.raises(cap.JobFailure) as caught:
        asyncio.run(adapter.make_handler(adapter.load_workloads({"echo": entry()}))(ctx, {}))
    assert caught.value.code == code


def test_sdk_failure_latch_propagates(monkeypatch):
    assert hasattr(adapter, "make_handler"), "handler not implemented"
    failure = cap.JobFailure(1001, retryable=True)

    async def report(marker):
        raise failure

    async def child(argv, job, limits, emit):
        assert job == {"factory": "operator_module:factory", "inputModel": "operator_module:Input",
                       "artifactTasks": {}, "input": {"text": "hello"}}
        await emit({"type": "progress", "marker": "step"})

    monkeypatch.setattr(_proc, "run_child", child)
    ctx = SimpleNamespace(job=SimpleNamespace(meta=SimpleNamespace(capability="echo")), report_progress=report)
    with pytest.raises(cap.JobFailure) as caught:
        asyncio.run(adapter.make_handler(adapter.load_workloads({"echo": entry()}))(ctx, {"text": "hello"}))
    assert caught.value is failure
