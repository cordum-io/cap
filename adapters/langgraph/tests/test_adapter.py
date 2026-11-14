"""cap-adapter-langgraph: goldens, packet-field safety, tracing, deadline/cancel, config and CLI."""
import asyncio
import importlib.metadata
import json
import os
import socket
import ssl
import subprocess
import sys
import threading
from pathlib import Path

import cap
import golden_graphs as gg
import langsmith.utils
import pytest
from cap.production_replay import InMemoryReplayStore
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

import cap_adapter_langgraph as adapter
from cap_adapter_langgraph import ConfigError

CASES = {case["id"]: case for case in gg.golden_cases()}
TRACING = ("LANGSMITH_TRACING_V2", "LANGSMITH_TRACING", "LANGCHAIN_TRACING_V2", "LANGCHAIN_TRACING")


@pytest.fixture(autouse=True, scope="module")
def trap_clean():
    yield
    assert socket._cap_nettrap_blocked == []


def case(workload, data, **extra):
    return dict({"id": "custom", "workload": workload, "input": data}, **extra)


def test_network_trap_is_active():
    blocked = socket._cap_nettrap_blocked
    start = len(blocked)
    socket.getaddrinfo(None, 0)
    socket.getaddrinfo("LocalHost", 0)
    socket.getaddrinfo("127.0.0.1", 0)
    with pytest.raises(OSError):
        socket.getaddrinfo("example.invalid", 80)
    with pytest.raises(OSError):
        socket.create_connection(("192.0.2.1", 9), timeout=0.1)
    with socket.socket(socket.AF_INET6) as sock, pytest.raises(OSError):
        sock.connect(("::ffff:192.0.2.1", 9))
    assert blocked[start:] == [("getaddrinfo", "example.invalid", 80), ("create_connection", "192.0.2.1", 9),
                               ("connect", "::ffff:192.0.2.1", 9)]
    del blocked[start:]


def test_golden_file_has_nine_cases():
    assert list(CASES) == ["success-echo", "multi-step-progress", "artifact-emitted", "framework-failure-redacted",
                           "cancel-mid-run", "deadline-exceeded", "output-over-bound", "input-rejected",
                           "unregistered-workload"]


@pytest.mark.parametrize("case_id", list(CASES))
def test_golden_case(case_id):
    view = gg.compare(CASES[case_id], asyncio.run(gg.run_case(CASES[case_id])))
    assert view["got"] == view["want"]


def test_packet_fields_never_select_code():
    hostile = ["os:system", "../../etc/passwd", "http://203.0.113.9/x.py", "rm -rf /"]

    def edit(req):
        for i, value in enumerate(hostile):
            req.env["K%d" % i], req.labels["K%d" % i], req.meta.labels["K%d" % i] = value, value, value
        req.adapter_id = hostile[0]

    async def scenario():
        plain = await gg.run_case(case("echo", {"text": "hello"}))
        modules = set(sys.modules)
        run = await gg.run_case(case("echo", {"text": "hello"}), edit=edit)
        assert (run.status, run.output, dict(gg.COUNTS)) == (plain.status, {"text": "hello"}, {"echo": 1})
        assert set(sys.modules) - modules == set()
        refused = await gg.run_case(case("os:system", {"text": "hello"}), edit=edit)
        assert (refused.status, refused.error_code) == ("JOB_STATUS_FAILED", "ERROR_CODE_JOB_NOT_FOUND")
        assert sum(gg.COUNTS.values()) == 0

    asyncio.run(scenario())


def refused(workload, data, code):
    run = asyncio.run(gg.run_case(case(workload, data)))
    assert (run.status, run.error_code, run.progress, sum(gg.COUNTS.values())) == ("JOB_STATUS_FAILED", code, [], 0)


def test_unregistered_capability_zero_invocations():
    refused("not-registered", {"text": "hello"}, "ERROR_CODE_JOB_NOT_FOUND")


def test_oversize_input_zero_invocations():
    refused("echo", {"text": "x" * gg.LIMITS.max_input_bytes}, "ERROR_CODE_JOB_RESOURCE_EXHAUSTED")


def test_invalid_input_zero_invocations():
    refused("two-step-upper", {"text": 42}, "ERROR_CODE_JOB_INVALID_INPUT")
    refused("two-step-upper", {"text": "hello", "extra": 1}, "ERROR_CODE_JOB_INVALID_INPUT")


