"""CAP-PRODUCTION managed-worker echo/signing parity for Python.

Mirrors sdk/go/worker/managed_events.go + managed_production.go. The Go SDK
gained real worker-side fencing first; until Python and Node match it, DoD-4
("dispatch/attempt/event identity prevents late results or cancellation from
affecting a different retry attempt") is only true for one of three stable SDKs.

Every assertion here compares against the EXACT admitted request rather than
merely checking a field is non-empty. A non-empty check would stay green if the
producer echoed a fresh-but-wrong identity, which is precisely the bug class
this suite exists to catch.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest
from cryptography.hazmat.primitives.asymmetric import ec

from cap.pb.cordum.agent.v1.buspacket_pb2 import BusPacket
from cap.pb.cordum.agent.v1.job_pb2 import (
    DispatchIdentity,
    IdentityBinding,
    JobCancel,
    JobProgress,
    JobRequest,
    JobResult,
)
from cap.production_events import (
    ProductionEventAuthority,
    ProductionEventConflictError,
    admit_production_event,
    bind_production_event,
    freeze_production_authority,
    seal_production_event,
)
from cap.production_replay import (
    InMemoryReplayStore,
    ReplayConflictError,
    ReplayStoreUnavailableError,
)
from cap.production_signing import ProductionTrust, verify_production_packet

TENANT = "tenant-a"
PRINCIPAL = "principal-a"
ACTOR = "actor-a"
WORKER = "worker-1"
JOB = "job-42"
DISPATCH = "dispatch-abc"
ATTEMPT = 7
KEY_ID = "worker-key"
OUT_SUBJECT = "sys.job.result"


def _identity() -> IdentityBinding:
    return IdentityBinding(
        tenant_id=TENANT, principal_id=PRINCIPAL, actor_id=ACTOR, delegation_id="delegation-a"
    )


def _dispatch() -> DispatchIdentity:
    return DispatchIdentity(dispatch_id=DISPATCH, attempt=ATTEMPT, assigned_worker_id=WORKER)


def _request() -> JobRequest:
    return JobRequest(
        job_id=JOB,
        topic="sys.job.submit",
        tenant_id=TENANT,
        principal_id=PRINCIPAL,
        identity=_identity(),
        dispatch=_dispatch(),
    )


def _key() -> ec.EllipticCurvePrivateKey:
    return ec.generate_private_key(ec.SECP256R1())


def _authority() -> ProductionEventAuthority:
    return freeze_production_authority(_request())


def _packet(**kwargs) -> BusPacket:
    """Envelope identity is required: verify_production_packet binds
    packet.identity.tenant_id to trust.tenant."""
    return BusPacket(sender_id=WORKER, identity=_identity(), **kwargs)


def _seal(packet: BusPacket, key: ec.EllipticCurvePrivateKey, *, audience: str = OUT_SUBJECT) -> bytes:
    return seal_production_event(
        packet, key=key, key_id=KEY_ID, audience=audience, lifetime=timedelta(minutes=5)
    )


# --- echo: exact field equality against the admitted request ------------------


@pytest.mark.parametrize(
    "event",
    [JobResult(job_id=JOB), JobProgress(job_id=JOB), JobCancel(job_id=JOB)],
    ids=["result", "progress", "cancel"],
)
def test_outgoing_events_echo_admitted_identity_and_dispatch(event) -> None:
    authority = _authority()
    bind_production_event(event, authority)

    # Exact equality, not "is set". A producer minting its own identity would
    # pass a non-empty check and fail this one.
    assert event.identity == _identity()
    assert event.dispatch == _dispatch()
    assert event.dispatch.dispatch_id == DISPATCH
    assert event.dispatch.attempt == ATTEMPT
    assert event.dispatch.assigned_worker_id == WORKER
    assert event.job_id == JOB


def test_binding_does_not_alias_the_admitted_authority() -> None:
    """Mutating an emitted event must not retroactively edit the frozen authority."""
    authority = _authority()
    first, second = JobResult(job_id=JOB), JobResult(job_id=JOB)
    bind_production_event(first, authority)
    first.dispatch.attempt = 999
    first.identity.actor_id = "actor-IMPOSTER"

    bind_production_event(second, authority)
    assert second.dispatch.attempt == ATTEMPT
    assert second.identity.actor_id == ACTOR


# --- handler-supplied conflicts are rejected, never silently overridden -------


@pytest.mark.parametrize(
    "mutate",
    [
        lambda e: e.identity.CopyFrom(IdentityBinding(tenant_id="tenant-EVIL")),
        lambda e: e.identity.CopyFrom(
            IdentityBinding(
                tenant_id=TENANT, principal_id=PRINCIPAL, actor_id="actor-IMPOSTER"
            )
        ),
        lambda e: e.dispatch.CopyFrom(
            DispatchIdentity(dispatch_id="dispatch-OTHER", attempt=ATTEMPT, assigned_worker_id=WORKER)
        ),
        lambda e: e.dispatch.CopyFrom(
            DispatchIdentity(dispatch_id=DISPATCH, attempt=ATTEMPT + 1, assigned_worker_id=WORKER)
        ),
        lambda e: e.dispatch.CopyFrom(
            DispatchIdentity(dispatch_id=DISPATCH, attempt=ATTEMPT, assigned_worker_id="worker-OTHER")
        ),
        lambda e: setattr(e, "job_id", "job-OTHER"),
    ],
    ids=["tenant", "actor", "dispatch_id", "attempt", "worker", "job_id"],
)
def test_conflicting_handler_authority_is_rejected(mutate) -> None:
    authority = _authority()
    event = JobResult(job_id=JOB)
    mutate(event)
    with pytest.raises(ProductionEventConflictError):
        bind_production_event(event, authority)


def test_handler_may_leave_authority_unset() -> None:
    """An untouched event is the normal path: bind fills it rather than failing."""
    authority = _authority()
    event = JobResult()
    bind_production_event(event, authority)
    assert event.job_id == JOB
    assert event.identity == _identity()


# --- signing: production signer, real outbound subject, fresh message id ------


def test_sealed_event_verifies_against_the_actual_outbound_subject() -> None:
    key = _key()
    authority = _authority()
    result = JobResult(job_id=JOB)
    bind_production_event(result, authority)
    packet = _packet(job_result=result)

    raw = _seal(packet, key)

    trust = ProductionTrust(
        audience=OUT_SUBJECT,
        public_keys={KEY_ID: key.public_key()},
        tenant=TENANT,
        sender=WORKER,
    )
    verified = verify_production_packet(raw, trust)
    assert verified.signature_metadata.audience == OUT_SUBJECT
    assert verified.signature_metadata.key_id == KEY_ID
    assert len(verified.signature_metadata.message_id) == 16
    assert verified.job_result.dispatch == _dispatch()
    assert verified.job_result.identity == _identity()


def test_sealed_event_is_rejected_under_a_different_subject() -> None:
    """Audience binding must be to the subject actually published on."""
    key = _key()
    packet = _packet(job_result=JobResult(job_id=JOB))
    raw = _seal(packet, key, audience="sys.job.result")

    trust = ProductionTrust(
        audience="sys.job.progress",
        public_keys={KEY_ID: key.public_key()},
        tenant=TENANT,
        sender=WORKER,
    )
    with pytest.raises(Exception):
        verify_production_packet(raw, trust)


def test_each_sealed_event_carries_a_fresh_unique_message_id() -> None:
    """Uniqueness across N sends, not merely non-empty: a constant 16-byte id
    would satisfy a length check and still break replay de-duplication."""
    key = _key()
    seen: set[bytes] = set()
    for _ in range(8):
        packet = _packet(job_result=JobResult(job_id=JOB))
        raw = _seal(packet, key)
        trust = ProductionTrust(
            audience=OUT_SUBJECT,
            public_keys={KEY_ID: key.public_key()},
            tenant=TENANT,
            sender=WORKER,
        )
        message_id = verify_production_packet(raw, trust).signature_metadata.message_id
        assert len(message_id) == 16
        assert message_id not in seen, "reused production message id"
        seen.add(message_id)
    assert len(seen) == 8


def test_sealed_event_carries_a_bounded_future_expiry() -> None:
    key = _key()
    packet = _packet(job_result=JobResult(job_id=JOB))
    raw = _seal(packet, key)
    trust = ProductionTrust(
        audience=OUT_SUBJECT,
        public_keys={KEY_ID: key.public_key()},
        tenant=TENANT,
        sender=WORKER,
    )
    verified = verify_production_packet(raw, trust)
    expires = verified.signature_metadata.expires_at.ToDatetime().replace(tzinfo=timezone.utc)
    assert expires > datetime.now(timezone.utc)


def test_sealing_requires_a_key_id() -> None:
    key = _key()
    packet = _packet(job_result=JobResult(job_id=JOB))
    with pytest.raises(ValueError):
        seal_production_event(
            packet, key=key, key_id="", audience=OUT_SUBJECT, lifetime=timedelta(minutes=5)
        )


# --- inbound admission: redelivery dropped, replay failures fail closed -------


def test_identical_redelivery_is_dropped_before_the_handler() -> None:
    store = InMemoryReplayStore()
    key = _key()
    packet = _packet(job_request=_request())
    raw = _seal(packet, key, audience="sys.job.submit")
    trust = ProductionTrust(
        audience="sys.job.submit",
        public_keys={KEY_ID: key.public_key()},
        tenant=TENANT,
        sender=WORKER,
    )

    calls: list[str] = []

    def handler(_request: JobRequest) -> None:
        calls.append("invoked")

    admit_production_event(raw, trust=trust, replay=store, handler=handler)
    admit_production_event(raw, trust=trust, replay=store, handler=handler)

    assert calls == ["invoked"], "redelivery must not reach the handler a second time"


def test_replay_conflict_fails_closed_without_invoking_the_handler() -> None:
    class ConflictingStore:
        def admit(self, *_args, **_kwargs):
            raise ReplayConflictError("same message id, different digest")

    key = _key()
    packet = _packet(job_request=_request())
    raw = _seal(packet, key, audience="sys.job.submit")
    trust = ProductionTrust(
        audience="sys.job.submit",
        public_keys={KEY_ID: key.public_key()},
        tenant=TENANT,
        sender=WORKER,
    )
    calls: list[str] = []

    with pytest.raises(ReplayConflictError):
        admit_production_event(
            raw, trust=trust, replay=ConflictingStore(), handler=lambda _r: calls.append("x")
        )
    assert calls == []


def test_replay_store_unavailable_fails_closed_without_invoking_the_handler() -> None:
    class DeadStore:
        def admit(self, *_args, **_kwargs):
            raise ReplayStoreUnavailableError("backend down")

    key = _key()
    packet = _packet(job_request=_request())
    raw = _seal(packet, key, audience="sys.job.submit")
    trust = ProductionTrust(
        audience="sys.job.submit",
        public_keys={KEY_ID: key.public_key()},
        tenant=TENANT,
        sender=WORKER,
    )
    calls: list[str] = []

    with pytest.raises(ReplayStoreUnavailableError):
        admit_production_event(
            raw, trust=trust, replay=DeadStore(), handler=lambda _r: calls.append("x")
        )
    assert calls == [], "an unreachable replay store must never be read as first-seen"


def test_admission_rejects_identity_mismatch_before_the_handler() -> None:
    """The envelope identity is authoritative; a payload mirror that disagrees
    must be refused rather than reconciled."""
    key = _key()
    request = _request()
    request.identity.actor_id = "actor-IMPOSTER"
    packet = _packet(job_request=request)
    raw = _seal(packet, key, audience="sys.job.submit")
    trust = ProductionTrust(
        audience="sys.job.submit",
        public_keys={KEY_ID: key.public_key()},
        tenant=TENANT,
        sender=WORKER,
    )
    calls: list[str] = []

    with pytest.raises(Exception):
        admit_production_event(
            raw, trust=trust, replay=InMemoryReplayStore(), handler=lambda _r: calls.append("x")
        )
    assert calls == []
