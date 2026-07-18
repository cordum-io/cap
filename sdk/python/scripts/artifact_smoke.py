"""Isolated consumer smoke used by verify_artifacts.py."""

import importlib
import json
import sys
from datetime import datetime, timezone
from importlib import metadata
from pathlib import Path

from cryptography.hazmat.primitives.asymmetric import ec


def inside(path: Path, root: Path) -> bool:
    return path == root or root in path.parents


expected = json.loads(sys.argv[1])
repo = Path(sys.argv[2]).resolve()
venv = Path(sys.argv[3]).resolve()
import cap

origin = Path(cap.__file__).resolve()
assert inside(origin, venv), origin
assert not inside(origin, repo), origin
marker = origin.parent / "py.typed"
assert marker.is_file() and inside(marker.resolve(), venv), marker
for module_name in expected:
    importlib.import_module(module_name)
for entry in filter(None, sys.path):
    assert not inside(Path(entry).resolve(), repo), entry

from cap import (
    WORKER_HANDSHAKE_AUDIENCE,
    WorkerHandshakeRequestOptions,
    WorkerTrustConfig,
    WorkerTrustMode,
    build_challenge_request,
    marshal_worker_trust_packet,
    parse_worker_trust_mode,
    unmarshal_worker_trust_packet,
    validate_worker_trust_config,
)
from cap.pb.cordum.agent.v1 import handshake_pb2, job_pb2
from cordum.agent.v1 import job_pb2 as bridge_job_pb2

request = job_pb2.JobRequest(job_id="artifact-smoke")
request.CopyFrom(bridge_job_pb2.JobRequest(job_id="artifact-smoke"))
payload = request.SerializeToString(deterministic=True)
decoded = job_pb2.JobRequest.FromString(payload)
assert decoded.job_id == request.job_id

proof_key = ec.generate_private_key(ec.SECP256R1())
trust_config = WorkerTrustConfig(
    worker_id="artifact-worker",
    expected_agent_id="artifact-agent",
    tenant_id="artifact-tenant",
    audience=WORKER_HANDSHAKE_AUDIENCE,
    proof_key_id="worker-key",
    proof_private_key=proof_key,
    expected_scheduler_id="artifact-scheduler",
    scheduler_public_keys={"scheduler-key": proof_key.public_key()},
    sdk_version="artifact-smoke",
)
validate_worker_trust_config(trust_config)
trust_request = build_challenge_request(
    trust_config,
    WorkerHandshakeRequestOptions(
        request_id="artifact-request",
        trace_id="artifact-trace",
        purpose=handshake_pb2.WORKER_HANDSHAKE_PURPOSE_ISSUE,
        client_nonce=b"a" * 32,
        created_at=datetime.now(timezone.utc),
    ),
)
trust_wire = marshal_worker_trust_packet(trust_request)
trust_decoded = unmarshal_worker_trust_packet(trust_wire)
trust_mode = parse_worker_trust_mode("enforce")
assert trust_mode is WorkerTrustMode.ENFORCE
assert trust_decoded.sender_id == trust_config.worker_id
print(json.dumps({
    "grpcio_version": metadata.version("grpcio"),
    "imports": len(expected),
    "protobuf_version": metadata.version("protobuf"),
    "serialization_bytes": len(payload),
    "worker_trust": {
        "challenge_bytes": len(trust_wire),
        "mode": trust_mode.value,
        "protocol_version": trust_decoded.protocol_version,
        "sender_id": trust_decoded.sender_id,
    },
}, sort_keys=True))
