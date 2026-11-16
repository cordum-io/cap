"""Operator crews with a deterministic BaseLLM double; no parent CrewAI import."""
import json
import time
from pathlib import Path

import cap
from pydantic import BaseModel, ConfigDict, Field


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
