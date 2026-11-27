"""One contained CrewAI job. Read stdin before loading framework or operator code."""
import importlib
import inspect
import json
import os
import sys
from threading import Lock
from typing import Callable


def _import(ref: str) -> object:
    module, name = ref.split(":")
    return getattr(importlib.import_module(module), name)


def _run(job: dict, send: Callable[[dict[str, object]], None]) -> object:
    from cap import JobFailure
    from cap.pb.cordum.agent.v1 import job_pb2
    from crewai import Crew, Process
    from pydantic import BaseModel

    factory, model = _import(job["factory"]), _import(job["inputModel"])
    if not callable(factory) or not isinstance(model, type) or not issubclass(model, BaseModel):
        raise JobFailure("FRAMEWORK_ERROR")
    try:
        inputs = model.model_validate(job["input"]).model_dump(mode="json")
    except JobFailure:
        raise
    except Exception:
        raise JobFailure(job_pb2.ERROR_CODE_JOB_INVALID_INPUT) from None
    crew = factory()
    if not isinstance(crew, Crew) or crew.tracing is not False or crew.process != Process.sequential:
        raise JobFailure("FRAMEWORK_ERROR")
    for task in crew.tasks:
        original = task.callback  # CrewAI separately invokes crew.task_callback; do not double-call it.

        async def callback(output, name=task.name, previous=original):
            if previous is not None:
                result = previous(output)
                if inspect.isawaitable(result):
                    await result
            if name is not None:
                send({"type": "progress", "marker": name})
            if name in job["artifactTasks"]:
                send({"type": "artifact", "name": job["artifactTasks"][name],
                      "mediaType": "text/plain", "content": output.raw})

        task.callback = callback
    output = crew.kickoff(inputs=inputs)
    return output.json_dict if output.json_dict is not None else {"raw": output.raw}


def _check(workloads: dict[str, dict[str, str]]) -> list[str] | None:
    from pydantic import BaseModel

    for key, refs in workloads.items():
        for field in ("factory", "inputModel"):
            try:
                value = _import(refs[field])
            except Exception:
                return [key, field, "cannot be imported"]
            if field == "factory" and not callable(value):
                return [key, field, "is not callable"]
            if field == "inputModel" and (not isinstance(value, type) or not issubclass(value, BaseModel)):
                return [key, field, "must be a pydantic BaseModel subclass"]
    return None


def main() -> None:
    line = sys.stdin.buffer.readline()  # Windows containment is established before this arrives.
    ipc = os.dup(1)
    os.set_inheritable(ipc, False)
    os.dup2(2, 1)  # Operator prints and inherited stdout are stderr, never the private event pipe.
    lock = Lock()  # Sequential crews may have async_execution tasks with threaded callbacks.
    with os.fdopen(ipc, "w", encoding="utf-8", newline="\n") as stream:
        def send(event: dict[str, object]) -> None:
            line = json.dumps(event, allow_nan=False) + "\n"
            with lock:
                stream.write(line)
                stream.flush()

        from cap import JobFailure

        try:
            job = json.loads(line)
            output = _check(job["check"]) if isinstance(job, dict) and set(job) == {"check"} else _run(job, send)
            send({"type": "result", "output": output})
        except JobFailure as failure:
            send({"type": "error", "code": failure.code, "retryable": failure.retryable})
        except Exception:
            send({"type": "error", "code": "FRAMEWORK_ERROR", "retryable": None})


if __name__ == "__main__":
    main()  # Normal shutdown lets the network trap's atexit hook report blocked connections.
