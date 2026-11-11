"""Typed job outcomes: cap.JobFailure and the runtime's fixed spec/13 failure table."""
from __future__ import annotations

import asyncio
import copy
import enum
import pickle
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, List, Optional, Tuple

import pytest
from pydantic import BaseModel

import cap
from cap.outcome import JobFailure, _failure_outcome
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.runtime import Agent, InMemoryBlobStore
from cap.subjects import SUBJECT_RESULT
from cap.testing import MockNATS

CANARY = "cap-canary-7d13"
TOPIC = "job.outcome"
JOB_ID = "job-outcome-1"
CTX_KEY = "ctx:" + JOB_ID
POINTER = "redis://" + CTX_KEY
INPUT = ('{"prompt": "%s"}' % CANARY).encode("utf-8")

FAILED = job_pb2.JOB_STATUS_FAILED
RETRYABLE = job_pb2.JOB_STATUS_FAILED_RETRYABLE
FATAL = job_pb2.JOB_STATUS_FAILED_FATAL
E = job_pb2  # ErrorCode values are module-level ints in the generated code.


class TestJobFailureConstruction:
    @pytest.mark.parametrize(
        "code, retryable, status, enum_value, error_code",
        [
            ("FRAMEWORK_ERROR", None, FAILED, 0, "FRAMEWORK_ERROR"),
            ("TIMEOUT", None, FAILED, 0, "TIMEOUT"),
            (E.ERROR_CODE_JOB_INVALID_INPUT, True, RETRYABLE, 203, ""),
            (E.ERROR_CODE_JOB_TIMEOUT, False, FATAL, 200, "TIMEOUT"),
            (E.ERROR_CODE_SAFETY_DENIED, None, FAILED, 300, "DENIED"),
            (E.ERROR_CODE_JOB_PERMISSION_DENIED, None, FAILED, 202, "PERMISSION_DENIED"),
            (E.ERROR_CODE_JOB_NOT_FOUND, None, FAILED, 204, ""),
            (4242, False, FATAL, 4242, ""),
            (1000, None, FAILED, 1000, ""),
            (9999, True, RETRYABLE, 9999, ""),
            ("A" * 64, None, FAILED, 0, "A" * 64),
        ],
    )
    def test_valid(self, code, retryable, status, enum_value, error_code):
        failure = JobFailure(code, retryable=retryable)
        assert (failure.code, failure.retryable) == (code, retryable)
        assert failure.status == status
        assert failure.error_code_enum == enum_value
        assert failure.error_code == error_code
        assert isinstance(failure, Exception)
        assert str(failure) == "job failure: %s" % code

    @pytest.mark.parametrize(
        "code",
        [True, False, 0, 50, 999, 10000, -1, 500, "framework_error", "BAD CODE", "",
         "A" * 65, "FRAMEWORK_ERROR\n", "1ERROR", "_ERROR", "ÉRROR", CANARY, 1.5, None, b"ERROR"],
    )
    def test_invalid_code(self, code):
        with pytest.raises((TypeError, ValueError)) as info:
            JobFailure(code)
        assert CANARY not in str(info.value)

    @pytest.mark.parametrize("retryable", ["yes", 1, 0, CANARY])
    def test_invalid_retryable(self, retryable):
        with pytest.raises(TypeError) as info:
            JobFailure("FRAMEWORK_ERROR", retryable=retryable)
        assert CANARY not in str(info.value)

    def test_properties_are_read_only(self):
        failure = JobFailure("FRAMEWORK_ERROR")
        for name in ("code", "retryable", "status", "error_code_enum", "error_code"):
            with pytest.raises(AttributeError):
                setattr(failure, name, "OTHER")

    def test_subclassed_codes_are_normalized(self):
        class AppCode(enum.IntEnum):
            CAP_CANARY_7D13 = 4242

        class Sneaky(str):
            def __str__(self) -> str:
                return CANARY

        by_int = JobFailure(AppCode.CAP_CANARY_7D13)
        by_str = JobFailure(Sneaky("FRAMEWORK_ERROR"))
        assert type(by_int.code) is int and by_int.code == 4242
        assert type(by_str.code) is str and by_str.error_code == "FRAMEWORK_ERROR"
        for failure in (by_int, by_str):
            text = str(failure) + repr(failure) + repr(failure.args)
            assert "CAP_CANARY_7D13" not in text and CANARY not in text

    @pytest.mark.parametrize("retryable", [None, True, False])
    def test_pickle_and_copy_keep_the_outcome(self, retryable):
        failure = SubclassedFailure(E.ERROR_CODE_JOB_TIMEOUT, retryable=retryable)
        for clone in (pickle.loads(pickle.dumps(failure)), copy.copy(failure)):
            assert type(clone) is SubclassedFailure
            assert (clone.code, clone.retryable, clone.status) == (200, retryable, failure.status)
            assert _failure_outcome(clone) == _failure_outcome(failure)

    def test_exported(self):
        assert cap.JobFailure is JobFailure
        assert "JobFailure" in cap.__all__


