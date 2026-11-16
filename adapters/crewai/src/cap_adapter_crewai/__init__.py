"""CAP handler for operator-registered CrewAI crews; framework imports stay in the child."""
import re
import sys
from dataclasses import dataclass
from types import MappingProxyType
from typing import Awaitable, Callable, Mapping, cast

from cap import Context, JobFailure
from cap.pb.cordum.agent.v1 import job_pb2

from . import _proc

_KEY = re.compile(r"[a-z0-9][a-z0-9._-]{0,63}")
_REF = re.compile(r"[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*:[A-Za-z_][A-Za-z0-9_]*")
FRAMEWORK_ERROR = "FRAMEWORK_ERROR"


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
    if not isinstance(value, dict) or set(value) != {"graceMs", "maxStdoutBytes", "maxStderrBytes"}:
        raise ConfigError(where + ": must have graceMs, maxStdoutBytes and maxStderrBytes")
    for key, upper in (("graceMs", 60000), ("maxStdoutBytes", 64 << 20), ("maxStderrBytes", 64 << 20)):
        if type(value[key]) is not int or not 1 <= value[key] <= upper:
            raise ConfigError(where + "." + key + ": out of bounds")
    return _proc.ChildLimits(value["graceMs"], value["maxStdoutBytes"], value["maxStderrBytes"])


def load_workloads(mapping: object) -> Mapping[str, Workload]:
    """Validate and freeze operator references without importing them in the parent."""
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
