"""Fenced cancellation and handler deadlines through real Agent subscriptions."""
import asyncio
import logging
import time
from collections import Counter
from dataclasses import dataclass

import pytest
from cryptography.hazmat.primitives.asymmetric import ec

from cap.packet_boundary import finalize_packet, parse_packet
from cap.pb.cordum.agent.v1 import job_pb2
from cap.production_events import seal_production_event
from cap.runtime import Agent, Context, InMemoryBlobStore
from cap.subjects import SUBJECT_CANCEL, SUBJECT_RESULT
from production_admission_support import (
    RecordingReplayStore, activate_production_session, make_packet, production_agent,
)
from runtime_support import Message, RecordingNATS

TOPIC = "job.cancel.test"
CANARY = "private-cancel-reason-requester"
SUCCEEDED, CANCELLED, FAILED = (job_pb2.JOB_STATUS_SUCCEEDED,
                              job_pb2.JOB_STATUS_CANCELLED, job_pb2.JOB_STATUS_FAILED)


@dataclass
class RoutedMessage(Message):
    subject: str = TOPIC


class Harness:
    def __init__(self, *, production=False, store=None, thread=False, **options):
        self.key = ec.generate_private_key(ec.SECP256R1())
        self.nats, self.store = RecordingNATS([]), store or InMemoryBlobStore()
        self.started, self.release = asyncio.Event(), asyncio.Event()
        self.calls = self.stops = self.completions = 0
        self.ctx, self.start_time = None, 0.0
        self.production, self.thread = production, thread
        self.replay = RecordingReplayStore()
        self.agent = (production_agent({"k1": self.key.public_key()}, self.replay)
                      if production else Agent(store=self.store, connect_fn=self.nats.connect,
                      public_keys={"scheduler-1": self.key.public_key()}, **options))
        self.agent.job(TOPIC)(self.handler)

    async def handler(self, ctx: Context, _data: object) -> dict:
        self.ctx, self.start_time = ctx, time.monotonic()
        self.calls += 1
        self.started.set()
        try:
            if self.thread:
                await asyncio.to_thread(time.sleep, 1.0)
            else:
                await self.release.wait()
        except asyncio.CancelledError:
            self.stops += 1
            raise
        self.completions += 1
        return {"ok": True}

    async def __aenter__(self):
        if self.production:
            # Inject the authenticated session fixture, not a fake decoder.
            self.agent._store, self.agent._connect_fn = self.store, self.nats.connect
            self.agent._nc = await self.agent._connect_fn(servers="unused", name="test")
            activate_production_session(self.agent)
            config = self.agent._trust_settings.config
            config.proof_key_id, config.proof_private_key = "k1", self.key
            await self.agent._subscribe_handlers()
        else:
            await self.agent.start()
        return self

    async def __aexit__(self, *_exc):
        self.release.set()
        await asyncio.wait_for(asyncio.gather(*self.agent._handler_tasks,
                                             return_exceptions=True), 2)
        if self.production:
            self.agent._worker_trust = None
        await self.agent.close()

    def request(self, job_id="job-1", deadline_ms=0):
        packet = make_packet(audience=TOPIC)
        req = packet.job_request
        req.job_id, req.topic, req.context_ptr = job_id, TOPIC, "redis://ctx:" + job_id
        req.budget.deadline_ms = deadline_ms
        req.dispatch.CopyFrom(job_pb2.DispatchIdentity(
            dispatch_id="dispatch-1", attempt=1, assigned_worker_id=self.agent._sender_id))
        return packet

    def wire(self, packet, audience=TOPIC, key=None):
        key = key or self.key
        raw = (seal_production_event(packet, key=key, key_id="k1", audience=audience)
               if self.production else finalize_packet(packet, key).SerializeToString())
        return RoutedMessage(raw, audience)

    async def submit(self, packet, wait=True):
        await self.store.set("ctx:" + packet.job_request.job_id, b"{}")
        await self.nats.deliver(TOPIC, self.wire(packet))
        if wait:
            await asyncio.wait_for(self.started.wait(), 1)

    def cancel(self, request):
        packet = make_packet()
        packet.ClearField("job_request")
        packet.job_cancel.CopyFrom(job_pb2.JobCancel(
            job_id=request.job_request.job_id, dispatch=request.job_request.dispatch,
            reason=CANARY, requested_by=CANARY))
        return packet

    async def send_cancel(self, packet, **options):
        await asyncio.wait_for(self.nats.deliver(
            SUBJECT_CANCEL, self.wire(packet, **options)), 0.8)

    async def result(self):
        await self.nats.wait_for_result(1.2)
        return [parse_packet(raw).job_result for subject, raw in self.nats.published
                if subject == SUBJECT_RESULT][-1]


