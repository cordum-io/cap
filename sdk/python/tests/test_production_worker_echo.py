import asyncio
import logging
from types import SimpleNamespace

import pytest
from cryptography.hazmat.primitives.asymmetric import ec

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.production_events import (
    ProductionEventAuthority,
    ProductionEventConflictError,
    freeze_production_authority,
)
from cap.production_replay import InMemoryReplayStore
from cap.production_signing import ProductionTrust, verify_production_packet
from cap.runtime import Agent, Context, InMemoryBlobStore
from cap.subjects import SUBJECT_RESULT
from cap.worker_trust import WorkerTrustMode


class RecordingNats:
    def __init__(self) -> None:
        self.published: list[tuple[str, bytes]] = []

    async def publish(self, subject: str, payload: bytes) -> None:
        self.published.append((subject, payload))


def production_context() -> tuple[Context, ProductionEventAuthority]:
    identity = job_pb2.IdentityBinding(
        tenant_id="tenant-a", principal_id="principal-a", actor_id="actor-a"
    )
    dispatch = job_pb2.DispatchIdentity(
        dispatch_id="dispatch-a", attempt=3, assigned_worker_id="worker-a"
    )
    request = job_pb2.JobRequest(
        job_id="job-a", topic="job.test", context_ptr="redis://ctx:production-echo"
    )
    request.identity.CopyFrom(identity)
    request.dispatch.CopyFrom(dispatch)
    packet = buspacket_pb2.BusPacket(trace_id="trace-a", sender_id="scheduler-a")
    packet.job_request.CopyFrom(request)
    packet.identity.CopyFrom(identity)
    logger = logging.LoggerAdapter(logging.getLogger("production-worker-echo"), {})
    return Context(job=request, packet=packet, logger=logger), freeze_production_authority(request)


def production_agent(worker_key: ec.EllipticCurvePrivateKey) -> Agent:
    scheduler_key = ec.generate_private_key(ec.SECP256R1())
    trust = ProductionTrust(
        audience="job.worker.pool-a",
        tenant="tenant-a",
        sender="scheduler-a",
        public_keys={"scheduler-key": scheduler_key.public_key()},
    )
    agent = Agent(
        sender_id="worker-a",
        production_trust=trust,
        replay_store=InMemoryReplayStore(),
    )
    agent._nc = RecordingNats()
    agent._store = InMemoryBlobStore()
    agent._trust_settings = SimpleNamespace(
        mode=WorkerTrustMode.ENFORCE,
        config=SimpleNamespace(
            proof_key_id="worker-key", proof_private_key=worker_key
        )
    )
    agent._worker_trust = SimpleNamespace(session_token=lambda: "session-a")
    agent._trust_admitting = True
    return agent


def test_runtime_result_echoes_and_signs_frozen_authority() -> None:
    key = ec.generate_private_key(ec.SECP256R1())
    agent = production_agent(key)
    ctx, authority = production_context()

    async def publish_results() -> None:
        await agent._publish_result(
            ctx,
            job_pb2.JobResult(
                status=job_pb2.JOB_STATUS_SUCCEEDED, worker_id="worker-a"
            ),
            authority,
        )
        await agent._publish_result(
            ctx,
            job_pb2.JobResult(
                status=job_pb2.JOB_STATUS_SUCCEEDED, worker_id="worker-a"
            ),
            authority,
        )

    asyncio.run(publish_results())

    published = agent._nc.published
    assert [subject for subject, _ in published] == [SUBJECT_RESULT, SUBJECT_RESULT]
    verify = ProductionTrust(
        audience=SUBJECT_RESULT,
        tenant="tenant-a",
        sender="worker-a",
        public_keys={"worker-key": key.public_key()},
    )
    packets = [verify_production_packet(raw, verify) for _, raw in published]
    assert packets[0].job_result.identity == authority.identity
    assert packets[0].job_result.dispatch == authority.dispatch
    assert packets[0].identity == authority.identity
    assert packets[0].auth_token == "session-a"
    assert len(packets[0].signature_metadata.message_id) == 16
    assert packets[0].signature_metadata.message_id != packets[1].signature_metadata.message_id


def test_runtime_result_rejects_conflicting_job_id() -> None:
    agent = production_agent(ec.generate_private_key(ec.SECP256R1()))
    ctx, authority = production_context()

    with pytest.raises(ProductionEventConflictError, match="job id"):
        asyncio.run(
            agent._publish_result(
                ctx,
                job_pb2.JobResult(job_id="job-evil", worker_id="worker-a"),
                authority,
            )
        )


def test_runtime_result_requires_frozen_authority_in_production() -> None:
    agent = production_agent(ec.generate_private_key(ec.SECP256R1()))
    ctx, _ = production_context()

    with pytest.raises(ProductionEventConflictError, match="requires admitted"):
        asyncio.run(
            agent._publish_result(
                ctx,
                job_pb2.JobResult(job_id="job-a", worker_id="worker-a"),
            )
        )


def test_runtime_freezes_authority_before_handler_mutation() -> None:
    key = ec.generate_private_key(ec.SECP256R1())
    agent = production_agent(key)
    ctx, authority = production_context()

    @agent.job("job.test")
    async def mutate_request(context: Context, _data: object) -> dict[str, bool]:
        context.job.identity.actor_id = "actor-evil"
        context.job.dispatch.attempt = 99
        return {"ok": True}

    async def execute_handler() -> None:
        await agent._store.set("ctx:production-echo", b"{}")
        await agent._on_msg(
            SimpleNamespace(data=b""), agent._handlers["job.test"], ctx.packet
        )

    asyncio.run(execute_handler())

    verify = ProductionTrust(
        audience=SUBJECT_RESULT,
        tenant="tenant-a",
        sender="worker-a",
        public_keys={"worker-key": key.public_key()},
    )
    packet = verify_production_packet(agent._nc.published[0][1], verify)
    assert packet.job_result.identity == authority.identity
    assert packet.job_result.dispatch == authority.dispatch
