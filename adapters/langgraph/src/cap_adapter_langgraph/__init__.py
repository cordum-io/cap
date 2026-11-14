"""CAP worker adapter for operator-registered LangGraph graphs (Apache-2.0, no Cordum dependency).

A job selects its workload only by JobMetadata.capability; bounds, deadline and cancel stay in cap-sdk-python.
"""
import os

# Tracing is forced off before langsmith/langgraph are imported (no phone-home).
for _name in ("LANGSMITH_TRACING_V2", "LANGSMITH_TRACING", "LANGCHAIN_TRACING_V2", "LANGCHAIN_TRACING"):
    os.environ[_name] = "false"

import argparse  # noqa: E402
import asyncio  # noqa: E402
import contextlib  # noqa: E402
import functools  # noqa: E402
import importlib.metadata  # noqa: E402
import json  # noqa: E402
import logging  # noqa: E402
import re  # noqa: E402
import signal  # noqa: E402
import ssl  # noqa: E402
import sys  # noqa: E402
from dataclasses import dataclass  # noqa: E402
from pathlib import Path  # noqa: E402
from types import MappingProxyType  # noqa: E402
from typing import Any, Callable, Dict, Mapping, Optional, Sequence, Type  # noqa: E402

import cap  # noqa: E402
import nats  # noqa: E402
from cap import Context, JobFailure  # noqa: E402
from cap.pb.cordum.agent.v1 import job_pb2  # noqa: E402
from cap.production_replay import InMemoryReplayStore  # noqa: E402
from cap.production_signing import ProductionTrust  # noqa: E402
from cryptography.hazmat.primitives import serialization  # noqa: E402
from cryptography.hazmat.primitives.asymmetric import ec  # noqa: E402
from langsmith.run_helpers import tracing_context  # noqa: E402
from pydantic import BaseModel  # noqa: E402

__all__ = ["AdapterConfig", "AdapterTrust", "ConfigError", "Limits", "Workload", "agent_kwargs", "agent_options",
           "load_config", "load_trust", "load_workloads", "main", "make_handler", "serve", "tls_context"]

LOG = logging.getLogger("cap_adapter_langgraph")
_KEY = re.compile(r"[a-z0-9][a-z0-9._-]{0,63}")
_REF = re.compile(r"[A-Za-z_][A-Za-z0-9_.]*:[A-Za-z_][A-Za-z0-9_]*")
FRAMEWORK_ERROR = "FRAMEWORK_ERROR"
_MAX_FILE_BYTES = 1 << 20
_BUCKET = re.compile(r"[A-Za-z0-9_-]{1,64}")
_LIMITS = {"maxInputBytes": 64 << 20, "maxOutputBytes": 64 << 20, "maxArtifacts": 64,
           "maxArtifactBytes": 64 << 20, "maxDeadlineMs": 86_400_000}
_CONFIG_KEYS = ("schemaVersion", "topic", "workerId", "nats", "trustFile", "resources", "limits", "workloads")
_TRUST_KEYS = ("schemaVersion", "tenantId", "agentId", "schedulerId", "proofKeyId", "proofKeyFile",
               "schedulerTrustKeys", "schedulerSigningKeys")


class ConfigError(ValueError):
    """Invalid operator configuration; the message names the key, never file contents."""


@dataclass(frozen=True)
class Limits:
    max_input_bytes: int
    max_output_bytes: int
    max_artifacts: int
    max_artifact_bytes: int
    max_deadline_ms: int


@dataclass(frozen=True)
class Workload:
    factory: Callable[[], Any]
    input_model: Type[BaseModel]


def _import(ref: Any, where: str) -> Any:
    if not isinstance(ref, str) or _REF.fullmatch(ref) is None:
        raise ConfigError(where + ": must be a module:attribute reference")
    module, attr = ref.split(":")
    try:
        return getattr(importlib.import_module(module), attr)
    except Exception:
        raise ConfigError(where + ": cannot be imported") from None


def load_workloads(mapping: Any) -> Mapping[str, Workload]:
    """Import every registered factory and input model now, so a bad config fails at startup."""
    if not isinstance(mapping, dict) or not mapping:
        raise ConfigError("workloads: must be a non-empty object")
    loaded: Dict[str, Workload] = {}
    for key, entry in mapping.items():
        if not isinstance(key, str) or _KEY.fullmatch(key) is None:
            raise ConfigError("workloads: key must match [a-z0-9][a-z0-9._-]{0,63}")
        where = "workloads." + key
        if not isinstance(entry, dict) or set(entry) != {"factory", "inputModel"}:
            raise ConfigError(where + ": must have exactly factory and inputModel")
        factory = _import(entry["factory"], where + ".factory")
        model = _import(entry["inputModel"], where + ".inputModel")
        if not callable(factory):
            raise ConfigError(where + ".factory: must be callable")
        if not (isinstance(model, type) and issubclass(model, BaseModel)):
            raise ConfigError(where + ".inputModel: must be a pydantic BaseModel subclass")
        loaded[key] = Workload(factory, model)
    return MappingProxyType(loaded)


