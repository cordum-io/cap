"""Operator crews, golden harness and python -I wheel smoke; no parent CrewAI import."""
import asyncio
import json
import logging
import math
import os
import sys
import tempfile
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path
from types import SimpleNamespace

REPO = Path(__file__).resolve().parents[4]
TRAP = REPO / "testdata" / "adapters" / "nettrap"
if __name__ == "__main__":
    import runpy
    runpy.run_path(str(TRAP / "sitecustomize.py"))

import cap
from cap.packet_boundary import finalize_packet
from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2
from cap.testing import MockNATS
from cryptography.hazmat.primitives.asymmetric import ec
from pydantic import BaseModel, ConfigDict, Field

import cap_adapter_crewai as adapter

LIMITS = adapter.Limits(65536, 65536, 8, 65536, 30000)
TOPIC, WORKER, TENANT, SCHEDULER = "job.adapter.crewai", "wrk-crew-golden", "tenant-golden", "sched-golden"
SCHED_KEY, PROOF_KEY = ec.generate_private_key(ec.SECP256R1()), ec.generate_private_key(ec.SECP256R1())


class TextInput(BaseModel):
    model_config = ConfigDict(extra="forbid")
    text: str


class RepeatInput(TextInput):
    times: int = Field(ge=1, le=1_000_000)


class CanaryInput(BaseModel):
    model_config = ConfigDict(extra="forbid")
    canary: str


class StepsOut(TextInput):
    steps: list[str]


class SummaryOut(BaseModel):
    summary: str


def _answer(description):
    op, text = description.split(":", 1)
    if op == "raise":
        raise RuntimeError(text)
    if op == "wait":
        time.sleep(120)
    if op == "repeat":
        times, text = text.split(":", 1)
        return {"text": text * int(times)}
    if op == "suffix":
        return {"text": text.upper() + "!", "steps": ["upper", "suffix"]}
    if op == "summary":
        return {"summary": text.upper()}
    return {"text": text} if op == "echo" else text.upper()


def _crew(steps, **options):
    from crewai import Agent, BaseLLM, Crew, Process, Task

    class ScriptedLLM(BaseLLM):
        def call(self, messages, tools=None, callbacks=None, available_functions=None,
                 from_task=None, from_agent=None, response_model=None):
            output = _answer(from_task.description)
            return "Final Answer: " + (output if isinstance(output, str) else json.dumps(output))

        def supports_function_calling(self):
            return False

    class CountedCrew(Crew):
        def kickoff(self, inputs=None):
            # The parent observes this only after real kickoff, not merely factory construction.
            with (Path.cwd().parent / "kickoffs.txt").open("a", encoding="utf-8") as marker:
                marker.write("kickoff\n")
            return super().kickoff(inputs=inputs)

    agent = Agent(role="scripted", goal="Return scripted outputs", backstory="Offline test double",
                  llm=ScriptedLLM(model="scripted-double"), max_retry_limit=0, max_iter=2, verbose=False)
    tasks = [Task(name=name, description=description, expected_output="scripted output", agent=agent,
                  output_json=model) for name, description, model in steps]
    return CountedCrew(agents=[agent], tasks=tasks, process=Process.sequential, tracing=False,
                       verbose=False, **options)


def echo_crew():
    return _crew([(None, "echo:{text}", TextInput)])


def upper_crew():
    return _crew([("upper", "upper:{text}", None), ("suffix", "suffix:{text}", StepsOut)])


def summary_crew():
    return _crew([("summarize", "upper:{text}", None), (None, "summary:{text}", SummaryOut)])


def raise_crew():
    return _crew([("prepare", "prepare:ready", None), (None, "raise:{canary}", None)])


def wait_crew():
    return _crew([("started", "prepare:ready", None), (None, "wait:{text}", None)])


def repeat_crew():
    return _crew([(None, "repeat:{times}:{text}", TextInput)])


def failure_crew():
    raise cap.JobFailure(1001, retryable=True)


