"""Deterministic no-LLM LangGraph workloads, the in-process golden harness and the wheel smoke.

`python -I golden_graphs.py` (from outside the repo) is the installed-wheel smoke.
"""
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[4]
TRAP = REPO / "testdata" / "adapters" / "nettrap" / "sitecustomize.py"
if __name__ == "__main__":  # the smoke loads the trap by path before any framework import
    import importlib.util

    _spec = importlib.util.spec_from_file_location("sitecustomize", TRAP)
    _spec.loader.exec_module(importlib.util.module_from_spec(_spec))
    sys.modules["golden_graphs"] = sys.modules["__main__"]

import asyncio  # noqa: E402
import json  # noqa: E402
import logging  # noqa: E402
import os  # noqa: E402
import subprocess  # noqa: E402
import tempfile  # noqa: E402
import time  # noqa: E402
from collections import Counter  # noqa: E402
from dataclasses import dataclass, field  # noqa: E402
from datetime import datetime, timedelta, timezone  # noqa: E402
from types import SimpleNamespace  # noqa: E402
from typing import Any, Callable, Dict, List, Optional, TypedDict  # noqa: E402

import cap  # noqa: E402
from cap.packet_boundary import finalize_packet  # noqa: E402
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2  # noqa: E402
from cap.testing import MockNATS  # noqa: E402
from cryptography import x509  # noqa: E402
from cryptography.hazmat.primitives import hashes, serialization  # noqa: E402
from cryptography.hazmat.primitives.asymmetric import ec  # noqa: E402
from cryptography.x509.oid import NameOID  # noqa: E402
from langgraph.config import get_stream_writer  # noqa: E402
from langgraph.graph import END, START, StateGraph  # noqa: E402
from langsmith.utils import tracing_is_enabled  # noqa: E402
from pydantic import BaseModel, ConfigDict, Field  # noqa: E402

import cap_adapter_langgraph as adapter  # noqa: E402

GOLDEN = REPO / "testdata" / "adapters" / "framework-golden-v1.json"
TOPIC, WORKER_ID, TENANT, SCHEDULER_ID = "job.adapter.langgraph", "wrk-lg-golden", "tenant-golden", "sched-golden"
SCHEDULER_KEY, PROOF_KEY = ec.generate_private_key(ec.SECP256R1()), ec.generate_private_key(ec.SECP256R1())
LIMITS = adapter.Limits(65536, 65536, 8, 65536, 30000)
COUNTS: Counter = Counter()
CANCEL_SEEN: List[bool] = []
TRACING_SEEN: List[Any] = []


class TextInput(BaseModel):
    model_config = ConfigDict(extra="forbid")
    text: str


class RepeatInput(TextInput):
    times: int = Field(ge=1, le=1_000_000)


class CanaryInput(BaseModel):
    model_config = ConfigDict(extra="forbid")
    canary: str


class State(TypedDict, total=False):
    text: str
    times: int
    canary: str
    steps: List[str]
    summary: str
    tracing: Any


class TextOut(TypedDict):
    text: str


class StepsOut(TypedDict):
    text: str
    steps: List[str]


class SummaryOut(TypedDict):
    summary: str


class TracingOut(TypedDict):
    tracing: Any


def progress(marker: str) -> None:
    get_stream_writer()({"cap.progress": marker})


async def echo(state: State) -> State:
    return {"text": state["text"]}


async def upper(state: State) -> State:
    progress("upper")
    return {"text": state["text"].upper(), "steps": ["upper"]}


async def suffix(state: State) -> State:
    progress("suffix")
    return {"text": state["text"] + "!", "steps": state["steps"] + ["suffix"]}


async def summarize(state: State) -> State:
    progress("summarize")
    summary = state["text"].upper()
    get_stream_writer()({"cap.artifact": {"name": "summary", "mediaType": "text/plain", "content": summary}})
    return {"summary": summary}


async def prepare(state: State) -> State:
    progress("prepare")
    raise RuntimeError("prepare failed for " + state["canary"])