@pytest.mark.parametrize("max_parallel", [1, 2])
def test_matching_cancel_is_bounded_without_a_handler_slot(max_parallel, caplog):
    async def run():
        async with Harness(max_parallel=max_parallel) as h:
            packet = h.request()
            await h.submit(packet)
            start = time.monotonic()
            await h.send_cancel(h.cancel(packet))
            result = await h.result()
            assert time.monotonic() - start < 1.0
            assert (result.status, result.error_code_enum, result.error_code,
                    result.error_message) == (CANCELLED, 0, "", "job cancelled")
            await asyncio.gather(*h.agent._handler_tasks)
            assert (h.calls, h.stops, h.completions, h.nats.result_count()) == (1, 1, 0, 1)
            assert h.ctx.cancelled is True
            assert all(CANARY.encode() not in raw for _, raw in h.nats.published)
            assert CANARY not in caplog.text
    asyncio.run(run())


@pytest.mark.parametrize("mismatch", ["job_id", "dispatch_id", "attempt",
                                    "assigned_worker_id", "empty", "untrusted", "not-running"])
def test_refused_cancel_leaves_handler_and_publish_unchanged(mismatch):
    async def run():
        async with Harness() as h:
            packet = h.request()
            await h.submit(packet)
            cancel = h.cancel(packet)
            if mismatch in ("job_id", "not-running"):
                cancel.job_cancel.job_id = "other-job"
            elif mismatch == "empty":
                cancel.job_cancel.ClearField("dispatch")
            elif mismatch != "untrusted":
                setattr(cancel.job_cancel.dispatch, mismatch, 2 if mismatch == "attempt" else "other")
            key = ec.generate_private_key(ec.SECP256R1()) if mismatch == "untrusted" else h.key
            await h.send_cancel(cancel, key=key)
            await asyncio.sleep(0.02)
            assert (h.calls, h.stops, h.nats.result_count()) == (1, 0, 0)
            h.release.set()
            assert (await h.result()).status == SUCCEEDED
            assert (h.completions, h.nats.result_count(), h.ctx.cancelled) == (1, 1, False)
    asyncio.run(run())


@pytest.mark.parametrize("case", ["matching", "audience", "identity", "not-running"])
def test_production_cancel_admission(case):
    async def run():
        async with Harness(production=True) as h:
            packet = h.request()
            await h.submit(packet)
            cancel = h.cancel(packet)
            if case == "identity":
                cancel.job_cancel.identity.tenant_id = "other-tenant"
            if case == "not-running":
                cancel.job_cancel.job_id = "other-job"
            before = len(h.replay.digests)
            await h.send_cancel(cancel, audience=TOPIC if case == "audience" else SUBJECT_CANCEL)
            if case != "matching":
                await asyncio.sleep(0.02)
                assert (h.stops, h.nats.result_count()) == (0, 0)
                if case in ("audience", "not-running"):
                    assert len(h.replay.digests) == before
                h.release.set()
            assert (await h.result()).status == (CANCELLED if case == "matching" else SUCCEEDED)
            await asyncio.gather(*h.agent._handler_tasks)
            assert (h.calls, h.nats.result_count()) == (1, 1)
            assert h.completions == (0 if case == "matching" else 1)
    asyncio.run(run())


