"""Isolated consumer smoke used by verify_artifacts.py."""

import importlib
import json
import sys
from importlib import metadata
from pathlib import Path


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

from cap.pb.cordum.agent.v1 import job_pb2
from cordum.agent.v1 import job_pb2 as bridge_job_pb2

request = job_pb2.JobRequest(job_id="artifact-smoke")
request.CopyFrom(bridge_job_pb2.JobRequest(job_id="artifact-smoke"))
payload = request.SerializeToString(deterministic=True)
decoded = job_pb2.JobRequest.FromString(payload)
assert decoded.job_id == request.job_id
print(json.dumps({
    "grpcio_version": metadata.version("grpcio"),
    "imports": len(expected),
    "protobuf_version": metadata.version("protobuf"),
    "serialization_bytes": len(payload),
}, sort_keys=True))