class RecordingMetrics:
    def __init__(self) -> None:
        self.calls: List[Tuple[Any, ...]] = []

    def on_job_received(self, job_id: str, topic: str) -> None:
        self.calls.append(("received", job_id, topic))

    def on_job_completed(self, job_id: str, duration_ms: int, status: str) -> None:
        self.calls.append(("completed", job_id, status))

    def on_job_failed(self, job_id: str, error_msg: str) -> None:
        self.calls.append(("failed", job_id, error_msg))

    def on_heartbeat_sent(self, worker_id: str) -> None:
        self.calls.append(("heartbeat", worker_id))


class FaultStore(InMemoryBlobStore):
    """Blob store whose context reads and result writes can fail or stall."""

    def __init__(self, get_error: Optional[Exception] = None, set_error: Optional[Exception] = None,
                 stall_get: bool = False, stall_set: bool = False) -> None:
        super().__init__()
        self.get_error, self.set_error = get_error, set_error
        self.stall_get, self.stall_set = stall_get, stall_set

    async def get(self, key: str) -> Optional[bytes]:
        if self.get_error is not None:
            raise self.get_error
        if self.stall_get:
            await asyncio.sleep(5)
        return await super().get(key)

    async def set(self, key: str, data: bytes) -> None:
        if self.set_error is not None:
            raise self.set_error
        if self.stall_set:
            await asyncio.sleep(5)
        await super().set(key, data)


class StrictInput(BaseModel):
    count: int


class StrictOutput(BaseModel):
    count: int


def returns(value: Any) -> Callable[[], Any]:
    return lambda: value


def raises(make: Callable[[], BaseException]) -> Callable[[], Any]:
    def act() -> Any:
        raise make()
    return act


def chained_failure() -> BaseException:
    try:
        raise RuntimeError(CANARY)
    except RuntimeError as exc:
        try:
            raise JobFailure("FRAMEWORK_ERROR") from exc
        except JobFailure as failure:
            return failure


class SubclassedFailure(JobFailure):
    pass


class OverridingFailure(JobFailure):
    # Class attributes shadow the base properties; the published outcome must not use them.
    status = job_pb2.JOB_STATUS_SUCCEEDED
    error_code_enum = 7
    error_code = "free text " + CANARY


class UninitializedFailure(JobFailure):
    def __init__(self) -> None:  # skips JobFailure.__init__, so there is no validated outcome
        pass


@dataclass(frozen=True)
class Case:
    id: str
    expect: Tuple[int, int, str, str]
    calls: int = 0
    retries: int = 2
    blob: Optional[bytes] = INPUT
    pointer: str = POINTER
    act: Callable[[], Any] = returns({"ok": True})
    input_model: Optional[type] = None
    output_model: Optional[type] = None
    agent: Dict[str, Any] = field(default_factory=dict)
    store: Dict[str, Any] = field(default_factory=dict)
    in_middleware: bool = False


INVALID, NOT_FOUND = E.ERROR_CODE_JOB_INVALID_INPUT, E.ERROR_CODE_JOB_NOT_FOUND
EXHAUSTED, UNSPECIFIED = E.ERROR_CODE_JOB_RESOURCE_EXHAUSTED, E.ERROR_CODE_UNSPECIFIED
REPORTED = "handler reported failure"
TINY = {"io_timeout": 0.05}

