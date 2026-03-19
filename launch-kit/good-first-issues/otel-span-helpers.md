# [Good First Issue] Add OpenTelemetry span helpers to Python SDK

## Area
Python SDK, observability

## Context
The Python SDK (`sdk/python/`) handles job submission, result handling, and bus communication, but does not currently create OpenTelemetry spans around these operations. Adding span helpers would give users out-of-the-box distributed tracing for their CAP agents — making it easy to see job latency, error rates, and call chains in tools like Jaeger or Grafana Tempo.

The helpers should be opt-in (only activate if `opentelemetry-api` is installed) so they don't add a hard dependency.

## What to Do
1. Read `sdk/python/cap/bus.py` and `sdk/python/cap/client.py` to understand the main operations: `publish`, `subscribe`, `submit_job`, `wait_for_result`
2. Create `sdk/python/cap/tracing.py` with helper functions:
   - `trace_job_submit(job_id, topic)` — creates a span for job submission
   - `trace_job_result(job_id, status)` — creates a span for result handling
   - `trace_bus_publish(subject, size)` — creates a span for bus publish
3. Use `opentelemetry-api` with a try/except import so it's optional:
   ```python
   try:
       from opentelemetry import trace
       tracer = trace.get_tracer("cap-python-sdk")
   except ImportError:
       tracer = None
   ```
4. Add tests in `sdk/python/tests/test_tracing.py` that verify spans are created when OTel is available and that the helpers are no-ops when it's not
5. Run `cd sdk/python && python -m pytest tests/`

## Files to Look At
- `sdk/python/cap/bus.py` — bus operations to instrument
- `sdk/python/cap/client.py` — client operations to instrument
- `sdk/python/tests/` — test patterns to follow

## Definition of Done
- [ ] `sdk/python/cap/tracing.py` exists with at least 3 span helpers
- [ ] Helpers are no-ops when `opentelemetry-api` is not installed
- [ ] At least 2 tests in `test_tracing.py`
- [ ] Existing tests still pass
- [ ] No new hard dependencies added to `setup.py` / `pyproject.toml`

## Helpful Resources
- [OpenTelemetry Python API](https://opentelemetry.io/docs/languages/python/)
- [Getting Started](../../docs/getting-started.md)
- [CAP Spec](../../spec/00-index.md)

## Estimated Effort
3-4 hours
