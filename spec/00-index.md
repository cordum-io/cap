# Cortex Agent Protocol - Specification Index

This folder contains the normative specification for the Cortex Agent Protocol (CAP). The protobuf definitions in `proto/` are the canonical wire format; the prose here defines semantics, expectations, and recommended behaviors.

## Conformance
- RFC 2119 keywords (MUST, SHOULD, MAY, etc.) are normative.
- Compatibility is append-only: existing fields are never renumbered or repurposed.
- Implementations are compliant when they honor message shapes, state machine rules, and safety hooks defined here.

## Versioning
- `protocol_version` in `BusPacket` is used for wire negotiation. Current: `1`.
- Protobuf evolution follows the standard rule set: add new fields with new numbers; do not delete or reuse.
- Transport and workflow guidance are versioned in this repo via tags/releases; consumers should pin to a release.

## Table of Contents
- [01 Overview](01-overview.md)
- [02 Envelope - BusPacket](02-envelope-buspacket.md)
- [03 Job Protocol](03-job-protocol.md)
- [04 Memory Pointer Spec](04-memory-pointer-spec.md)
- [05 Heartbeats](05-heartbeats.md)
- [06 Safety](06-safety.md)
- [07 State Machine](07-state-machine.md)
- [08 Workflows](08-workflows.md)
- [09 Transport Profile](09-transport-profile.md)
- [10 Security and Observability](10-security-observability.md)