async def wait(state: State) -> State:
    progress("started")
    saw = False
    try:
        await asyncio.Event().wait()
    except asyncio.CancelledError:
        saw = True
        raise
    finally:
        CANCEL_SEEN.append(saw)
    return {}


async def repeat(state: State) -> State:
    return {"text": state["text"] * state["times"]}


async def probe(state: State) -> State:
    TRACING_SEEN.append(tracing_is_enabled())
    return {"tracing": TRACING_SEEN[-1]}


def chain(key: str, out: type, *nodes: Callable) -> Callable[[], Any]:
    def factory() -> Any:
        COUNTS[key] += 1
        graph = StateGraph(State, output_schema=out)
        previous = START
        for node in nodes:
            graph.add_node(node.__name__, node)
            graph.add_edge(previous, node.__name__)
            previous = node.__name__
        graph.add_edge(previous, END)
        return graph.compile()
    return factory


echo_graph = chain("echo", TextOut, echo)
upper_graph = chain("two-step-upper", StepsOut, upper, suffix)
summary_graph = chain("summarize-with-artifact", SummaryOut, summarize)
raise_graph = chain("raise-in-step", TextOut, prepare)
wait_graph = chain("wait-for-cancel", TextOut, wait)
repeat_graph = chain("repeat-text", TextOut, repeat)
probe_graph = chain("tracing-probe", TracingOut, probe)
WORKLOADS = {key: {"factory": "golden_graphs:" + factory, "inputModel": "golden_graphs:" + model}
             for key, factory, model in [
                 ("echo", "echo_graph", "TextInput"), ("two-step-upper", "upper_graph", "TextInput"),
                 ("summarize-with-artifact", "summary_graph", "TextInput"),
                 ("raise-in-step", "raise_graph", "CanaryInput"), ("wait-for-cancel", "wait_graph", "TextInput"),
                 ("repeat-text", "repeat_graph", "RepeatInput"), ("tracing-probe", "probe_graph", "TextInput")]}


def golden_cases() -> List[Dict[str, Any]]:
    return json.loads(GOLDEN.read_text(encoding="utf-8"))["cases"]


def case_limits(case: Dict[str, Any], base: adapter.Limits = LIMITS) -> adapter.Limits:
    limits = case.get("limits", {})  # R7: golden limits are operator bounds
    return adapter.Limits(base.max_input_bytes, limits.get("maxOutputBytes", base.max_output_bytes),
                          base.max_artifacts, base.max_artifact_bytes, limits.get("deadlineMs", base.max_deadline_ms))


@dataclass
class Run:
    status: str
    error_code: str
    error_string: str
    progress: List[str]
    output: Any
    artifacts: List[Dict[str, str]]
    published: List[bytes] = field(repr=False)
    logs: List[str] = field(repr=False)
    elapsed: float
    after_cancel: Optional[float]


class _Capture(logging.Handler):
    def __init__(self) -> None:
        super().__init__(logging.WARNING)
        self.lines: List[str] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.lines.append(logging.Formatter().format(record) + repr(vars(record)))


def trust_config() -> cap.WorkerTrustConfig:
    return cap.WorkerTrustConfig(WORKER_ID, "agent-golden", TENANT, cap.WORKER_HANDSHAKE_AUDIENCE, "proof-1",
                                 PROOF_KEY, SCHEDULER_ID, {"sched-1": SCHEDULER_KEY.public_key()}, "golden")


def signed(body: Callable[[buspacket_pb2.BusPacket], None], job_id: str) -> Any:
    packet = buspacket_pb2.BusPacket(trace_id="trace-" + job_id, sender_id=SCHEDULER_ID, protocol_version=1)
    packet.created_at.GetCurrentTime()
    body(packet)
    return SimpleNamespace(data=finalize_packet(packet, SCHEDULER_KEY).SerializeToString(deterministic=True))


