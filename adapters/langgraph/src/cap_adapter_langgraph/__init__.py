"""CAP worker adapter for operator-registered LangGraph graphs (Apache-2.0, no Cordum dependency).

A job selects its workload only by JobMetadata.capability. Factories and input
models come from the operator's local config and are imported at startup.
Bounds, deadline, cancel, sealing and artifact externalization stay in
cap-sdk-python; this module maps graph stream events to CAP progress,
artifacts and a result.
"""
import os

# Tracing is forced off before langsmith/langgraph are imported (no phone-home).
for _name in ("LANGSMITH_TRACING_V2", "LANGSMITH_TRACING", "LANGCHAIN_TRACING_V2", "LANGCHAIN_TRACING"):
    os.environ[_name] = "false"

import contextlib  # noqa: E402
import importlib  # noqa: E402
import logging  # noqa: E402
import re  # noqa: E402
from dataclasses import dataclass  # noqa: E402
from types import MappingProxyType  # noqa: E402
from typing import Any, Callable, Dict, Mapping, Optional, Type  # noqa: E402

from cap import Context, JobFailure  # noqa: E402
from cap.pb.cordum.agent.v1 import job_pb2  # noqa: E402
from langsmith.run_helpers import tracing_context  # noqa: E402
from pydantic import BaseModel  # noqa: E402

__all__ = ["ConfigError", "Limits", "Workload", "load_workloads", "make_handler", "agent_options"]

LOG = logging.getLogger("cap_adapter_langgraph")
_KEY = re.compile(r"[a-z0-9][a-z0-9._-]{0,63}")
_REF = re.compile(r"[A-Za-z_][A-Za-z0-9_.]*:[A-Za-z_][A-Za-z0-9_]*")
FRAMEWORK_ERROR = "FRAMEWORK_ERROR"


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
