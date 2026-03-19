# [Good First Issue] Improve Python SDK error messages for NATS connection failures

## Area
Python SDK, error handling

## Context
When the Python SDK (`sdk/python/cap/bus.py`) fails to connect to NATS, the error messages can be cryptic — often just a raw `ConnectionRefusedError` or timeout without context about what went wrong or how to fix it. New users frequently hit these errors when setting up their development environment.

Better error messages save debugging time and reduce friction for adopters.

## What to Do
1. Read `sdk/python/cap/bus.py` and find the NATS connection code (look for `nats.connect()` or similar)
2. Identify the common failure modes:
   - NATS server not running (connection refused)
   - Wrong host/port (timeout)
   - Authentication failure (permission denied)
3. Wrap each failure with a more descriptive error message that includes:
   - What failed (e.g., "Could not connect to NATS server at nats://localhost:4222")
   - Likely cause (e.g., "Is the NATS server running?")
   - How to fix it (e.g., "Start NATS with: nats-server or docker run -p 4222:4222 nats:latest")
4. Run existing tests: `cd sdk/python && python -m pytest tests/`
5. Add a test for at least one improved error message

## Files to Look At
- `sdk/python/cap/bus.py` — NATS connection logic
- `sdk/python/tests/` — existing test suite
- `sdk/python/cap/errors.py` — existing error types (if any)

## Definition of Done
- [ ] At least 3 NATS connection error messages improved with context and remediation hints
- [ ] Existing tests still pass
- [ ] At least 1 new test verifying an improved error message
- [ ] No changes to the public API

## Helpful Resources
- [Getting Started](../../docs/getting-started.md) — covers NATS setup
- [Troubleshooting](../../docs/troubleshooting.md)

## Estimated Effort
2-3 hours