async def run_case(case: Dict[str, Any], *, limits: Optional[adapter.Limits] = None,
                   edit: Callable[[job_pb2.JobRequest], None] = lambda req: None, timeout: float = 10.0) -> Run:
    """Run one golden case through a real cap.Agent over MockNATS (worker trust warn, no session)."""
    COUNTS.clear()
    nats, backend = MockNATS(), cap.InMemoryResourceBackend()
    registry = cap.ResourceRegistry([cap.ResourceBackendRegistration("memory", 1 << 22, backend, backend)])
    agent = cap.Agent(connect_fn=nats.connect, worker_id=WORKER_ID, heartbeat_interval=60,
                      worker_trust_mode="warn", worker_trust=trust_config(), worker_trust_timeout=0.1,
                      worker_trust_retries=1, resource_registry=registry, result_backend_id="memory",
                      logger=logging.getLogger("golden.agent"), **adapter.agent_options(limits or case_limits(case)))
    agent.job(TOPIC)(adapter.make_handler(adapter.load_workloads(WORKLOADS)))
    capture, job_id = _Capture(), "job-" + case["id"]
    trusted = cap.TrustedResourceContext(TENANT, job_id)
    logging.getLogger().addHandler(capture)
    try:
        await agent.start()
        ref = await registry.externalize("memory", json.dumps(case["input"]).encode(), media_type="application/json",
                                         purpose="job-context", trusted=trusted,
                                         expires_at=datetime.now(timezone.utc) + timedelta(hours=1))
        req = job_pb2.JobRequest(job_id=job_id, topic=TOPIC, context_ref=ref,
                                 dispatch=job_pb2.DispatchIdentity(dispatch_id="d-1", attempt=1,
                                                                   assigned_worker_id=WORKER_ID))
        req.meta.capability = case["workload"]
        edit(req)
        start, cancel_at, marks, published = time.monotonic(), None, [], []
        await nats.subscriptions[TOPIC](signed(lambda p: p.job_request.CopyFrom(req), job_id))
        while True:
            subject, data = await asyncio.wait_for(nats.published.get(), timeout)
            published.append(data)
            packet = buspacket_pb2.BusPacket.FromString(data)
            if subject == cap.SUBJECT_PROGRESS:
                marks.append(packet.job_progress.message)
                if case.get("cancel", {}).get("afterProgress") == marks[-1]:
                    cancel = job_pb2.JobCancel(job_id=job_id, dispatch=req.dispatch, reason="golden", requested_by="golden")
                    cancel_at = time.monotonic()
                    await nats.subscriptions[cap.SUBJECT_CANCEL](signed(lambda p: p.job_cancel.CopyFrom(cancel),
                                                                        job_id))
            if subject == cap.SUBJECT_RESULT:
                result, done = packet.job_result, time.monotonic()
                break
        output = None
        if result.HasField("result_ref"):
            output = json.loads((await registry.resolve(result.result_ref, trusted)).content)
        artifacts = [{"name": r.purpose.split(":", 1)[1], "mediaType": r.media_type,
                      "content": (await registry.resolve(r, trusted)).content.decode()} for r in result.artifact_refs]
    finally:
        await agent.close()
        logging.getLogger().removeHandler(capture)
    return Run(job_pb2.JobStatus.Name(result.status), job_pb2.ErrorCode.Name(result.error_code_enum),
               result.error_code, marks, output, artifacts, published, capture.lines, done - start,
               None if cancel_at is None else done - cancel_at)


def compare(case: Dict[str, Any], run: Run) -> Dict[str, Any]:
    """Return (got, want) views of the golden expectation; they must be equal."""
    want = {k: v for k, v in case["expect"].items() if k != "forbiddenText"}
    want.setdefault("artifacts", [])
    got = {"status": run.status, "progress": run.progress, "artifacts": run.artifacts}
    if "output" in want or run.status == "JOB_STATUS_SUCCEEDED":
        got["output"] = run.output
    if "errorCode" in want:
        got["errorCode"] = run.error_code
    if "errorString" in want:
        got["errorString"] = run.error_string
    for text in case["expect"].get("forbiddenText", []):
        got.setdefault("leaks", []).extend(
            where for where, blobs in (("published", run.published), ("logs", [s.encode() for s in run.logs]))
            if any(text.encode() in blob for blob in blobs))
        want.setdefault("leaks", [])
    return {"got": got, "want": want}