def test_tracing_env_forced_off(monkeypatch):
    code = ("import asyncio, json, os, golden_graphs as gg; "
            "run = asyncio.run(gg.run_case({'id': 'p', 'workload': 'tracing-probe', 'input': {'text': 'x'}})); "
            "print(json.dumps([os.environ[k] for k in %r] + [run.output]))" % (TRACING,))
    env = dict(os.environ, **{name: "true" for name in TRACING})
    env["PYTHONPATH"] = os.pathsep.join([str(Path(gg.__file__).parent), str(gg.TRAP.parent)])
    out = subprocess.run([sys.executable, "-c", code], env=env, capture_output=True, text=True, timeout=120)
    assert (out.returncode, json.loads(out.stdout)) == (0, ["false"] * 4 + [{"tracing": False}]), out.stderr
    monkeypatch.setenv("LANGSMITH_TRACING_V2", "true")
    langsmith.utils.get_env_var.cache_clear()
    try:
        assert asyncio.run(gg.run_case(case("tracing-probe", {"text": "x"}))).output == {"tracing": False}
    finally:
        monkeypatch.setenv("LANGSMITH_TRACING_V2", "false")
        langsmith.utils.get_env_var.cache_clear()


def stopped(golden, limits=None, edit=lambda req: None):
    async def scenario():
        await gg.run_case(CASES["success-echo"])
        tasks, threads = set(asyncio.all_tasks()), set(threading.enumerate())
        gg.CANCEL_SEEN.clear()
        run = await gg.run_case(golden, limits=limits, edit=edit)
        await asyncio.sleep(0)
        assert set(asyncio.all_tasks()) - tasks == set()
        assert set(threading.enumerate()) - threads == set()
        assert gg.CANCEL_SEEN == [True]
        return run

    return asyncio.run(scenario())


def test_deadline_stops_blocking_graph():
    run = stopped(CASES["deadline-exceeded"])
    assert (run.status, run.error_code, run.error_string) == ("JOB_STATUS_FAILED", "ERROR_CODE_JOB_TIMEOUT", "TIMEOUT")
    assert 0.25 <= run.elapsed <= 0.25 + 2


def test_packet_deadline_below_operator_bound_wins():
    def edit(req):
        req.budget.deadline_ms = 200

    run = stopped(case("wait-for-cancel", {"text": "x"}), limits=gg.LIMITS, edit=edit)
    assert (run.status, run.error_string) == ("JOB_STATUS_FAILED", "TIMEOUT")
    assert 0.2 <= run.elapsed <= 0.2 + 2


def test_cancel_stops_blocking_graph():
    run = stopped(CASES["cancel-mid-run"])
    assert (run.status, run.progress) == ("JOB_STATUS_CANCELLED", ["started"])
    assert run.after_cancel <= 2


@pytest.fixture
def config_path(tmp_path):
    return gg.write_operator_files(tmp_path)


def rewrite(path, edit):
    data = json.loads(path.read_text())
    edit(data)
    path.write_text(json.dumps(data))
    return path


@pytest.mark.parametrize("edit", [
    lambda c: c.update(extra=1), lambda c: c["nats"].update(extra=1), lambda c: c["limits"].pop("maxArtifacts"),
    lambda c: c["nats"].update(url="nats://canary-secret@127.0.0.1:4222"), lambda c: c["nats"].update(caFile="no.pem"),
    lambda c: c["limits"].update(maxDeadlineMs=True), lambda c: c["limits"].update(maxInputBytes=(64 << 20) + 1),
    lambda c: c["resources"].update(bucket="bad bucket"), lambda c: c["workloads"].update(BAD={}),
    lambda c: c["workloads"]["echo"].update(factory="golden_graphs.echo_graph"),
    lambda c: c["workloads"]["echo"].update(factory="golden_graphs:TOPIC"),
    lambda c: c["workloads"]["echo"].update(inputModel="golden_graphs:echo_graph"),
    lambda c: c["workloads"]["echo"].update(factory="golden_graphs:missing"),
])
def test_load_config_rejects(config_path, edit):
    with pytest.raises(ConfigError) as err:
        adapter.load_config(rewrite(config_path, edit))
    assert "canary-secret" not in str(err.value)


def test_load_config_rejects_duplicate_missing_and_too_large(config_path, tmp_path):
    text = config_path.read_text()
    for name, data in [("dup.json", '{"topic": "a", ' + text[1:]), ("big.json", " " * (1 << 20) + text)]:
        (tmp_path / name).write_text(data)
        with pytest.raises(ConfigError):
            adapter.load_config(tmp_path / name)
    with pytest.raises(ConfigError):
        adapter.load_config(tmp_path / "missing.json")