CASES = [
    Case("a-pointer-scheme", (FAILED, INVALID, "", "context pointer invalid"), pointer="s3://" + CANARY),
    Case("a-pointer-empty", (FAILED, INVALID, "", "context pointer invalid"), pointer=""),
    Case("a-pointer-no-key", (FAILED, INVALID, "", "context pointer invalid"), pointer="redis://"),
    Case("b-fetch-error", (FAILED, UNSPECIFIED, "", "context fetch failed"),
         store={"get_error": RuntimeError(CANARY)}),
    Case("b-fetch-timeout", (FAILED, UNSPECIFIED, "", "context fetch failed"),
         store={"stall_get": True}, agent=TINY),
    Case("c-missing", (FAILED, NOT_FOUND, "", "context not found"), blob=None,
         pointer="redis://ctx:" + CANARY),
    Case("d-context-oversize", (FAILED, EXHAUSTED, "", "context exceeds max size"),
         agent={"max_context_bytes": 8}),
    Case("e-decode-utf8", (FAILED, INVALID, "", "context decode failed"),
         blob=b"\xff" + CANARY.encode("utf-8")),
    Case("e-decode-json", (FAILED, INVALID, "", "context decode failed"),
         blob=(CANARY + "{").encode("utf-8")),
    Case("f-input-invalid", (FAILED, INVALID, "", "input validation failed"),
         blob=('{"count": "%s"}' % CANARY).encode("utf-8"), input_model=StrictInput),
    Case("g-handler-raises", (FAILED, UNSPECIFIED, "", "handler failed"), calls=3,
         act=raises(lambda: RuntimeError(CANARY))),
    Case("h-output-invalid", (FAILED, UNSPECIFIED, "", "handler failed"), calls=3,
         act=returns({"count": CANARY}), output_model=StrictOutput),
    Case("i-encode-set", (FAILED, UNSPECIFIED, "", "result encode failed"), calls=1,
         act=returns({CANARY})),
    Case("i-encode-nested", (FAILED, UNSPECIFIED, "", "result encode failed"), calls=1,
         act=returns({"value": {CANARY}})),
    Case("j-result-oversize", (FAILED, EXHAUSTED, "", "result exceeds max size"), calls=1,
         act=returns({"value": CANARY}), agent={"max_result_bytes": 8}),
    Case("k-write-error", (FAILED, UNSPECIFIED, "", "result write failed"), calls=1,
         store={"set_error": RuntimeError(CANARY)}),
    Case("k-write-timeout", (FAILED, UNSPECIFIED, "", "result write failed"), calls=1,
         store={"stall_set": True}, agent=TINY),
    Case("l-failure-string", (FAILED, UNSPECIFIED, "FRAMEWORK_ERROR", REPORTED), calls=1,
         act=raises(lambda: JobFailure("FRAMEWORK_ERROR"))),
    Case("l-failure-from-cause", (FAILED, UNSPECIFIED, "FRAMEWORK_ERROR", REPORTED), calls=1,
         act=raises(chained_failure)),
    Case("m-failure-retryable", (RETRYABLE, INVALID, "", REPORTED), calls=1, retries=3,
         act=raises(lambda: JobFailure(203, retryable=True))),
    Case("n-failure-fatal-timeout", (FATAL, E.ERROR_CODE_JOB_TIMEOUT, "TIMEOUT", REPORTED), calls=1,
         act=raises(lambda: JobFailure(200, retryable=False))),
    Case("o-failure-application-code", (FATAL, 4242, "", REPORTED), calls=1,
         act=raises(lambda: JobFailure(4242, retryable=False))),
    Case("o-failure-subclass", (FATAL, UNSPECIFIED, "APP_ERROR", REPORTED), calls=1,
         act=raises(lambda: SubclassedFailure("APP_ERROR", retryable=False))),
    Case("o-failure-overridden-fields", (FATAL, UNSPECIFIED, "APP_ERROR", REPORTED), calls=1,
         act=raises(lambda: OverridingFailure("APP_ERROR", retryable=False))),
    Case("o-failure-without-init", (FAILED, UNSPECIFIED, "", "handler failed"), calls=1,
         act=raises(UninitializedFailure)),
    Case("p-failure-in-middleware", (FAILED, UNSPECIFIED, "FRAMEWORK_ERROR", REPORTED), calls=1,
         act=raises(lambda: JobFailure("FRAMEWORK_ERROR")), in_middleware=True),
    Case("q-success", (job_pb2.JOB_STATUS_SUCCEEDED, UNSPECIFIED, "", ""), calls=1),
]


def job_message(job_id: str, pointer: str) -> Any:
    packet = buspacket_pb2.BusPacket(trace_id="trace-outcome", sender_id="client", protocol_version=1)
    packet.created_at.GetCurrentTime()
    packet.job_request.CopyFrom(job_pb2.JobRequest(job_id=job_id, topic=TOPIC, context_ptr=pointer))
    return type("Msg", (object,), {"data": packet.SerializeToString(deterministic=True)})()