def _framework_error(key: str, exc: BaseException) -> JobFailure:
    LOG.warning("workload %s failed: %s", key, type(exc).__name__)
    return JobFailure(FRAMEWORK_ERROR)


def _artifact(value: Any) -> Optional[tuple]:
    if not isinstance(value, dict) or set(value) != {"name", "mediaType", "content"}:
        return None
    content = value["content"]
    if isinstance(content, str):
        content = content.encode("utf-8")
    return (value["name"], value["mediaType"], content) if isinstance(content, bytes) else None


async def _emit(ctx: Context, key: str, event: Any) -> None:
    """Map one custom stream event: cap.progress and cap.artifact only; other events are ignored."""
    if not isinstance(event, dict) or not any(isinstance(k, str) and k.startswith("cap.") for k in event):
        return
    marker = event["cap.progress"] if set(event) == {"cap.progress"} else None
    artifact = _artifact(event["cap.artifact"]) if set(event) == {"cap.artifact"} else None
    if not isinstance(marker, str) and artifact is None:
        raise _framework_error(key, ValueError())
    try:
        if isinstance(marker, str):
            await ctx.report_progress(marker)
        else:
            await ctx.attach_artifact(*artifact)
    except (ValueError, TypeError) as exc:
        raise _framework_error(key, exc) from None


async def _run_graph(ctx: Context, key: str, workload: Workload, state: Dict[str, Any]) -> Any:
    final: Any = None
    with tracing_context(enabled=False):
        try:
            graph = workload.factory()
            # output_keys as in ainvoke: the result holds only the graph's output_schema keys.
            stream = graph.astream(state, stream_mode=["custom", "values"], output_keys=graph.output_channels)
        except JobFailure:
            raise
        except Exception as exc:
            raise _framework_error(key, exc) from None
        async with contextlib.aclosing(stream) as events:
            while True:
                try:
                    mode, chunk = await anext(events)
                except StopAsyncIteration:
                    break
                except JobFailure:
                    raise
                except Exception as exc:
                    raise _framework_error(key, exc) from None
                if mode == "values":
                    final = chunk
                else:
                    await _emit(ctx, key, chunk)
    return final if final is not None else {}


def make_handler(workloads: Mapping[str, Workload]) -> Callable[[Context, Any], Any]:
    """The cap.Agent job handler: exact-key workload lookup, input validation, then the graph run."""
    table = dict(workloads)

    async def handler(ctx: Context, data: Any) -> Any:
        key = ctx.job.meta.capability
        workload = table.get(key)
        if workload is None:
            raise JobFailure(job_pb2.ERROR_CODE_JOB_NOT_FOUND)
        try:
            state = workload.input_model.model_validate(data).model_dump(mode="json")
        except Exception:
            raise JobFailure(job_pb2.ERROR_CODE_JOB_INVALID_INPUT) from None
        return await _run_graph(ctx, key, workload, state)

    return handler


def agent_options(limits: Limits) -> Dict[str, Any]:
    """cap.Agent bounds for the operator limits; the SDK enforces them."""
    return {"max_context_bytes": limits.max_input_bytes, "max_result_bytes": limits.max_output_bytes,
            "max_artifacts": limits.max_artifacts, "max_artifact_bytes": limits.max_artifact_bytes,
            "max_deadline": limits.max_deadline_ms / 1000, "retries": 0, "max_parallel": 1}


@dataclass(frozen=True)
class AdapterConfig:
    """A loaded cap-adapter-config/v1 file; paths are absolute."""
    topic: str
    worker_id: str
    nats_url: str
    ca_file: str
    cert_file: str
    key_file: str
    trust_file: str
    bucket: str
    limits: Limits
    workloads: Mapping[str, Workload]


@dataclass(frozen=True)
class AdapterTrust:
    """A loaded cap-adapter-trust/v1 file with its P-256 keys."""
    tenant_id: str
    agent_id: str
    scheduler_id: str
    proof_key_id: str
    proof_private_key: ec.EllipticCurvePrivateKey
    scheduler_trust_keys: Mapping[str, ec.EllipticCurvePublicKey]
    scheduler_signing_keys: Mapping[str, ec.EllipticCurvePublicKey]


