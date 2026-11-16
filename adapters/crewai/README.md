# cap-adapter-crewai

A CAP worker adapter for operator-registered CrewAI crews, built on cap-sdk-python's `cap.Agent`, with no Cordum dependency. It is Apache-2.0, supports Python 3.10 to 3.13, and is not published to PyPI. This slice ships the crew handler and bounded child runner. Configuration, trust loading and the `cap-adapter-crewai` command follow in task-beb195b6.

## Install (from the repo root)

```sh
pip install --require-hashes --no-deps -r adapters/crewai/requirements.lock
python -m build --no-isolation --wheel --outdir dist sdk/python && python -m build --no-isolation --wheel --outdir dist adapters/crewai
pip install --no-index --no-deps dist/*.whl && pip check
```

The lock is generated with uv 0.11.15 from the repo root:
`printf '%s\n' 'crewai==1.15.30' 'protobuf>=6.31.1,<7' 'grpcio>=1.76.0,<2' 'nats-py>=2.6.0' 'cryptography>=41.0.0' 'pydantic>=2.6.0' 'redis>=5.0.0' 'pytest>=8.0,<9' 'build>=1.2,<2' 'setuptools==84.1.0' 'wheel==0.47.0' | uv pip compile - --universal --python-version 3.10 --generate-hashes --no-header --exclude-newer 2026-10-28T23:59:59Z -o adapters/crewai/requirements.lock`

## The child process

Every job runs in a fresh child process, so a crew that cannot be stopped in-process is still stopped.

- **Launch.** An explicit argv, never a shell. The child's whole environment is `PATH`, `SYSTEMROOT` and `PYTHONPATH`, plus `APPDATA`, `LOCALAPPDATA` and `USERPROFILE` on Windows; `TMP`, `TEMP` and `TMPDIR`, and `XDG_DATA_HOME` on POSIX, all set to a new temp dir that is also its working directory; and the telemetry switches `OTEL_SDK_DISABLED`, `CREWAI_DISABLE_TELEMETRY`, `CREWAI_DISABLE_TRACKING` and `CREWAI_DISABLE_VERSION_CHECK` set to `true` and `CREWAI_TRACING_ENABLED` set to `false`, which override any operator value.
- **Containment** is in place before the one JSON job line is written to the child's stdin. On POSIX the child leads a new session, and a kill sends `SIGTERM` to its process group, then `SIGKILL` after `graceMs`. On Windows it runs in a new process group inside a kill-on-close Job Object, and a kill is `TerminateJobObject`. If the Job Object cannot be set up, the child is killed and the job fails closed; there is no `taskkill` fallback.
- **Events.** The child writes JSONL events to stdout: `progress`, `artifact`, `result` and `error`. Each line is at most 1 MiB, and all of stdout at most `maxStdoutBytes`. stderr is drained, at most `maxStderrBytes` of it is kept, and it is never parsed, logged or published.
- **Outcome.** The first `result` or `error` event ends the job, and the child must then exit with status 0 within `graceMs`. Anything else fails the job with FAILED, `ERROR_CODE_UNSPECIFIED` and `error_code` `CHILD_CRASHED`: a spawn or containment failure, an exit before a terminal event, an over-long line or too much output, a line that is not a known event, or a non-zero exit.
- **Cleanup.** On every exit path, including a cancel and the SDK deadline, the whole process tree is dead before the temp dir is removed.
- **Ceilings.** On POSIX, a process that starts its own session leaves the process group and is not killed. On Windows, CrewAI keeps per-user data under `APPDATA`, which the child cannot override without breaking `import crewai`, so that data lives outside the temp dir and outlasts the job.

## Tests

`PYTHONPATH="$PWD/testdata/adapters/nettrap" python -m pytest -q adapters/crewai/tests` runs the lifecycle tests, all nine framework-golden-v1 cases, telemetry proof/negative control and security boundaries. `testdata/adapters/nettrap/sitecustomize.py` is test-only and never shipped. The harness sets an absolute fixtures-plus-trap `PYTHONPATH` for children, whose working directory is a fresh temp dir. Crews use a deterministic BaseLLM double, not an external LLM.