def noisy_crew():
    import asyncio
    import subprocess
    import sys

    print("operator noise", flush=True)
    subprocess.run([sys.executable, "-c", "print('tool noise')"], check=True)
    crew = _crew([("named", "echo:{text}", TextInput), (None, "echo:{text}", TextInput)])
    def record(name):
        with (Path.cwd().parent / "callbacks.txt").open("a", encoding="utf-8") as marker:
            marker.write(name + "\n")

    async def task_callback(output):
        await asyncio.sleep(0)
        record("task")

    crew.tasks[0].callback = task_callback
    crew.task_callback = lambda output: record("crew")
    return crew


def traced_crew():
    crew = echo_crew()
    crew.tracing = True
    return crew


def hierarchical_crew():
    from crewai import Process

    crew = echo_crew()
    crew.process = Process.hierarchical
    return crew


WORKLOADS = {key: {"factory": "golden_crews:" + factory, "inputModel": "golden_crews:" + model}
             for key, factory, model in [
                 ("echo", "echo_crew", "TextInput"), ("two-step-upper", "upper_crew", "TextInput"),
                 ("summarize-with-artifact", "summary_crew", "TextInput"),
                 ("raise-in-step", "raise_crew", "CanaryInput"), ("wait-for-cancel", "wait_crew", "TextInput"),
                 ("repeat-text", "repeat_crew", "RepeatInput"), ("failure", "failure_crew", "TextInput")]}
WORKLOADS["summarize-with-artifact"]["artifactTasks"] = {"summarize": "summary"}


def golden_cases():
    return json.loads((REPO / "testdata/adapters/framework-golden-v1.json").read_text(encoding="utf-8"))["cases"]


def signed(body, job_id):
    packet = buspacket_pb2.BusPacket(trace_id="trace-" + job_id, sender_id=SCHEDULER, protocol_version=1)
    packet.created_at.GetCurrentTime()
    body(packet)
    return SimpleNamespace(data=finalize_packet(packet, SCHED_KEY).SerializeToString(deterministic=True))


class _Capture(logging.Handler):
    def __init__(self):
        super().__init__(logging.WARNING)
        self.lines = []

    def emit(self, record):
        self.lines.append(self.format(record) + repr(vars(record)))


def _agent(nats, registry, limits):
    trust = cap.WorkerTrustConfig(WORKER, "agent-golden", TENANT, cap.WORKER_HANDSHAKE_AUDIENCE, "proof-1",
                                  PROOF_KEY, SCHEDULER, {"sched-1": SCHED_KEY.public_key()}, "golden")
    agent = cap.Agent(connect_fn=nats.connect, worker_id=WORKER, heartbeat_interval=60,
                      worker_trust_mode="warn", worker_trust=trust, worker_trust_timeout=0.1,
                      worker_trust_retries=1, resource_registry=registry, result_backend_id="memory",
                      **adapter.agent_options(limits))
    agent.job(TOPIC)(adapter.make_handler(adapter.load_workloads(WORKLOADS)))
    return agent


async def _collect(nats, req, case):
    marks, published, cancel_at = [], [], None
    while True:
        subject, data = await asyncio.wait_for(nats.published.get(), 120)
        published.append(data)
        packet = buspacket_pb2.BusPacket.FromString(data)
        if subject == cap.SUBJECT_PROGRESS:
            marks.append(packet.job_progress.message)
            if case.get("cancel", {}).get("afterProgress") == marks[-1]:
                cancel = job_pb2.JobCancel(job_id=req.job_id, dispatch=req.dispatch, reason="golden", requested_by="golden")
                cancel_at = time.monotonic()
                await nats.subscriptions[cap.SUBJECT_CANCEL](signed(lambda p: p.job_cancel.CopyFrom(cancel), req.job_id))
        if subject == cap.SUBJECT_RESULT:
            return packet.job_result, marks, published, cancel_at


