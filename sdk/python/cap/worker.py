import asyncio
import logging
from typing import Callable, Awaitable, Dict

from google.protobuf import timestamp_pb2

logger = logging.getLogger(__name__)
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import hashes

from cap.subjects import SUBJECT_RESULT
from cap.errors import MalformedPacketError, SignatureInvalidError

DEFAULT_PROTOCOL_VERSION = 1


async def run_worker(nats_url: str, subject: str, handler: Callable[[job_pb2.JobRequest], Awaitable[job_pb2.JobResult]],
                     public_keys: Dict[str, ec.EllipticCurvePublicKey] = None,
                     private_key: ec.EllipticCurvePrivateKey = None,
                     sender_id: str = "cap-worker",
                     connect_fn: Callable = None,
                     middlewares: list = None):
    """Subscribe to a NATS subject and dispatch incoming JobRequests to the handler.

    Results are wrapped in a signed BusPacket and published to the result subject.
    Runs indefinitely until cancelled.

    Args:
        nats_url: NATS server URL (e.g. ``nats://127.0.0.1:4222``).
        subject: NATS subject to subscribe to.
        handler: Async callable that receives a JobRequest and returns a JobResult.
        public_keys: Optional mapping of sender IDs to ECDSA public keys for
            signature verification.
        private_key: Optional ECDSA private key for signing outgoing packets.
        sender_id: Identity reported in outgoing BusPacket envelopes.
        connect_fn: Override the NATS connect function (useful for testing).
        middlewares: Optional list of middleware functions applied in FIFO order.
    """

    # Allow injection for tests; defaults to nats.connect.
    if connect_fn is None:
        try:
            import nats  # type: ignore
        except ImportError as exc:
            raise RuntimeError("nats-py is required to connect to NATS") from exc
        connect_fn = nats.connect

    nc = await connect_fn(servers=nats_url, name=sender_id)

    async def on_msg(msg):
        packet = buspacket_pb2.BusPacket()
        try:
            packet.ParseFromString(msg.data)
        except Exception as exc:
            logger.warning("%s", MalformedPacketError(str(exc)))
            return

        if public_keys:
            public_key = public_keys.get(packet.sender_id)
            if not public_key:
                logger.warning("no public key found for sender: %s", packet.sender_id)
                return

            signature = packet.signature
            packet.ClearField("signature")
            unsigned_data = packet.SerializeToString(deterministic=True)
            packet.signature = signature
            try:
                public_key.verify(signature, unsigned_data, ec.ECDSA(hashes.SHA256()))
            except Exception:
                logger.warning("%s", SignatureInvalidError(f"sender {packet.sender_id}"))
                return

        req = packet.job_request
        if not req.job_id:
            return
        # Apply middleware chain
        wrapped = handler
        if middlewares:
            for mw in reversed(middlewares):
                _next = wrapped
                wrapped = (lambda m, n: (lambda r: m(r, n)))(mw, _next)
        try:
            res = await wrapped(req)
            if res is None:
                res = job_pb2.JobResult(
                    job_id=req.job_id,
                    status=job_pb2.JOB_STATUS_FAILED,
                    error_message="handler returned null",
                )
        except Exception as exc:  # noqa: BLE001
            res = job_pb2.JobResult(
                job_id=req.job_id,
                status=job_pb2.JOB_STATUS_FAILED,
                error_message=str(exc),
            )
        if not res.job_id:
            res.job_id = req.job_id
        if not res.worker_id:
            res.worker_id = sender_id
        ts = timestamp_pb2.Timestamp()
        ts.GetCurrentTime()
        out = buspacket_pb2.BusPacket()
        out.trace_id = packet.trace_id
        out.sender_id = sender_id
        out.protocol_version = DEFAULT_PROTOCOL_VERSION
        out.created_at.CopyFrom(ts)
        out.job_result.CopyFrom(res)

        if private_key:
            unsigned_data = out.SerializeToString(deterministic=True)
            signature = private_key.sign(unsigned_data, ec.ECDSA(hashes.SHA256()))
            out.signature = signature

        await nc.publish(SUBJECT_RESULT, out.SerializeToString(deterministic=True))

    await nc.subscribe(subject, queue=subject, cb=on_msg)
    try:
        while True:
            await asyncio.sleep(1)
    finally:
        await nc.drain()
