"""Structural helpers for fail-closed GitHub workflow contract tests."""

from __future__ import annotations

from typing import Callable, Dict

import yaml


def _literal(value: object) -> str:
    return str(value).strip().lower().replace(" ", "")


def _assert_required(node: Dict[object, object], label: str) -> None:
    if "continue-on-error" not in node:
        return
    assert _literal(node["continue-on-error"]) in {"false", "${{false}}"}, (
        f"{label} must not use continue-on-error"
    )


def _is_mandatory(
    node: Dict[object, object], label: str, allow_disabled: bool
) -> bool:
    if "if" not in node:
        return True
    condition = _literal(node["if"])
    if condition in {"true", "${{true}}"}:
        return True
    if allow_disabled and condition in {"false", "${{false}}"}:
        return False
    raise AssertionError(f"{label} has a non-mandatory condition")


def workflow_mapping(workflow: str) -> Dict[object, object]:
    """Parse one workflow without YAML boolean-key coercion."""
    document = yaml.load(workflow, Loader=yaml.BaseLoader)
    assert isinstance(document, dict), "workflow root must be a mapping"
    return document


def _active_job(workflow: str, name: str) -> Dict[object, object]:
    document = workflow_mapping(workflow)
    jobs = document.get("jobs")
    assert isinstance(jobs, dict), "workflow jobs must be a mapping"
    job = jobs.get(name)
    assert isinstance(job, dict), f"missing workflow job: {name}"
    _assert_required(job, f"job {name}")
    _is_mandatory(job, f"job {name}", allow_disabled=False)
    steps = job.get("steps")
    if steps is None:
        return job
    assert isinstance(steps, list), f"job {name} steps must be a list"
    active_steps: list[object] = []
    for index, step in enumerate(steps):
        assert isinstance(step, dict), f"job {name} step {index} must be a mapping"
        _assert_required(step, f"job {name} step {index}")
        if not _is_mandatory(step, f"job {name} step {index}", allow_disabled=True):
            continue
        active_steps.append(step)
    active = dict(job)
    active["steps"] = active_steps
    return active


def _steps(job: Dict[object, object]) -> list[Dict[object, object]]:
    value = job.get("steps", [])
    assert isinstance(value, list), "workflow job steps must be a list"
    assert all(isinstance(step, dict) for step in value), (
        "workflow job steps must be mappings"
    )
    return value


def active_steps(workflow: str, name: str) -> tuple[Dict[object, object], ...]:
    """Return mandatory executable steps in source order."""
    return tuple(_steps(_active_job(workflow, name)))


def job_config(workflow: str, name: str) -> Dict[object, object]:
    """Return exact job-level configuration without step metadata."""
    return {
        key: value for key, value in _active_job(workflow, name).items()
        if key not in {"name", "steps", "if", "continue-on-error"}
    }


def job_value(workflow: str, name: str, *path: str) -> object:
    """Return one required nested job-config value by exact path."""
    current: object = job_config(workflow, name)
    for key in path:
        assert isinstance(current, dict) and key in current, (
            f"job {name} config path is missing: {'.'.join(path)}"
        )
        current = current[key]
    return current


def _strip_shell_comment(line: str) -> str:
    single = double = escaped = False
    for index, character in enumerate(line):
        if escaped:
            escaped = False
            continue
        if character == "\\" and not single:
            escaped = True
        elif character == "'" and not double:
            single = not single
        elif character == '"' and not single:
            double = not double
        elif character == "#" and not single and not double:
            previous = line[index - 1] if index else " "
            if previous.isspace() or previous in ";|&()":
                return line[:index].rstrip()
    return line.rstrip()


def _executable_run(step: Dict[object, object]) -> str:
    raw = str(step.get("run", ""))
    lines = (_strip_shell_comment(line) for line in raw.splitlines())
    return "\n".join(line for line in lines if line.strip())


def _unique_step_index(
    workflow: str,
    name: str,
    predicate: Callable[[Dict[object, object]], bool],
    label: str,
) -> int:
    matches = [index for index, step in enumerate(active_steps(workflow, name))
               if predicate(step)]
    assert len(matches) == 1, f"job {name} must have exactly one {label} step"
    return matches[0]


def run_step_index(workflow: str, name: str, token: str) -> int:
    """Locate the unique mandatory run step with a command-line prefix."""
    return _unique_step_index(
        workflow,
        name,
        lambda step: any(line.lstrip().startswith(token)
                         for line in _executable_run(step).splitlines()),
        f"run command starting with {token}",
    )


def run_step_text(workflow: str, name: str, prefix: str) -> str:
    """Return the normalized run body for one prefix-matched step."""
    step = active_steps(workflow, name)[run_step_index(workflow, name, prefix)]
    return _executable_run(step)


def action_step_index(workflow: str, name: str, prefix: str) -> int:
    """Locate the unique mandatory action step matching a prefix."""
    return _unique_step_index(
        workflow, name, lambda step: str(step.get("uses", "")).startswith(prefix),
        f"action matching {prefix}",
    )


def action_inputs(
    workflow: str, name: str, action_prefix: str
) -> Dict[object, object]:
    """Return inputs from the unique matching mandatory action."""
    step = active_steps(workflow, name)[
        action_step_index(workflow, name, action_prefix)
    ]
    inputs = step.get("with")
    assert isinstance(inputs, dict), f"{action_prefix} must define action inputs"
    return inputs


def action_input_value(
    workflow: str, name: str, action_prefix: str, input_name: str
) -> object:
    """Return an exact input from the unique matching mandatory action."""
    inputs = action_inputs(workflow, name, action_prefix)
    assert input_name in inputs, f"{action_prefix} input {input_name} is missing"
    return inputs[input_name]


def id_step_index(workflow: str, name: str, identifier: str) -> int:
    """Locate the unique mandatory step with an exact id."""
    return _unique_step_index(
        workflow, name, lambda step: step.get("id") == identifier,
        f"id {identifier}",
    )


def id_step_text(workflow: str, name: str, identifier: str) -> str:
    """Return the normalized run body for one exact-id step."""
    step = active_steps(workflow, name)[id_step_index(workflow, name, identifier)]
    return _executable_run(step)


def run_text(workflow: str, name: str) -> str:
    """Return only commands executed by mandatory steps in one job."""
    return "\n".join(_executable_run(step) for step in active_steps(workflow, name)
                     if "run" in step)


def uses_values(workflow: str, name: str) -> tuple[str, ...]:
    """Return action references from mandatory steps in source order."""
    return tuple(str(step["uses"]) for step in active_steps(workflow, name)
                 if "uses" in step)


def require_action_input(
    workflow: str,
    name: str,
    action_prefix: str,
    input_name: str,
    expected: str,
) -> None:
    """Require one mandatory action step to bind an exact input value."""
    actual = action_input_value(workflow, name, action_prefix, input_name)
    assert _literal(actual) == _literal(expected), (
        f"{action_prefix} input {input_name} must equal {expected}, got {actual}"
    )