def write_operator_files(directory: Path, url: str = "tls://127.0.0.1:4222") -> Path:
    """Write cap-adapter-config/v1 and cap-adapter-trust/v1 with generated P-256 keys; return the config path."""
    now, pem = datetime.now(timezone.utc), serialization.Encoding.PEM

    def cert(subject: str, key: Any, issuer: str, signer: Any, ca: bool) -> bytes:
        name = lambda cn: x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, cn)])  # noqa: E731
        built = (x509.CertificateBuilder().subject_name(name(subject)).issuer_name(name(issuer))
                 .public_key(key.public_key()).serial_number(x509.random_serial_number())
                 .not_valid_before(now - timedelta(minutes=5)).not_valid_after(now + timedelta(days=1))
                 .add_extension(x509.BasicConstraints(ca=ca, path_length=None), critical=True))
        return built.sign(signer, hashes.SHA256()).public_bytes(pem)

    def private(key: Any) -> bytes:
        return key.private_bytes(pem, serialization.PrivateFormat.PKCS8, serialization.NoEncryption())

    ca_key, client_key = ec.generate_private_key(ec.SECP256R1()), ec.generate_private_key(ec.SECP256R1())
    public = SCHEDULER_KEY.public_key().public_bytes(pem, serialization.PublicFormat.SubjectPublicKeyInfo)
    files = {"ca.pem": cert("golden-ca", ca_key, "golden-ca", ca_key, True), "client.key": private(client_key),
             "client.pem": cert("golden-worker", client_key, "golden-ca", ca_key, False),
             "proof.pem": private(PROOF_KEY), "sched.pem": public,
             "trust.json": json.dumps({"schemaVersion": "cap-adapter-trust/v1", "tenantId": TENANT,
                                       "agentId": "agent-golden", "schedulerId": SCHEDULER_ID, "proofKeyId": "proof-1",
                                       "proofKeyFile": "proof.pem", "schedulerTrustKeys": {"sched-1": "sched.pem"},
                                       "schedulerSigningKeys": {"sched-1": "sched.pem"}}).encode(),
             "config.json": json.dumps({"schemaVersion": "cap-adapter-config/v1", "topic": TOPIC, "workerId": WORKER_ID,
                                        "nats": {"url": url, "caFile": "ca.pem", "certFile": "client.pem",
                                                 "keyFile": "client.key"},
                                        "trustFile": "trust.json", "resources": {"bucket": "cap-golden"},
                                        "limits": {"maxInputBytes": 65536, "maxOutputBytes": 65536, "maxArtifacts": 8,
                                                   "maxArtifactBytes": 65536, "maxDeadlineMs": 30000},
                                        "workloads": WORKLOADS}).encode()}
    for name, data in files.items():
        (directory / name).write_bytes(data)
    return directory / "config.json"


def smoke() -> int:
    module = Path(adapter.__file__).resolve()
    assert REPO not in module.parents, "cap_adapter_langgraph must come from the installed wheel: %s" % module
    failures = 0
    for case in golden_cases():
        view = compare(case, asyncio.run(run_case(case)))
        failures += view["got"] != view["want"]
        print("%s: %s" % (case["id"], "ok" if view["got"] == view["want"] else "MISMATCH %r" % view))
    with tempfile.TemporaryDirectory() as tmp:
        script = Path(sys.executable).parent / ("cap-adapter-langgraph" + (".exe" if os.name == "nt" else ""))
        env = dict(os.environ, PYTHONPATH=os.pathsep.join([str(Path(__file__).parent), str(TRAP.parent)]))
        check = subprocess.run([str(script), "--config", str(write_operator_files(Path(tmp))), "--check"],
                               capture_output=True, text=True, env=env, cwd=tmp, timeout=120)
    print("check: exit %d %s" % (check.returncode, (check.stdout + check.stderr).strip()))
    if failures or check.returncode != 0:
        return 1
    print("SMOKE OK %d/%d" % (len(golden_cases()), len(golden_cases())))
    return 0


if __name__ == "__main__":
    sys.exit(smoke())
