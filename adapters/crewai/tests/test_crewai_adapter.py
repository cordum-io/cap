"""CrewAI adapter boundaries, goldens and installed-child security checks."""
import asyncio
from types import SimpleNamespace

import cap
import pytest

import cap_adapter_crewai as adapter
from cap_adapter_crewai import _proc


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