def make_agent(case: Case, metrics: RecordingMetrics, calls: List[str]) -> Tuple[Agent, MockNATS, FaultStore]:
    nats, store = MockNATS(), FaultStore(**case.store)
    agent = Agent(store=store, connect_fn=nats.connect, sender_id="outcome-worker", retries=case.retries,
                  heartbeat_interval=60, metrics=metrics, **case.agent)

    async def middleware(ctx: Any, data: Any, next_fn: Any) -> Any:
        calls.append("middleware")
        case.act()
        return await next_fn(ctx, data)

    if case.in_middleware:
        agent.use(middleware)

    @agent.job(TOPIC, input_model=case.input_model, output_model=case.output_model)
    async def handler(ctx: Any, data: Any) -> Any:
        calls.append("handler")
        return case.act()

    return agent, nats, store


async def drive(case: Case) -> Tuple[job_pb2.JobResult, List[bytes], RecordingMetrics, List[str]]:
    metrics, calls = RecordingMetrics(), []
    agent, nats, store = make_agent(case, metrics, calls)
    await agent.start()
    if case.blob is not None:
        store._data[CTX_KEY] = case.blob
    await nats.subscriptions[TOPIC](job_message(JOB_ID, case.pointer))
    raw: List[bytes] = []
    while True:
        subject, payload = await asyncio.wait_for(nats.published.get(), timeout=5.0)
        raw.append(payload)
        if subject == SUBJECT_RESULT:
            break
    await agent.close()
    while not nats.published.empty():
        raw.append(nats.published.get_nowait()[1])
    return buspacket_pb2.BusPacket.FromString(raw[-1]).job_result, raw, metrics, calls


class TestRuntimeFailureTable:
    @pytest.mark.parametrize("case", CASES, ids=[case.id for case in CASES])
    def test_case(self, case: Case):
        result, raw, metrics, calls = asyncio.run(drive(case))
        status, enum_value, error_code, message = case.expect
        assert (result.status, result.error_code_enum, result.error_code, result.error_message) == (
            status, enum_value, error_code, message)
        assert result.job_id == JOB_ID and result.worker_id == "outcome-worker"
        assert len(calls) == case.calls
        assert all(CANARY.encode("utf-8") not in payload for payload in raw)
        assert CANARY not in repr(metrics.calls)
        failed = [call for call in metrics.calls if call[0] == "failed"]
        if status == job_pb2.JOB_STATUS_SUCCEEDED:
            assert failed == [] and result.result_ptr == "redis://res:" + JOB_ID
        else:
            assert failed == [("failed", JOB_ID, message)] and result.result_ptr == ""

    def test_concurrent_jobs_publish_their_own_outcomes(self):
        async def scenario() -> Dict[str, job_pb2.JobResult]:
            nats, store = MockNATS(), InMemoryBlobStore()
            agent = Agent(store=store, connect_fn=nats.connect, sender_id="outcome-worker",
                          max_parallel=2, heartbeat_interval=60)
            gate = asyncio.Event()

            @agent.job(TOPIC)
            async def handler(ctx: Any, data: Any) -> Any:
                await gate.wait()
                raise JobFailure(data["code"], retryable=data["retryable"])

            await agent.start()
            store._data["ctx:job-a"] = b'{"code": "FRAMEWORK_ERROR", "retryable": null}'
            store._data["ctx:job-b"] = b'{"code": 203, "retryable": true}'
            for job_id in ("job-a", "job-b"):
                await nats.subscriptions[TOPIC](job_message(job_id, "redis://ctx:" + job_id))
            gate.set()
            results: Dict[str, job_pb2.JobResult] = {}
            while len(results) < 2:
                subject, payload = await asyncio.wait_for(nats.published.get(), timeout=5.0)
                if subject == SUBJECT_RESULT:
                    result = buspacket_pb2.BusPacket.FromString(payload).job_result
                    results[result.job_id] = result
            await agent.close()
            return results

        results = asyncio.run(scenario())
        assert (results["job-a"].status, results["job-a"].error_code) == (FAILED, "FRAMEWORK_ERROR")
        assert (results["job-b"].status, results["job-b"].error_code_enum) == (RETRYABLE, 203)