The deadline golden retains every expected field but adds a measured startup allowance: twice a fresh echo child's elapsed time, rounded up to milliseconds, plus 1000 ms, before the golden's 250 ms. This accounts for importing CrewAI in a new process; it changes only the harness's operator deadline, not production behavior. The harness uses signed packets and the SDK ResourceRegistry over MockNATS in worker-trust warn mode: it is not real-NATS or CAP-PRODUCTION admission evidence (task-dbed002a owns that).

After installing both wheels and the lock in a **new venv**, run from outside the repo: `python -I <repo>/adapters/crewai/tests/fixtures/golden_crews.py`. It checks the installed origin, loads the trap by path and prints `SMOKE OK 9/9`.

## Workloads and events

`load_workloads(mapping)` accepts only operator configuration. Each capability key matches `[a-z0-9][a-z0-9._-]{0,63}`. The handler selects it by exact `JobMetadata.capability`; packet `env`, `labels`, `meta.labels` and `adapter_id` never select imports, paths, URLs, commands or environment.

Each entry requires `factory` and `inputModel` as `module:attribute` references. Optional `artifactTasks` maps task names to unique artifact names matching the capability-key pattern. Optional `child` requires all three integers: `graceMs` (1..60000), `maxStdoutBytes` and `maxStderrBytes` (1..67108864). Omission uses the child defaults above. Unknown fields are rejected. Example:

```python
workloads = load_workloads({"summarize": {
    "factory": "operator_crews:summary_crew", "inputModel": "operator_crews:TextInput",
    "artifactTasks": {"summarize": "summary"},
}})
agent.job(topic)(make_handler(workloads))  # construct cap.Agent with agent_options(operator_limits)
```

The parent validates reference shapes but never imports an operator module or CrewAI. The child reads the contained job line first, reserves a non-inheritable IPC fd and redirects ordinary stdout to stderr, then imports and validates the Pydantic input before calling the factory. It refuses a crew unless `tracing is False` and `process == Process.sequential`, before kickoff. Operator factories are trusted code, not sandboxed code.

Each completed named task emits progress using its exact name (the SDK requires 1..256 printable UTF-8 bytes); unnamed tasks emit none. Existing task callbacks are preserved, including async callbacks. A task in `artifactTasks` emits its raw output as UTF-8 `text/plain`. Empty content is refused by the SDK. The result is `CrewOutput.json_dict`, or `{"raw": CrewOutput.raw}` when that is absent. SDK bounds on input, output, artifact count/size and deadline come from `agent_options(Limits(...))`; oversize values are refused, not truncated.

| Condition | Status | Code |
|---|---|---|
| Unregistered capability (no child) | FAILED | ERROR_CODE_JOB_NOT_FOUND |
| Oversize input (no child), output or artifact | FAILED | ERROR_CODE_JOB_RESOURCE_EXHAUSTED |
| Invalid input (no kickoff) | FAILED | ERROR_CODE_JOB_INVALID_INPUT |
| SDK deadline | FAILED | ERROR_CODE_JOB_TIMEOUT / TIMEOUT |
| Framework exception or SDK event validation | FAILED | FRAMEWORK_ERROR |
| Malformed IPC, invalid error code, crash or IPC overflow | FAILED | CHILD_CRASHED |
| Fenced cancellation | CANCELLED | none |
| Operator `cap.JobFailure` | FAILED, FAILED_RETRYABLE or FAILED_FATAL | operator code and retryable preserved |

Framework exception text and child stderr never reach parent logs or published fields. An SDK JobFailure latch is propagated, not converted to a generic framework failure.

## License

This adapter is Apache-2.0. CrewAI 1.15.30 states MIT only in its README: its distribution has no LICENSE file and no license metadata. This upstream license gap must be rechecked on every CrewAI version bump.

Tags: cap, crewai, adapter, process-isolation.
