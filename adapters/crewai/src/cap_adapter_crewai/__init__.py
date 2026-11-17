"""CAP handler for operator-registered CrewAI crews; framework imports stay in the child."""
from __future__ import annotations

import importlib.metadata
import json
import re
import ssl
import sys
from dataclasses import dataclass
from pathlib import Path
from types import MappingProxyType
from typing import TYPE_CHECKING, Awaitable, Callable, Mapping, Sequence, cast

if TYPE_CHECKING:
    from cryptography.hazmat.primitives.asymmetric import ec
    from cap import Context
    from . import _proc

_KEY = re.compile(r"[a-z0-9][a-z0-9._-]{0,63}")
_REF = re.compile(r"[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*:[A-Za-z_][A-Za-z0-9_]*")
FRAMEWORK_ERROR = "FRAMEWORK_ERROR"
_MAX_FILE_BYTES = 1 << 20
_BUCKET = re.compile(r"[A-Za-z0-9_-]{1,64}")
_LIMITS = {"maxInputBytes": 64 << 20, "maxOutputBytes": 64 << 20, "maxArtifacts": 64,
           "maxArtifactBytes": 64 << 20, "maxDeadlineMs": 86_400_000}
_CONFIG_KEYS = ("schemaVersion", "topic", "workerId", "nats", "trustFile", "resources", "limits", "workloads")
_TRUST_KEYS = ("schemaVersion", "tenantId", "agentId", "schedulerId", "proofKeyId", "proofKeyFile",
               "schedulerTrustKeys", "schedulerSigningKeys")


class ConfigError(ValueError):
    """Invalid operator configuration; messages name keys, not values."""


@dataclass(frozen=True)
class Limits:
    max_input_bytes: int
    max_output_bytes: int
    max_artifacts: int
    max_artifact_bytes: int
    max_deadline_ms: int


@dataclass(frozen=True)
class Workload:
    factory: str
    input_model: str
    artifact_tasks: Mapping[str, str]
    child: _proc.ChildLimits


def _child_limits(value: object, where: str) -> _proc.ChildLimits:
    from . import _proc

    if not isinstance(value, dict) or set(value) != {"graceMs", "maxStdoutBytes", "maxStderrBytes"}:
        raise ConfigError(where + ": must have graceMs, maxStdoutBytes and maxStderrBytes")
    for key, upper in (("graceMs", 60000), ("maxStdoutBytes", 64 << 20), ("maxStderrBytes", 64 << 20)):
        if type(value[key]) is not int or not 1 <= value[key] <= upper:
            raise ConfigError(where + "." + key + ": out of bounds")
    return _proc.ChildLimits(value["graceMs"], value["maxStdoutBytes"], value["maxStderrBytes"])


def load_workloads(mapping: object) -> Mapping[str, Workload]:
    """Validate and freeze operator references without importing them in the parent."""
    from . import _proc

    if not isinstance(mapping, dict) or not mapping:
        raise ConfigError("workloads: must be a non-empty object")
    loaded: dict[str, Workload] = {}
    for key, entry in mapping.items():
        if not isinstance(key, str) or not _KEY.fullmatch(key):
            raise ConfigError("workloads: invalid key")
        where = "workloads." + key
        if (not isinstance(entry, dict) or not {"factory", "inputModel"} <= set(entry)
                or set(entry) - {"factory", "inputModel", "artifactTasks", "child"}):
            raise ConfigError(where + ": invalid fields")
        for name in ("factory", "inputModel"):
            if not isinstance(entry[name], str) or not _REF.fullmatch(entry[name]):
                raise ConfigError(where + "." + name + ": must be a module:attribute reference")
        artifacts = entry.get("artifactTasks", {})
        if (not isinstance(artifacts, dict)
                or any(not isinstance(k, str) or not k or not isinstance(v, str) or not _KEY.fullmatch(v)
                       for k, v in artifacts.items())
                or len(set(artifacts.values())) != len(artifacts)):
            raise ConfigError(where + ".artifactTasks: invalid or duplicate names")
        child = _child_limits(entry["child"], where + ".child") if "child" in entry else _proc.ChildLimits()
        loaded[key] = Workload(entry["factory"], entry["inputModel"], MappingProxyType(dict(artifacts)), child)
    return MappingProxyType(loaded)


async def _emit(ctx: Context, event: dict[str, object]) -> None:
    from cap import JobFailure
    from . import _proc

    kind = event.get("type")
    fields = {"type", "marker"} if kind == "progress" else {"type", "name", "mediaType", "content"}
    if kind not in ("progress", "artifact") or set(event) != fields or any(
            not isinstance(event[k], str) for k in fields):
        raise JobFailure(_proc.CHILD_CRASHED)
    strings = cast(dict[str, str], event)
    try:
        if kind == "progress":
            await ctx.report_progress(strings["marker"])
        else:
            await ctx.attach_artifact(strings["name"], strings["mediaType"], strings["content"].encode("utf-8"))
    except (ValueError, TypeError):
        raise JobFailure(FRAMEWORK_ERROR) from None


