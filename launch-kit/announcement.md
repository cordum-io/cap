# coretex Agent Protocol (CAP) v1.0 Launch Announcement

**coretexOS announces CAP v1.0, the open protocol for distributed AI agents.**

CAP standardizes how gateways, schedulers, workers, orchestrators, and safety services exchange jobs over a bus. It makes multi-agent systems interoperable across vendors and transports.

## Why CAP
- MCP is for single-model tool calls; CAP is for distributed execution with scheduling, heartbeats, safety, and state.
- Keeps payloads off the wire via pointers for scale and security.
- Built-in safety hook for allow/deny/throttle/human-in-loop.

## What’s in v1.0
- Normative spec in `spec/` with RFC 2119 language.
- Canonical protobufs in `proto/coretex/agent/v1/`.
- Examples for jobs, heartbeats, and workflows in `examples/`.
- Reference implementation: **coretexOS** (official CAP reference).
- Apache-2.0 license.

## How to Get Started
- Read `README.md` and `spec/00-index.md`.
- Copy the Python/Go worker quickstarts (coming in `/examples`).
- Use the provided `.proto` files for any language.

## Call to Action
- Build your first CAP worker today.
- Join the CAP working group (Slack/Discord placeholder).
- File issues and proposals under `/rfcs` (coming soon).
