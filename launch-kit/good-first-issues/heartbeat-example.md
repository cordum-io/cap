# [Good First Issue] Add standalone heartbeat example to examples/

## Area
Examples, documentation

## Context
CAP workers send periodic heartbeats to signal liveness, load, and capacity. The heartbeat protocol is defined in `proto/cordum/agent/v1/heartbeat.proto` and documented in the spec, but there is no standalone example showing how to implement heartbeat sending and receiving. The existing `examples/simple-echo/` focuses on job request/result flow.

A heartbeat example would help new users understand worker liveness monitoring.

## What to Do
1. Read the heartbeat proto definition: `proto/cordum/agent/v1/heartbeat.proto`
2. Read the relevant spec section on heartbeats
3. Look at `examples/simple-echo/python-worker/main.py` for the general example pattern
4. Create `examples/heartbeat/` with:
   - `python-sender/main.py` — a worker that sends heartbeats every 5 seconds with load/capacity info
   - `python-receiver/main.py` — a monitor that subscribes to heartbeat subjects and prints liveness info
   - `README.md` — setup instructions, what it demonstrates, expected output
5. Use the Python SDK (`sdk/python/`) for NATS connection and message encoding
6. Test with a local NATS server: `nats-server` then run sender and receiver in separate terminals

## Files to Look At
- `proto/cordum/agent/v1/heartbeat.proto` — heartbeat message definition
- `examples/simple-echo/python-worker/main.py` — example pattern to follow
- `sdk/python/cap/bus.py` — NATS publish/subscribe
- `examples/heartbeat.json` — sample heartbeat JSON payload

## Definition of Done
- [ ] `examples/heartbeat/` directory with sender, receiver, and README
- [ ] Sender publishes heartbeats with worker ID, load, and capacity
- [ ] Receiver subscribes and prints received heartbeats
- [ ] README includes setup, run instructions, and expected output
- [ ] Works with a local NATS server (no other dependencies)

## Helpful Resources
- [Getting Started](../../docs/getting-started.md)
- [CAP Spec](../../spec/00-index.md)
- [Heartbeat JSON example](../../examples/heartbeat.json)

## Estimated Effort
2-3 hours