def make_handler(workloads: Mapping[str, Workload]) -> Callable[[Context, object], Awaitable[object]]:
    """Resolve only the capability; cap.Agent enforces input/output bounds and cancellation."""
    from cap import JobFailure
    from cap.pb.cordum.agent.v1 import job_pb2
    from . import _proc

    table = dict(workloads)

    async def handler(ctx: Context, data: object) -> object:
        workload = table.get(ctx.job.meta.capability)
        if workload is None:
            raise JobFailure(job_pb2.ERROR_CODE_JOB_NOT_FOUND)
        job = {"factory": workload.factory, "inputModel": workload.input_model,
               "artifactTasks": dict(workload.artifact_tasks), "input": data}

        async def emit(event: dict[str, object]) -> None:
            await _emit(ctx, event)

        event = await _proc.run_child([sys.executable, "-m", "cap_adapter_crewai._child"], job,
                                      workload.child, emit)
        if event.get("type") == "result" and set(event) == {"type", "output"}:
            return event["output"]
        if event.get("type") == "error" and set(event) == {"type", "code", "retryable"}:
            try:
                failure = JobFailure(event["code"], retryable=event["retryable"])
            except (ValueError, TypeError):
                raise JobFailure(_proc.CHILD_CRASHED) from None
            raise failure
        raise JobFailure(_proc.CHILD_CRASHED)

    return handler


def agent_options(limits: Limits) -> dict[str, object]:
    """Map operator limits to the existing SDK bounds."""
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


def _unique(pairs: Sequence[tuple[str, object]]) -> dict[str, object]:
    if len({key for key, _ in pairs}) != len(pairs):
        raise ConfigError("duplicate key")
    return dict(pairs)


def _fields(value: object, where: str, keys: Sequence[str]) -> dict[str, object]:
    if not isinstance(value, dict) or set(value) != set(keys):
        raise ConfigError(where + ": must be an object with exactly the keys " + ", ".join(sorted(keys)))
    return cast(dict[str, object], value)


def _document(path: Path, where: str, schema: str, keys: Sequence[str]) -> dict[str, object]:
    raw = _read(path, where)
    try:
        data = json.loads(raw.decode("utf-8"), object_pairs_hook=_unique)
    except ConfigError:
        raise ConfigError(where + ": duplicate key") from None
    except ValueError:
        raise ConfigError(where + ": not valid UTF-8 JSON") from None
    data = _fields(data, where, keys)
    if data["schemaVersion"] != schema:
        raise ConfigError(where + ".schemaVersion: must be " + schema)
    return data


def _text(value: object, where: str) -> str:
    if not isinstance(value, str) or not value or any(c.isspace() or not c.isprintable() for c in value):
        raise ConfigError(where + ": must be a non-empty string without spaces")
    return value


def _file(base: Path, value: object, where: str) -> str:
    if not isinstance(value, str) or not value:
        raise ConfigError(where + ": must be a file path")
    path = (base / value).resolve()
    if not path.is_file():
        raise ConfigError(where + ": file does not exist")
    return str(path)


def _limit(value: object, where: str, bound: int) -> int:
    if type(value) is not int or not 0 < value <= bound:
        raise ConfigError("%s: must be an integer from 1 to %d" % (where, bound))
    return value


def load_config(path: str | Path) -> AdapterConfig:
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


def _p256(path: str, where: str, private: bool) -> ec.EllipticCurvePrivateKey | ec.EllipticCurvePublicKey:
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric import ec

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


def _public_keys(base: Path, value: object, where: str) -> Mapping[str, ec.EllipticCurvePublicKey]:
    if not isinstance(value, dict) or not value:
        raise ConfigError(where + ": must be a non-empty object of key id to PEM path")
    return MappingProxyType({_text(key, where): _p256(_file(base, path, where + "." + key), where + "." + key, False)
                             for key, path in value.items()})


def load_trust(path: str | Path) -> AdapterTrust:
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


def agent_kwargs(config: AdapterConfig, trust: AdapterTrust) -> dict[str, object]:
    """cap.Agent keyword arguments for the production path: worker trust ENFORCE and CAP-PRODUCTION admission."""
    import cap
    from cap.production_replay import InMemoryReplayStore
    from cap.production_signing import ProductionTrust

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
            "replay_store": InMemoryReplayStore(), "result_backend_id": cap.NATS_KV_RESOLVER_ID,
            "shutdown_timeout": max(w.child.grace_ms for w in config.workloads.values()) / 1000 + 10}


def tls_context(config: AdapterConfig) -> ssl.SSLContext:
    """Client TLS for NATS: the operator CA, hostname checking on, and the client certificate chain."""
    try:
        context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH, cafile=config.ca_file)
        context.load_cert_chain(config.cert_file, config.key_file)
    except (OSError, ValueError):
        raise ConfigError("config.nats: TLS CA, certificate or key cannot be loaded") from None
    return context