async def run_case(case, *, edit=lambda req: None):
    bounds = case.get("limits", {})
    deadline = bounds.get("deadlineMs", LIMITS.max_deadline_ms)
    if case["id"] == "deadline-exceeded":
        warm = await run_case(golden_cases()[0])
        assert warm.status == "JOB_STATUS_SUCCEEDED", warm
        allowance = math.ceil(warm.elapsed * 2000) + 1000
        print("STARTUP allowanceMs=%d measuredMs=%.0f" % (allowance, warm.elapsed * 1000))
        deadline += allowance
    limits = adapter.Limits(LIMITS.max_input_bytes, bounds.get("maxOutputBytes", LIMITS.max_output_bytes),
                             LIMITS.max_artifacts, LIMITS.max_artifact_bytes, deadline)
    nats, backend = MockNATS(), cap.InMemoryResourceBackend()
    registry = cap.ResourceRegistry([cap.ResourceBackendRegistration("memory", 1 << 22, backend, backend)])
    agent, capture, job_id = _agent(nats, registry, limits), _Capture(), "job-" + case["id"]
    trusted = cap.TrustedResourceContext(TENANT, job_id)
    logging.getLogger().addHandler(capture)
    try:
        await agent.start()
        ref = await registry.externalize("memory", json.dumps(case["input"]).encode(), media_type="application/json",
                                         purpose="job-context", trusted=trusted,
                                         expires_at=datetime.now(timezone.utc) + timedelta(hours=1))
        req = job_pb2.JobRequest(job_id=job_id, topic=TOPIC, context_ref=ref,
                                 dispatch=job_pb2.DispatchIdentity(dispatch_id="d-1", attempt=1, assigned_worker_id=WORKER))
        req.meta.capability = case["workload"]
        edit(req)
        start = time.monotonic()
        await nats.subscriptions[TOPIC](signed(lambda p: p.job_request.CopyFrom(req), job_id))
        result, marks, published, cancel_at = await _collect(nats, req, case)
        done = time.monotonic()
        output = json.loads((await registry.resolve(result.result_ref, trusted)).content) if result.HasField("result_ref") else None
        artifacts = [{"name": r.purpose.split(":", 1)[1], "mediaType": r.media_type,
                      "content": (await registry.resolve(r, trusted)).content.decode()} for r in result.artifact_refs]
    finally:
        await agent.close()
        logging.getLogger().removeHandler(capture)
    code = job_pb2.ErrorCode.DESCRIPTOR.values_by_number.get(result.error_code_enum)
    return SimpleNamespace(status=job_pb2.JobStatus.Name(result.status), error_code=code.name if code else str(result.error_code_enum),
                           error_string=result.error_code, progress=marks, output=output, artifacts=artifacts,
                           published=published, logs=capture.lines, elapsed=done-start,
                           after_cancel=None if cancel_at is None else done-cancel_at)


def compare(case, run):
    want = {k: v for k, v in case["expect"].items() if k != "forbiddenText"}
    want.setdefault("artifacts", [])
    got = {"status": run.status, "progress": run.progress, "artifacts": run.artifacts}
    if "output" in want or run.status == "JOB_STATUS_SUCCEEDED":
        got["output"] = run.output
    for key, value in (("errorCode", run.error_code), ("errorString", run.error_string)):
        if key in want:
            got[key] = value
    for text in case["expect"].get("forbiddenText", []):
        assert not any(text.encode() in blob for blob in run.published + [s.encode() for s in run.logs])
    return got, want


def smoke():
    assert "site-packages" in Path(adapter.__file__).resolve().parts, adapter.__file__
    os.environ["PYTHONPATH"] = os.pathsep.join([str(Path(__file__).parent), str(TRAP)])
    with tempfile.TemporaryDirectory() as tmp:
        previous, tempfile.tempdir = tempfile.tempdir, tmp
        try:
            for case in golden_cases():
                got, want = compare(case, asyncio.run(run_case(case)))
                assert got == want, (case["id"], got, want)
        finally:
            tempfile.tempdir = previous
    print("SMOKE OK 9/9")


if __name__ == "__main__":
    smoke()