def _read(path: Path, where: str) -> bytes:
    try:
        with open(path, "rb") as handle:
            raw = handle.read(_MAX_FILE_BYTES + 1)
    except OSError:
        raise ConfigError(where + ": cannot be read") from None
    if len(raw) > _MAX_FILE_BYTES:
        raise ConfigError(where + ": larger than 1 MiB")
    return raw


def _unique(pairs: Sequence[tuple]) -> Dict[str, Any]:
    if len({key for key, _ in pairs}) != len(pairs):
        raise ConfigError("duplicate key")
    return dict(pairs)


def _fields(value: Any, where: str, keys: Sequence[str]) -> Dict[str, Any]:
    if not isinstance(value, dict) or set(value) != set(keys):
        raise ConfigError(where + ": must be an object with exactly the keys " + ", ".join(sorted(keys)))
    return value


def _document(path: Path, where: str, schema: str, keys: Sequence[str]) -> Dict[str, Any]:
    raw = _read(path, where)
    try:
        data = json.loads(raw.decode("utf-8"), object_pairs_hook=_unique)
    except ConfigError:
        raise ConfigError(where + ": duplicate key") from None
    except ValueError:
        raise ConfigError(where + ": not valid UTF-8 JSON") from None
    _fields(data, where, keys)
    if data["schemaVersion"] != schema:
        raise ConfigError(where + ".schemaVersion: must be " + schema)
    return data


def _text(value: Any, where: str) -> str:
    if not isinstance(value, str) or not value or any(c.isspace() or not c.isprintable() for c in value):
        raise ConfigError(where + ": must be a non-empty string without spaces")
    return value


def _file(base: Path, value: Any, where: str) -> str:
    if not isinstance(value, str) or not value:
        raise ConfigError(where + ": must be a file path")
    path = (base / value).resolve()
    if not path.is_file():
        raise ConfigError(where + ": file does not exist")
    return str(path)


def _limit(value: Any, where: str, bound: int) -> int:
    if type(value) is not int or not 0 < value <= bound:
        raise ConfigError("%s: must be an integer from 1 to %d" % (where, bound))
    return value


def load_config(path: Any) -> AdapterConfig:
    """Strictly read cap-adapter-config/v1; relative paths resolve against the file's directory."""
    path = Path(path)
    data = _document(path, "config", "cap-adapter-config/v1", _CONFIG_KEYS)
    base = path.resolve().parent
    server = _fields(data["nats"], "config.nats", ("url", "caFile", "certFile", "keyFile"))
    if not isinstance(server["url"], str) or not server["url"].startswith("tls://") or len(server["url"]) < 7:
        raise ConfigError("config.nats.url: must be a tls:// URL")
    bucket = _fields(data["resources"], "config.resources", ("bucket",))["bucket"]
    if not isinstance(bucket, str) or _BUCKET.fullmatch(bucket) is None:
        raise ConfigError("config.resources.bucket: must match [A-Za-z0-9_-]{1,64}")
    limits = _fields(data["limits"], "config.limits", tuple(_LIMITS))
    return AdapterConfig(
        topic=_text(data["topic"], "config.topic"), worker_id=_text(data["workerId"], "config.workerId"),
        nats_url=server["url"], ca_file=_file(base, server["caFile"], "config.nats.caFile"),
        cert_file=_file(base, server["certFile"], "config.nats.certFile"),
        key_file=_file(base, server["keyFile"], "config.nats.keyFile"),
        trust_file=_file(base, data["trustFile"], "config.trustFile"), bucket=bucket,
        limits=Limits(*(_limit(limits[k], "config.limits." + k, bound) for k, bound in _LIMITS.items())),
        workloads=load_workloads(data["workloads"]))


def _p256(path: str, where: str, private: bool) -> Any:
    raw = _read(Path(path), where)
    try:
        key = (serialization.load_pem_private_key(raw, password=None) if private
               else serialization.load_pem_public_key(raw))
    except Exception:
        key = None
    expected = ec.EllipticCurvePrivateKey if private else ec.EllipticCurvePublicKey
    if not isinstance(key, expected) or not isinstance(key.curve, ec.SECP256R1):
        raise ConfigError("%s: must be an unencrypted P-256 %s key PEM" % (where, "private" if private else "public"))
    return key


def _public_keys(base: Path, value: Any, where: str) -> Mapping[str, ec.EllipticCurvePublicKey]:
    if not isinstance(value, dict) or not value:
        raise ConfigError(where + ": must be a non-empty object of key id to PEM path")
    return MappingProxyType({_text(key, where): _p256(_file(base, path, where + "." + key), where + "." + key, False)
                             for key, path in value.items()})


