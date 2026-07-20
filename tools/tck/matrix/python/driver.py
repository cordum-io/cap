"""Python side of the CAP cross-language fixture matrix.

Run as ``driver.py produce|consume`` with the request JSON on stdin and the
response JSON on stdout. Only the installed ``cap`` package's public API is
used, so a green matrix edge is evidence about the published wheel rather than
about repository source.
"""

from __future__ import annotations

import base64
import hashlib
import json
import sys
from typing import Any, Dict, List

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

from cap.pb.cordum.agent.v1.buspacket_pb2 import BusPacket
from cap.production_signing import (
    ProductionTrust,
    extract_signature,
    sign_production_packet,
    verify_production_packet,
)
from cap.validate import validate_bus_packet

SDK = "python"


def build_packet(case: Dict[str, Any], corpus: Dict[str, Any], key_id: str,
                 created_at_unix: int, expires_at_unix: int) -> BusPacket:
    """Build one corpus case into a BusPacket using only generated types."""
    packet = BusPacket()
    packet.trace_id = case["traceId"]
    packet.sender_id = corpus["senderId"]
    packet.created_at.seconds = created_at_unix
    packet.protocol_version = 1
    metadata = packet.signature_metadata
    metadata.profile_version = "cap-production-v1"
    metadata.algorithm = "ECDSA-P256-SHA256"
    metadata.message_id = base64.b64decode(case["messageId"])
    metadata.audience = corpus["audience"]
    metadata.expires_at.seconds = expires_at_unix
    metadata.key_id = key_id
    identity = case["identity"]
    packet.identity.tenant_id = identity["tenantId"]
    packet.identity.principal_id = identity["principalId"]
    packet.identity.actor_id = identity["actorId"]
    packet.identity.delegation_id = identity["delegationId"]
    _attach_payload(packet, case["payload"])
    return packet


def _attach_payload(packet: BusPacket, payload: Dict[str, Any]) -> None:
    kind = payload.get("kind")
    if kind == "jobRequest":
        request = packet.job_request
        request.job_id = payload.get("jobId", "")
        request.topic = payload.get("topic", "")
        request.tenant_id = payload.get("tenantId", "")
        request.principal_id = payload.get("principalId", "")
    elif kind == "jobResult":
        result = packet.job_result
        result.job_id = payload.get("jobId", "")
        result.status = payload.get("status", 0)
        result.worker_id = payload.get("workerId", "")
        result.execution_ms = payload.get("executionMs", 0)
        _attach_dispatch(result.dispatch, payload)
    elif kind == "jobProgress":
        progress = packet.job_progress
        progress.job_id = payload.get("jobId", "")
        progress.step_id = payload.get("stepId", "")
        progress.percent = payload.get("percent", 0)
        progress.message = payload.get("message", "")
        _attach_dispatch(progress.dispatch, payload)
    elif kind == "heartbeat":
        heartbeat = packet.heartbeat
        heartbeat.worker_id = payload.get("workerId", "")
        heartbeat.region = payload.get("region", "")
        heartbeat.type = payload.get("type", "")
        heartbeat.active_jobs = payload.get("activeJobs", 0)
        heartbeat.pool = payload.get("pool", "")
    else:
        raise ValueError(f"unsupported payload kind {kind!r}")


def _attach_dispatch(target: Any, payload: Dict[str, Any]) -> None:
    dispatch = payload.get("dispatch")
    if not dispatch:
        return
    target.dispatch_id = dispatch.get("dispatchId", "")
    target.attempt = dispatch.get("attempt", 0)
    target.assigned_worker_id = dispatch.get("assignedWorkerId", "")


def digests(raw: bytes, packet: BusPacket) -> Dict[str, str]:
    """Recompute both digests independently of any producer claim."""
    unsigned, _ = extract_signature(raw)
    clone = BusPacket()
    clone.CopyFrom(packet)
    clone.signature = b""
    canonical = clone.SerializeToString(deterministic=True)
    return {
        "normalizedDigest": hashlib.sha256(canonical).hexdigest(),
        "preimageDigest": hashlib.sha256(unsigned).hexdigest(),
    }


def produce(request: Dict[str, Any]) -> Dict[str, Any]:
    key = serialization.load_pem_private_key(
        request["privateKeyPem"].encode(), password=None
    )
    corpus = request["corpus"]
    fixtures: List[Dict[str, Any]] = []
    for case in corpus["cases"]:
        packet = build_packet(
            case, corpus, request["keyId"],
            request["createdAtUnix"], request["expiresAtUnix"],
        )
        raw = sign_production_packet(packet, key)
        fixtures.append({
            "case": case["name"],
            "wire": base64.b64encode(raw).decode(),
            "keyId": request["keyId"],
            **digests(raw, packet),
        })
    return {"sdk": SDK, "fixtures": fixtures}


def consume(request: Dict[str, Any]) -> Dict[str, Any]:
    return {"sdk": SDK, "results": [_run_job(request, job) for job in request["jobs"]]}


def _run_job(request: Dict[str, Any], job: Dict[str, Any]) -> Dict[str, Any]:
    result = {"id": job["id"], "ok": False, "normalizedDigest": "",
              "preimageDigest": "", "error": ""}
    try:
        raw = base64.b64decode(job["wire"])
        public_key = serialization.load_der_public_key(
            base64.b64decode(job["publicKeyDer"])
        )
        if not isinstance(public_key, ec.EllipticCurvePublicKey):
            raise ValueError("public key is not ECDSA")
        trust = ProductionTrust(
            audience=request["audience"],
            public_keys={job["keyId"]: public_key},
            tenant=request["tenantId"],
            sender=request["senderId"],
        )
        packet = verify_production_packet(raw, trust)
        errors = validate_bus_packet(packet)
        if errors:
            raise ValueError(f"validate: {errors}")
        result.update(digests(raw, packet), ok=True)
    except Exception as exc:
        result["error"] = f"{type(exc).__name__}: {exc}"
    return result


def main() -> int:
    if len(sys.argv) != 2 or sys.argv[1] not in {"produce", "consume"}:
        print("usage: driver.py produce|consume", file=sys.stderr)
        return 2
    request = json.load(sys.stdin)
    handler = produce if sys.argv[1] == "produce" else consume
    json.dump(handler(request), sys.stdout)
    return 0


if __name__ == "__main__":
    sys.exit(main())
