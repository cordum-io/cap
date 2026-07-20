"""CAP-PRODUCTION managed-worker event echo, sealing, and admission.

Python parity for sdk/go/worker/managed_events.go + managed_production.go.

The contract, in one line: the authority a worker echoes on outgoing events is
frozen from the ADMITTED request before handler code runs, and a handler that
contradicts it is rejected rather than silently overridden. Silent override is
tempting and wrong -- it turns a handler bug (or a compromised handler) into a
correctly-signed event carrying someone else's identity.

Ordering matters and is not incidental:
  verify signature/audience/expiry -> validate identity binding -> replay-admit
  -> only then invoke the handler.
Replay admission sits AFTER verification so an unauthenticated packet can never
consume a replay slot, and BEFORE the handler so a redelivery cannot re-run a
side effect.
"""

from __future__ import annotations

import hashlib
import os
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Callable, Optional, Protocol

from cryptography.hazmat.primitives.asymmetric import ec

from .constants import DEFAULT_PROTOCOL_VERSION
from .pb.cordum.agent.v1.buspacket_pb2 import BusPacket
from .pb.cordum.agent.v1.job_pb2 import DispatchIdentity, IdentityBinding, JobRequest
from .production_replay import ReplayOutcome
from .production_signing import (
    ALGORITHM,
    DEFAULT_MAX_LIFETIME,
    DOMAIN,
    PROFILE_VERSION,
    ProductionSignatureError,
    ProductionTrust,
    extract_signature,
    sign_production_packet,
    verify_production_packet,
)
from .production_validation import validate_identity_binding

MESSAGE_ID_BYTES = 16

__all__ = [
    "ProductionEventAuthority",
    "ProductionEventConflictError",
    "admit_production_event",
    "bind_production_event",
    "freeze_production_authority",
    "seal_production_event",
]


class ProductionEventConflictError(ValueError):
    """A handler supplied identity, dispatch, or job_id contradicting the admitted request."""


class _ReplayStore(Protocol):
    def admit(
        self,
        tenant: str,
        audience: str,
        sender: str,
        message_id: bytes,
        digest: bytes,
        expires_at: datetime,
    ) -> ReplayOutcome: ...


@dataclass(frozen=True)
class ProductionEventAuthority:
    """Immutable snapshot of the admitted request's authority.

    Holds deep copies, so neither handler code nor a previously emitted event can
    reach back and mutate what later events will echo.
    """

    job_id: str
    identity: IdentityBinding
    dispatch: DispatchIdentity


def freeze_production_authority(request: JobRequest) -> ProductionEventAuthority:
    """Snapshot the admitted request's authority before any handler runs."""
    if request is None:
        raise ProductionEventConflictError("cannot freeze authority from a missing request")
    if not request.job_id:
        raise ProductionEventConflictError("admitted request carries no job id")
    identity = IdentityBinding()
    identity.CopyFrom(request.identity)
    dispatch = DispatchIdentity()
    dispatch.CopyFrom(request.dispatch)
    return ProductionEventAuthority(job_id=request.job_id, identity=identity, dispatch=dispatch)


def bind_production_event(event, authority: ProductionEventAuthority) -> None:
    """Echo the frozen authority onto an outgoing Result/Progress/Cancel.

    An unset field is filled. A field the handler set to something DIFFERENT is a
    conflict and raises -- we do not quietly correct it, because a handler that
    believes it is emitting for another job/attempt has a bug worth surfacing.
    """
    if event is None:
        raise ProductionEventConflictError("cannot bind a missing event")
    if authority is None:
        raise ProductionEventConflictError("cannot bind without an admitted authority")

    if event.job_id and event.job_id != authority.job_id:
        raise ProductionEventConflictError(
            f"event job id {event.job_id!r} conflicts with admitted {authority.job_id!r}"
        )
    event.job_id = authority.job_id

    if event.HasField("identity") and event.identity != authority.identity:
        raise ProductionEventConflictError("event identity conflicts with admitted authority")
    event.identity.CopyFrom(authority.identity)

    if event.HasField("dispatch") and event.dispatch != authority.dispatch:
        raise ProductionEventConflictError("event dispatch conflicts with admitted authority")
    event.dispatch.CopyFrom(authority.dispatch)


def seal_production_event(
    packet: BusPacket,
    *,
    key: ec.EllipticCurvePrivateKey,
    key_id: str,
    audience: str,
    lifetime: timedelta = DEFAULT_MAX_LIFETIME,
    now: Optional[Callable[[], datetime]] = None,
) -> bytes:
    """Stamp fresh signature metadata and produce signed production wire bytes.

    `audience` MUST be the subject the caller is about to publish on. Binding to
    anything else (a configured default, the inbound subject) would let a packet
    captured on one subject be replayed onto another.
    """
    if not key_id or not key_id.strip():
        raise ValueError("production sealing requires a key id")
    if not audience or not audience.strip():
        raise ValueError("production sealing requires the outbound subject as audience")
    if lifetime <= timedelta(0) or lifetime > DEFAULT_MAX_LIFETIME:
        raise ValueError("production sealing requires a bounded positive lifetime")

    outgoing = BusPacket()
    outgoing.CopyFrom(packet)
    outgoing.ClearField("signature")
    # extract_signature rejects wire without field 4, so an unset protocol
    # version yields bytes that cannot be verified by anyone. Defaulting here
    # keeps that failure impossible rather than merely documented; a caller that
    # set it explicitly is left alone.
    if not outgoing.protocol_version:
        outgoing.protocol_version = DEFAULT_PROTOCOL_VERSION

    metadata = outgoing.signature_metadata
    metadata.profile_version = PROFILE_VERSION
    metadata.algorithm = ALGORITHM
    # Fresh per message. A reused id silently breaks replay de-duplication for
    # every consumer, so this is generated here rather than taken from a caller.
    metadata.message_id = os.urandom(MESSAGE_ID_BYTES)
    metadata.audience = audience
    metadata.key_id = key_id
    issued = now() if now is not None else datetime.now(timezone.utc)
    metadata.expires_at.FromDatetime(issued + lifetime)

    return sign_production_packet(outgoing, key)


def admit_production_event(
    raw: bytes,
    *,
    trust: ProductionTrust,
    replay: _ReplayStore,
    handler: Callable[[JobRequest], None],
) -> Optional[BusPacket]:
    """Verify, identity-check, and replay-admit raw wire bytes, then dispatch.

    Returns the verified packet when the handler ran, or None when the message
    was a duplicate redelivery. Replay conflict and replay-store-unavailable both
    propagate: the caller must fail closed (NAK/retry), never ack-and-drop.
    """
    packet = verify_production_packet(raw, trust)

    request = packet.job_request if packet.HasField("job_request") else None
    if request is not None and packet.HasField("identity"):
        # Envelope identity is authoritative; a payload mirror that disagrees is
        # refused rather than reconciled.
        validate_identity_binding(request, packet.identity)

    unsigned, _ = extract_signature(raw)
    digest = hashlib.sha256(DOMAIN + unsigned).digest()
    metadata = packet.signature_metadata
    expires_at = metadata.expires_at.ToDatetime(tzinfo=timezone.utc)

    outcome = replay.admit(
        trust.tenant,
        metadata.audience,
        packet.sender_id,
        metadata.message_id,
        digest,
        expires_at,
    )
    if outcome is not ReplayOutcome.FIRST:
        return None

    handler(request)
    return packet