def load_trust(path: Any) -> AdapterTrust:
    """Strictly read cap-adapter-trust/v1; key paths resolve against the trust file's directory."""
    path = Path(path)
    data = _document(path, "trust", "cap-adapter-trust/v1", _TRUST_KEYS)
    base = path.resolve().parent
    return AdapterTrust(
        tenant_id=_text(data["tenantId"], "trust.tenantId"), agent_id=_text(data["agentId"], "trust.agentId"),
        scheduler_id=_text(data["schedulerId"], "trust.schedulerId"),
        proof_key_id=_text(data["proofKeyId"], "trust.proofKeyId"),
        proof_private_key=_p256(_file(base, data["proofKeyFile"], "trust.proofKeyFile"), "trust.proofKeyFile", True),
        scheduler_trust_keys=_public_keys(base, data["schedulerTrustKeys"], "trust.schedulerTrustKeys"),
        scheduler_signing_keys=_public_keys(base, data["schedulerSigningKeys"], "trust.schedulerSigningKeys"))


def agent_kwargs(config: AdapterConfig, trust: AdapterTrust) -> Dict[str, Any]:
    """cap.Agent keyword arguments for the production path: worker trust ENFORCE and CAP-PRODUCTION admission."""
    worker_trust = cap.WorkerTrustConfig(
        config.worker_id, trust.agent_id, trust.tenant_id, cap.WORKER_HANDSHAKE_AUDIENCE, trust.proof_key_id,
        trust.proof_private_key, trust.scheduler_id, trust.scheduler_trust_keys,
        importlib.metadata.version("cap-sdk-python"))
    try:
        cap.validate_worker_trust_config(worker_trust)
    except cap.WorkerTrustConfigError:
        raise ConfigError("trust: worker trust configuration is invalid") from None
    return {**agent_options(config.limits), "nats_url": config.nats_url, "worker_id": config.worker_id,
            "worker_trust_mode": "enforce", "worker_trust": worker_trust,
            "production_trust": ProductionTrust(audience=config.topic, public_keys=trust.scheduler_signing_keys,
                                                tenant=trust.tenant_id, sender=trust.scheduler_id),
            "replay_store": InMemoryReplayStore(), "result_backend_id": cap.NATS_KV_RESOLVER_ID}


def tls_context(config: AdapterConfig) -> ssl.SSLContext:
    """Client TLS for NATS: the operator CA, hostname checking on, and the client certificate chain."""
    try:
        context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH, cafile=config.ca_file)
        context.load_cert_chain(config.cert_file, config.key_file)
    except (OSError, ValueError):
        raise ConfigError("config.nats: TLS CA, certificate or key cannot be loaded") from None
    return context


async def serve(config: AdapterConfig, trust: AdapterTrust) -> None:
    """Run the worker on config.topic until cancelled (the real-NATS E2E is task-dbed002a's scope)."""
    connect = functools.partial(nats.connect, tls=tls_context(config))
    resources = await connect(servers=config.nats_url, name=config.worker_id + "-resources")
    try:
        backend = await cap.NatsKvResourceBackend.open(await resources.jetstream().key_value(config.bucket))
        agent = cap.Agent(**agent_kwargs(config, trust), connect_fn=connect,
                          resource_registry=cap.ResourceRegistry([backend.registration()]))
        agent.job(config.topic)(make_handler(config.workloads))
        await agent.run()
    finally:
        try:
            await resources.drain()
        except Exception as exc:
            LOG.warning("resources connection drain failed: %s", type(exc).__name__)


def main(argv: Optional[Sequence[str]] = None) -> int:
    """Console script: 0 on a check or clean stop, 2 on a config error, 1 on any other failure."""
    parser = argparse.ArgumentParser(prog="cap-adapter-langgraph", description="CAP worker for LangGraph graphs")
    parser.add_argument("--config", required=True, metavar="PATH", help="cap-adapter-config/v1 JSON file")
    parser.add_argument("--check", action="store_true", help="validate config, trust, TLS files and workloads only")
    args = parser.parse_args(argv)
    logging.basicConfig(level=logging.INFO, stream=sys.stderr)
    try:
        config = load_config(args.config)
        trust = load_trust(config.trust_file)
        agent_kwargs(config, trust)
        tls_context(config)
    except ConfigError as exc:
        print("config error: %s" % exc, file=sys.stderr)
        return 2
    if args.check:
        print("CHECK OK %d workloads" % len(config.workloads))
        return 0
    signal.signal(signal.SIGTERM, signal.default_int_handler)  # SIGTERM stops like Ctrl-C: serve is cancelled
    try:
        asyncio.run(serve(config, trust))
    except KeyboardInterrupt:
        return 0
    except Exception as exc:
        LOG.error("adapter stopped: %s", type(exc).__name__)
        return 1
    return 0