class GatedStore(InMemoryBlobStore):
    def __init__(self):
        super().__init__()
        self.entered, self.release = asyncio.Event(), asyncio.Event()

    async def get(self, key):
        self.entered.set()
        await self.release.wait()
        return await super().get(key)


def test_cancel_during_input_resolution_never_starts_handler():
    async def run():
        store = GatedStore()
        async with Harness(store=store) as h:
            packet = h.request()
            await h.submit(packet, wait=False)
            await asyncio.wait_for(store.entered.wait(), 1)
            try:
                await h.send_cancel(h.cancel(packet))
            finally:
                store.release.set()
            result = await h.result()
            assert (result.status, result.execution_ms, h.calls) == (CANCELLED, 0, 0)
            assert h.nats.result_count() == 1
    asyncio.run(run())


@pytest.mark.parametrize("deadline_ms, maximum, delay",
                         [(200, None, 0), (0, 0.2, 0), (5000, 0.2, 0), (200, None, 0.05)])
def test_deadline_and_agent_maximum_are_bounded(deadline_ms, maximum, delay):
    async def slow_middleware(ctx, data, next_):
        await asyncio.sleep(delay)  # Middleware time counts toward the deadline.
        return await next_(ctx, data)

    async def run():
        async with Harness(max_deadline=maximum) as h:
            if delay:
                h.agent.use(slow_middleware)
            await h.submit(h.request(deadline_ms=deadline_ms))
            result = await h.result()
            # Anchor on ctx.deadline: the handler body starts after the runtime's start.
            late = time.monotonic() - h.ctx.deadline
            assert 0 <= late <= 1.0
            assert (result.status, result.error_code_enum, result.error_code,
                    result.error_message) == (FAILED, job_pb2.ERROR_CODE_JOB_TIMEOUT,
                                             "TIMEOUT", "job deadline exceeded")
            assert abs(h.ctx.deadline - h.start_time - (0.2 - delay)) < 0.02
            await asyncio.gather(*h.agent._handler_tasks)
            assert (h.ctx.cancelled, h.stops, h.nats.result_count()) == (True, 1, 1)
            print("deadline", deadline_ms, maximum, delay, "late", late)
    asyncio.run(run())


def test_zero_deadline_without_maximum_is_unbounded():
    async def run():
        async with Harness(max_deadline=None) as h:
            await h.submit(h.request())
            await asyncio.sleep(0.3)
            assert h.nats.result_count() == 0 and h.ctx.deadline is None
            h.release.set()
            assert (await h.result()).status == SUCCEEDED
            assert h.ctx.cancelled is False
    asyncio.run(run())


def test_thread_keeps_running_without_a_second_terminal_result():
    async def run():
        async with Harness(thread=True) as h:
            await h.submit(h.request(deadline_ms=200))
            result = await h.result()
            assert 0 <= time.monotonic() - h.ctx.deadline <= 1.0
            assert (result.status, result.error_code) == (FAILED, "TIMEOUT")
            assert h.ctx.cancelled is True
            await asyncio.sleep(1.2)
            assert (h.stops, h.completions, h.nats.result_count()) == (1, 0, 1)
    asyncio.run(run())


async def _keep_loop_awake(stop, on_tick=None):
    # Iterate the loop without pause, so every pass checks the timers, the way
    # other I/O wakes a busy worker's loop.
    while not stop.is_set():
        if on_tick is not None:
            on_tick()
        await asyncio.sleep(0)


def _fire_timers_early(loop):
    # CPython runs a timer once it is due within loop._clock_resolution: 15.625 ms
    # on Windows before 3.13 (GetTickCount64). Widen it so every platform sees a
    # deadline timer fire early. Fail, never skip, if CPython drops the attribute.
    assert isinstance(loop._clock_resolution, float)
    loop._clock_resolution = 0.15


