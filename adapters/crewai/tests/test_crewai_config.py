"""Operator configuration and startup boundaries; no external LLM or NATS server."""
import asyncio
import importlib.metadata
import json
import os
import signal
import ssl
import sys
import tempfile
from pathlib import Path

import cap
import golden_crews as gg
import nats
import pytest
from cap.production_replay import InMemoryReplayStore
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

import cap_adapter_crewai as adapter
from cap_adapter_crewai import _proc

FIXTURES = Path(__file__).parent / "fixtures"
TRAP = Path(__file__).resolve().parents[3] / "testdata" / "adapters" / "nettrap"


@pytest.fixture(autouse=True)
def child_env(monkeypatch, tmp_path):
    monkeypatch.setenv("PYTHONPATH", os.pathsep.join([str(FIXTURES), str(TRAP)]))
    monkeypatch.setattr(tempfile, "tempdir", str(tmp_path))
    return tmp_path


@pytest.fixture
def config_path(tmp_path):
    return gg.write_operator_files(tmp_path)


def rewrite(path, edit):
    data = json.loads(path.read_text())
    edit(data)
    path.write_text(json.dumps(data))
    return path


@pytest.mark.parametrize("edit", [
    pytest.param(lambda c: c.update(extra=1), id="unknown-key"),
    pytest.param(lambda c: c.pop("topic"), id="missing-key"),
    pytest.param(lambda c: c["nats"].update(url="nats://canary-secret@127.0.0.1:4222"), id="non-tls-url"),
    pytest.param(lambda c: c["resources"].update(bucket="bad bucket"), id="bad-bucket"),
    pytest.param(lambda c: c["limits"].update(maxInputBytes=(64 << 20) + 1), id="limit-over-bound"),
    pytest.param(lambda c: c["workloads"]["echo"].update(artifactTasks={"task": "bad name"}), id="bad-artifact-name"),
    pytest.param(lambda c: c["workloads"]["echo"].update(child={
        "graceMs": 60001, "maxStdoutBytes": 1024, "maxStderrBytes": 1024}), id="child-out-of-bounds"),
    pytest.param(lambda c: c["nats"].update(extra=1), id="nested-unknown-key"),
    pytest.param(lambda c: c["limits"].pop("maxArtifacts"), id="nested-missing-key"),
    pytest.param(lambda c: c["nats"].update(caFile="missing.pem"), id="missing-tls-file"),
    pytest.param(lambda c: c["limits"].update(maxDeadlineMs=True), id="boolean-limit"),
    pytest.param(lambda c: c["workloads"].update(BAD={}), id="bad-workload-key"),
    pytest.param(lambda c: c["workloads"]["echo"].update(factory="golden_crews.echo_crew"), id="bad-reference"),
])
def test_load_config_rejects(config_path, edit):
    with pytest.raises(adapter.ConfigError) as err:
        adapter.load_config(rewrite(config_path, edit))
    assert "canary-secret" not in str(err.value)


def test_load_config_rejects_duplicate_missing_and_too_large(config_path, tmp_path):
    original = config_path.read_text()
    for name, data in [("dup.json", '{"topic": "a", ' + original[1:]),
                       ("big.json", " " * (1 << 20) + original)]:
        path = tmp_path / name
        path.write_text(data)
        with pytest.raises(adapter.ConfigError):
            adapter.load_config(path)
    with pytest.raises(adapter.ConfigError):
        adapter.load_config(tmp_path / "missing.json")


def test_relative_paths_resolve_against_config_dir(config_path, tmp_path, monkeypatch):
    elsewhere = tmp_path / "elsewhere"
    elsewhere.mkdir()
    monkeypatch.chdir(elsewhere)
    config = adapter.load_config(Path(os.path.relpath(config_path, elsewhere)))
    assert [Path(p) for p in (config.ca_file, config.trust_file)] == [
        config_path.parent.resolve() / "ca.pem", config_path.parent.resolve() / "trust.json"]
    assert adapter.load_trust(config.trust_file).scheduler_id == gg.SCHEDULER
    assert config.limits == gg.LIMITS and config.bucket == "cap-golden"
    assert list(config.workloads) == list(gg.WORKLOADS)


@pytest.mark.parametrize("edit", [
    pytest.param(lambda t: t.update(extra=1), id="unknown-key"),
    pytest.param(lambda t: t.pop("agentId"), id="missing-key"),
    pytest.param("p384", id="p384"),
    pytest.param(lambda t: t.update(schedulerSigningKeys={}), id="empty-map"),
    pytest.param(lambda t: t["schedulerTrustKeys"].update(k2="missing.pem"), id="missing-key-file"),
])
def test_load_trust_rejects(config_path, edit):
    trust = config_path.parent / "trust.json"
    if edit == "p384":
        key = ec.generate_private_key(ec.SECP384R1())
        (config_path.parent / "proof.pem").write_bytes(key.private_bytes(
            serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()))
    else:
        rewrite(trust, edit)
    with pytest.raises(adapter.ConfigError):
        adapter.load_trust(trust)


def test_agent_kwargs(config_path):
    rewrite(config_path, lambda c: c["workloads"]["echo"].update(child={
        "graceMs": 60000, "maxStdoutBytes": 1024, "maxStderrBytes": 1024}))
    config = adapter.load_config(config_path)
    kwargs = adapter.agent_kwargs(config, adapter.load_trust(config.trust_file))
    wt, pt = kwargs.pop("worker_trust"), kwargs.pop("production_trust")
    assert isinstance(kwargs.pop("replay_store"), InMemoryReplayStore)
    assert kwargs.pop("shutdown_timeout") == 70
    assert kwargs == {"nats_url": "tls://127.0.0.1:4222", "worker_id": gg.WORKER, "worker_trust_mode": "enforce",
                      "result_backend_id": "nats-kv", "max_context_bytes": 65536, "max_result_bytes": 65536,
                      "max_artifacts": 8, "max_artifact_bytes": 65536, "max_deadline": 30.0, "retries": 0,
                      "max_parallel": 1}
    assert (wt.worker_id, wt.expected_agent_id, wt.tenant_id, wt.audience, wt.proof_key_id, wt.expected_scheduler_id,
            wt.sdk_version) == (gg.WORKER, "agent-golden", gg.TENANT, cap.WORKER_HANDSHAKE_AUDIENCE, "proof-1",
                               gg.SCHEDULER, importlib.metadata.version("cap-sdk-python"))
    assert wt.proof_private_key.private_numbers() == gg.PROOF_KEY.private_numbers()
    scheduler = gg.SCHED_KEY.public_key().public_numbers()
    assert {k: v.public_numbers() for k, v in wt.scheduler_public_keys.items()} == {"sched-1": scheduler}
    assert (pt.audience, pt.tenant, pt.sender) == (gg.TOPIC, gg.TENANT, gg.SCHEDULER)
    assert {k: v.public_numbers() for k, v in pt.public_keys.items()} == {"sched-1": scheduler}
    backend = cap.InMemoryResourceBackend()
    registry = cap.ResourceRegistry([cap.ResourceBackendRegistration("nats-kv", 1024, backend, backend)])
    cap.Agent(**adapter.agent_kwargs(config, adapter.load_trust(config.trust_file)), resource_registry=registry)


def test_tls_context_verifies_server_and_loads_client_chain(config_path):
    context = adapter.tls_context(adapter.load_config(config_path))
    assert (context.verify_mode, context.check_hostname, len(context.get_ca_certs())) == (ssl.CERT_REQUIRED, True, 1)
