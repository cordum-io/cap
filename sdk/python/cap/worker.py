import asyncio
from typing import Callable, Awaitable, Dict

import nats
from google.protobuf import timestamp_pb2
from cap.pb.coretex.agent.v1 import buspacket_pb2, job_pb2
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import hashes
import hashlib

DEFAULT_PROTOCOL_VERSION = 1
SUBJECT_RESULT = "sys.job.result"


async def run_worker(nats_url: str, subject: str, handler: Callable[[job_pb2.JobRequest], Awaitable[job_pb2.JobResult]],
                     public_keys: Dict[str, ec.EllipticCurvePublicKey] = None,
                     private_key: ec.EllipticCurvePrivateKey = None,
                     sender_id: str = "cap-worker",
                     connect_fn: Callable = nats.connect):

    # Allow injection for tests; defaults to nats.connect.
    nc = await connect_fn(servers=nats_url, name=sender_id)

    async def on_msg(msg):
        packet = buspacket_pb2.BusPacket()
        packet.ParseFromString(msg.data)

        if public_keys:
            public_key = public_keys.get(packet.sender_id)
            if not public_key:
                print(f"worker: no public key found for sender: {packet.sender_id}")
                return

            signature = packet.signature
            packet.ClearField("signature")
            unsigned_data = packet.SerializeToString()
            digest = hashlib.sha256(unsigned_data).digest()
            try:
                public_key.verify(signature, digest, ec.ECDSA(hashes.SHA256()))
            except Exception:
                print(f"worker: invalid signature from sender: {packet.sender_id}")
                return

        req = packet.job_request
        if not req.job_id:
            return
        try:
            res = await handler(req)
        except Exception as exc:  # noqa: BLE001
            res = job_pb2.JobResult(
                job_id=req.job_id,
                status=job_pb2.JOB_STATUS_FAILED,
                error_message=str(exc),
            )
        ts = timestamp_pb2.Timestamp()
        ts.GetCurrentTime()
        out = buspacket_pb2.BusPacket()
        out.trace_id = packet.trace_id
        out.sender_id = sender_id
        out.protocol_version = DEFAULT_PROTOCOL_VERSION
        out.created_at.CopyFrom(ts)
        out.job_result.CopyFrom(res)

        if private_key:
            unsigned_data = out.SerializeToString()
            digest = hashlib.sha256(unsigned_data).digest()
            signature = private_key.sign(digest, ec.ECDSA(hashes.SHA256()))
            out.signature = signature

        await nc.publish(SUBJECT_RESULT, out.SerializeToString())

    await nc.subscribe(subject, queue=subject, cb=on_msg)
    try:
        while True:
            await asyncio.sleep(1)
    finally:
        await nc.drain()