@pytest.mark.parametrize("deadline_ms, maximum", [(200, None), (0, 0.2)])
def test_early_deadline_timer_never_times_out_before_the_deadline(deadline_ms, maximum):
    async def run():
        _fire_timers_early(asyncio.get_running_loop())
        stop = asyncio.Event()
        async with Harness(max_deadline=maximum) as h:
            waker = asyncio.create_task(_keep_loop_awake(stop))
            try:
                await h.submit(h.request(deadline_ms=deadline_ms))
                result = await h.result()
                late = time.monotonic() - h.ctx.deadline
            finally:
                stop.set()
                await waker
            assert 0 <= late <= 1.0
            assert (result.status, result.error_code_enum, result.error_code,
                    result.error_message) == (FAILED, job_pb2.ERROR_CODE_JOB_TIMEOUT,
                                              "TIMEOUT", "job deadline exceeded")
            await asyncio.gather(*h.agent._handler_tasks)
            assert (h.ctx.cancelled, h.stops, h.nats.result_count()) == (True, 1, 1)
    asyncio.run(run())


def test_handler_finishing_after_an_early_deadline_timer_succeeds():
    async def run():
        _fire_timers_early(asyncio.get_running_loop())
        stop = asyncio.Event()
        async with Harness(max_deadline=0.2) as h:
            def release_before_deadline():
                # 100 ms before the deadline: after the early timer, before the deadline.
                if h.ctx is not None and time.monotonic() >= h.ctx.deadline - 0.1:
                    h.release.set()
            waker = asyncio.create_task(_keep_loop_awake(stop, release_before_deadline))
            try:
                await h.submit(h.request())
                result = await h.result()
            finally:
                stop.set()
                await waker
            assert result.status == SUCCEEDED
            await asyncio.gather(*h.agent._handler_tasks)
            assert (h.completions, h.stops, h.ctx.cancelled, h.nats.result_count()) == (1, 0, False, 1)
    asyncio.run(run())


def test_completion_cancel_deadline_race_has_one_terminal_result():
    async def run():
        counts = Counter()
        for index in range(100):
            async with Harness() as h:
                packet = h.request(job_id="race-%d" % index, deadline_ms=30)
                await h.submit(packet)
                assert h.ctx.deadline is not None
                loop = asyncio.get_running_loop()
                tasks = []
                offset = (index % 3 - 1) * 0.004
                done = loop.call_later(0.030 + offset, h.release.set)
                cancel = loop.call_later(0.030 - offset, lambda: tasks.append(
                    asyncio.create_task(h.send_cancel(h.cancel(packet)))))
                try:
                    result = await h.result()
                    counts[result.status] += 1
                    assert result.status in (SUCCEEDED, CANCELLED, FAILED)
                    if result.status == FAILED:
                        assert result.error_code == "TIMEOUT"
                    await asyncio.sleep(0.02)
                    await asyncio.gather(*tasks)
                    assert h.nats.result_count() == 1
                finally:
                    done.cancel()
                    cancel.cancel()
        assert sum(counts.values()) == 100
        print("race status counts", dict(counts))
    asyncio.run(run())


@pytest.mark.parametrize("maximum", [0, -1, float("nan"), float("inf"), True, "1",
                                     pytest.param(10**400, id="overflow")])
def test_invalid_agent_maximum(maximum):
    with pytest.raises(ValueError):
        Agent(max_deadline=maximum)


def test_context_defaults_and_read_only_cancelled():
    ctx = Context(job=job_pb2.JobRequest(), packet=make_packet(),
                  logger=logging.LoggerAdapter(logging.getLogger(__name__), {}))
    assert ctx.deadline is None and ctx.cancelled is False
    with pytest.raises(AttributeError):
        ctx.cancelled = True