def test_relative_paths_resolve_against_config_dir(config_path, tmp_path, monkeypatch):
    elsewhere = tmp_path / "elsewhere"
    elsewhere.mkdir()
    monkeypatch.chdir(elsewhere)
    config = adapter.load_config(Path(os.path.relpath(config_path, elsewhere)))
    assert [Path(p) for p in (config.ca_file, config.trust_file)] == [config_path.parent.resolve() / "ca.pem",
                                                                     config_path.parent.resolve() / "trust.json"]
    assert adapter.load_trust(config.trust_file).scheduler_id == gg.SCHEDULER_ID
    assert config.limits == gg.LIMITS and config.bucket == "cap-golden" and list(config.workloads) == list(gg.WORKLOADS)


@pytest.mark.parametrize("edit", [lambda t: t.pop("agentId"), lambda t: t.update(extra=1),
                                  lambda t: t["schedulerTrustKeys"].update(k2="none.pem"),
                                  lambda t: t.update(schedulerSigningKeys={}), "p384"])
def test_load_trust_rejects(config_path, edit):
    trust = config_path.parent / "trust.json"
    if edit == "p384":
        key = ec.generate_private_key(ec.SECP384R1())
        (config_path.parent / "proof.pem").write_bytes(key.private_bytes(
            serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()))
    else:
        rewrite(trust, edit)
    with pytest.raises(ConfigError):
        adapter.load_trust(trust)


def test_agent_kwargs(config_path):
    config = adapter.load_config(config_path)
    kwargs = adapter.agent_kwargs(config, adapter.load_trust(config.trust_file))
    wt, pt = kwargs.pop("worker_trust"), kwargs.pop("production_trust")
    assert isinstance(kwargs.pop("replay_store"), InMemoryReplayStore)
    assert kwargs == {"nats_url": "tls://127.0.0.1:4222", "worker_id": gg.WORKER_ID, "worker_trust_mode": "enforce",
                      "result_backend_id": "nats-kv", "max_context_bytes": 65536, "max_result_bytes": 65536,
                      "max_artifacts": 8, "max_artifact_bytes": 65536, "max_deadline": 30.0, "retries": 0,
                      "max_parallel": 1}
    assert (wt.worker_id, wt.expected_agent_id, wt.tenant_id, wt.audience, wt.proof_key_id, wt.expected_scheduler_id,
            wt.sdk_version) == (gg.WORKER_ID, "agent-golden", gg.TENANT, cap.WORKER_HANDSHAKE_AUDIENCE, "proof-1",
                                gg.SCHEDULER_ID, importlib.metadata.version("cap-sdk-python"))
    assert wt.proof_private_key.private_numbers() == gg.PROOF_KEY.private_numbers()
    scheduler = gg.SCHEDULER_KEY.public_key().public_numbers()
    assert {k: v.public_numbers() for k, v in wt.scheduler_public_keys.items()} == {"sched-1": scheduler}
    assert (pt.audience, pt.tenant, pt.sender) == (gg.TOPIC, gg.TENANT, gg.SCHEDULER_ID)
    assert {k: v.public_numbers() for k, v in pt.public_keys.items()} == {"sched-1": scheduler}
    backend = cap.InMemoryResourceBackend()
    registry = cap.ResourceRegistry([cap.ResourceBackendRegistration("nats-kv", 1024, backend, backend)])
    cap.Agent(**adapter.agent_kwargs(config, adapter.load_trust(config.trust_file)), resource_registry=registry)


def test_tls_context_verifies_server_and_loads_client_chain(config_path):
    context = adapter.tls_context(adapter.load_config(config_path))
    assert (context.verify_mode, context.check_hostname, len(context.get_ca_certs())) == (ssl.CERT_REQUIRED, True, 1)


def test_main_check_and_bad_config(config_path, capsys):
    assert adapter.main(["--config", str(config_path), "--check"]) == 0
    assert capsys.readouterr().out.strip() == "CHECK OK %d workloads" % len(gg.WORKLOADS)
    rewrite(config_path, lambda c: c.update(topic=""))
    assert adapter.main(["--config", str(config_path), "--check"]) == 2
    assert capsys.readouterr().err.startswith("config error: ")
