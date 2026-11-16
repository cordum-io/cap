# cap-adapter-crewai

A CAP worker adapter for operator-registered CrewAI crews, built on cap-sdk-python's `cap.Agent`, with no Cordum dependency. It is Apache-2.0, supports Python 3.10 to 3.13, and is not published to PyPI. This slice ships the bounded child-process runner, `cap_adapter_crewai._proc`; the crew runner, its configuration and the `cap-adapter-crewai` command come with the next slice.

## Install (from the repo root)

```sh
pip install --require-hashes --no-deps -r adapters/crewai/requirements.lock
python -m build --no-isolation --wheel --outdir dist sdk/python && python -m build --no-isolation --wheel --outdir dist adapters/crewai
pip install --no-index --no-deps dist/*.whl && pip check
```

The lock is generated with uv 0.11.15 from the repo root:
`printf '%s\n' 'protobuf>=6.31.1,<7' 'grpcio>=1.76.0,<2' 'nats-py>=2.6.0' 'cryptography>=41.0.0' 'pydantic>=2.6.0' 'redis>=5.0.0' 'pytest>=8.0,<9' 'build>=1.2,<2' 'setuptools==84.1.0' 'wheel==0.47.0' | uv pip compile - --universal --python-version 3.10 --generate-hashes --no-header --exclude-newer 2026-10-28T23:59:59Z -o adapters/crewai/requirements.lock`

## The child process

Every job runs in a fresh child process, so a crew that cannot be stopped in-process is still stopped.

- **Launch.** An explicit argv, never a shell. The child's whole environment is `PATH`, `SYSTEMROOT` and `PYTHONPATH`, plus `APPDATA`, `LOCALAPPDATA` and `USERPROFILE` on Windows; `TMP`, `TEMP` and `TMPDIR`, and `XDG_DATA_HOME` on POSIX, all set to a new temp dir that is also its working directory; and the telemetry switches `OTEL_SDK_DISABLED`, `CREWAI_DISABLE_TELEMETRY`, `CREWAI_DISABLE_TRACKING` and `CREWAI_DISABLE_VERSION_CHECK` set to `true` and `CREWAI_TRACING_ENABLED` set to `false`, which override any operator value.
- **Containment** is in place before the one JSON job line is written to the child's stdin. On POSIX the child leads a new session, and a kill sends `SIGTERM` to its process group, then `SIGKILL` after `graceMs`. On Windows it runs in a new process group inside a kill-on-close Job Object, and a kill is `TerminateJobObject`. If the Job Object cannot be set up, the child is killed and the job fails closed; there is no `taskkill` fallback.
- **Events.** The child writes JSONL events to stdout: `progress`, `artifact`, `result` and `error`. Each line is at most 1 MiB, and all of stdout at most `maxStdoutBytes`. stderr is drained, at most `maxStderrBytes` of it is kept, and it is never parsed, logged or published.
- **Outcome.** The first `result` or `error` event ends the job, and the child must then exit with status 0 within `graceMs`. Anything else fails the job with FAILED, `ERROR_CODE_UNSPECIFIED` and `error_code` `CHILD_CRASHED`: a spawn or containment failure, an exit before a terminal event, an over-long line or too much output, a line that is not a known event, or a non-zero exit.
- **Cleanup.** On every exit path, including a cancel and the SDK deadline, the whole process tree is dead before the temp dir is removed.
- **Ceilings.** On POSIX, a process that starts its own session leaves the process group and is not killed. On Windows, CrewAI keeps per-user data under `APPDATA`, which the child cannot override without breaking `import crewai`, so that data lives outside the temp dir and outlasts the job.

## Tests

`PYTHONPATH="$PWD/testdata/adapters/nettrap" python -m pytest -q adapters/crewai/tests` runs the child-lifecycle tests. `testdata/adapters/nettrap/sitecustomize.py` is the test-only network trap shared with the LangGraph adapter, and it is never shipped. `PYTHONPATH` must be absolute, because the child's working directory is its temp dir.
