# cap-adapter-langgraph

An operator-run CAP worker that runs operator-registered LangGraph graphs as CAP jobs. It is built on cap-sdk-python's `cap.Agent`, has no Cordum dependency, is Apache-2.0, and is not published to PyPI. It requires Python 3.11 or later: on Python 3.10, langgraph 1.2.18 cannot deliver custom stream events from an async run, and those events are the adapter's only progress and artifact channel.

## Install (from the repo root)

```sh
pip install --require-hashes --no-deps -r adapters/langgraph/requirements.lock
python -m build --no-isolation --wheel --outdir dist sdk/python && python -m build --no-isolation --wheel --outdir dist adapters/langgraph
pip install --no-index --no-deps dist/*.whl && pip check
```

The lock is generated with uv 0.11.15 from the repo root:
`printf '%s\n' 'langgraph==1.2.18' 'langsmith==0.14.14' 'protobuf>=6.31.1,<7' 'grpcio>=1.76.0,<2' 'nats-py>=2.6.0' 'cryptography>=41.0.0' 'pydantic>=2.6.0' 'redis>=5.0.0' 'pytest>=8.0,<9' 'build>=1.2,<2' 'setuptools==84.1.0' 'wheel==0.47.0' | uv pip compile - --universal --python-version 3.11 --generate-hashes --no-header --exclude-newer 2026-10-28T23:59:59Z -o adapters/langgraph/requirements.lock`

## Configuration

`cap-adapter-langgraph --config adapter.json` reads one `cap-adapter-config/v1` file. Files over 1 MiB, duplicate keys, unknown keys and missing keys are rejected.

```json
{"schemaVersion": "cap-adapter-config/v1", "topic": "job.adapter.langgraph", "workerId": "wrk-lg-1", "trustFile": "trust.json",
 "nats": {"url": "tls://nats.example:4222", "caFile": "ca.pem", "certFile": "client.pem", "keyFile": "client-key.pem"},
 "resources": {"bucket": "cap-resources"}, "limits": {"maxInputBytes": 65536, "maxOutputBytes": 65536, "maxArtifacts": 8, "maxArtifactBytes": 65536, "maxDeadlineMs": 30000},
 "workloads": {"two-step-upper": {"factory": "operator_graphs:build_upper", "inputModel": "operator_graphs:UpperInput"}}}
```

- Relative paths resolve against the directory of the file that names them, never the working directory, and every named file must exist.
- `nats.url` must be `tls://`. `caFile` verifies the server with hostname checking on; `certFile` and `keyFile` are the client's TLS identity.
- `resources.bucket` is the JetStream key-value bucket behind the nats-kv ResourceRegistry, which resolves `context_ref` and stores `result_ref` and artifacts.
- `trustFile` is a `cap-adapter-trust/v1` file with `tenantId`, `agentId`, `schedulerId`, `proofKeyId`, `proofKeyFile` (an unencrypted P-256 private key PEM), and `schedulerTrustKeys` and `schedulerSigningKeys`: non-empty `{keyId: P-256 public key PEM path}` maps for the worker handshake and for CAP-PRODUCTION packet signatures.
- The worker runs with worker trust ENFORCE, CAP-PRODUCTION admission with `topic` as the audience, no retries and one job at a time.

## Workloads and events

- A job selects its workload only by `JobMetadata.capability`, an exact key of `workloads`. The packet's `env`, `labels` and `adapter_id` never select code, paths or environment.
- Every `factory` and `inputModel` is imported at startup, so a bad config fails before connecting. The factory is called once per job and returns a compiled graph; the input JSON is validated with the pydantic `inputModel` first.
- The graph runs with `astream(state, stream_mode=["custom", "values"])`. Nodes report through `langgraph.config.get_stream_writer()`:
  - `{"cap.progress": "<marker>"}` publishes ordered, sealed progress;
  - `{"cap.artifact": {"name": ..., "mediaType": ..., "content": ...}}` attaches an artifact ref (str content is UTF-8 encoded, bytes are kept);
  - any other shape with a `cap.` key is a framework error, and events without a `cap.` key are ignored.
- The last `values` chunk, limited to the graph's output schema, is the result.
- Error messages are fixed texts. Exception text and packet fields never reach results, progress or adapter logs, which name only the workload and the exception type.

| condition | status | `error_code_enum` | `error_code` |
|---|---|---|---|
| capability not registered | FAILED | `ERROR_CODE_JOB_NOT_FOUND` | — |
| input or output over its bound | FAILED | `ERROR_CODE_JOB_RESOURCE_EXHAUSTED` | — |
| input fails `inputModel` | FAILED | `ERROR_CODE_JOB_INVALID_INPUT` | — |
| deadline reached | FAILED | `ERROR_CODE_JOB_TIMEOUT` | `TIMEOUT` |
| the graph raised | FAILED | `ERROR_CODE_UNSPECIFIED` | `FRAMEWORK_ERROR` |
| cancel honoured | CANCELLED | — | — |
| operator code raised `cap.JobFailure` | as raised | as raised | as raised |

## Runtime

- **Deadline and cancel** are owned by cap-sdk-python. The deadline is `min(budget.deadline_ms, maxDeadlineMs)`, or `maxDeadlineMs` when the packet sets none, measured from handler start; a fenced `sys.job.cancel` cancels the run. Async nodes stop at their next `await` and leave no task or thread behind. A synchronous node runs in LangGraph's executor thread and cannot be stopped in-process: the outcome is still reported at the bound, but that thread finishes its current node. Operators who need a hard stop should isolate the graph in a process.
- **Tracing is forced off.** At import the adapter sets `LANGSMITH_TRACING_V2`, `LANGSMITH_TRACING`, `LANGCHAIN_TRACING_V2` and `LANGCHAIN_TRACING` to `false`, overriding any operator value, and every run is wrapped in `langsmith.tracing_context(enabled=False)`, so no LangSmith trace is sent.
- **CLI:** `cap-adapter-langgraph --config PATH [--check]`. `--check` loads the config, the trust keys, the TLS files and every workload without network I/O and prints `CHECK OK <n> workloads`. Exit codes: 0 for a passed check or a clean stop (SIGTERM or Ctrl-C), 2 for a config error, 1 for any other failure.

## Tests

`PYTHONPATH=testdata/adapters/nettrap python -m pytest -q adapters/langgraph/tests` runs deterministic pure-Python graphs, with no LLM, through every case of `testdata/adapters/framework-golden-v1.json`. `testdata/adapters/nettrap/sitecustomize.py` is a test-only network trap that is never shipped: it refuses any non-loopback connection and makes the process exit with status 97. `python -I adapters/langgraph/tests/fixtures/golden_graphs.py`, run from outside the repo, is the installed-wheel smoke. The real-NATS end-to-end run of `serve()` belongs to the TCK harness slice.
