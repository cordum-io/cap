# coretex Agent Protocol - Specification Index

This folder contains the normative specification for the coretex Agent Protocol (CAP). The protobuf definitions in `proto/` are the canonical wire format; the prose here defines semantics, expectations, and recommended behaviors.

## Conformance
- RFC 2119 keywords (MUST, SHOULD, MAY, etc.) are normative.
- Compatibility is append-only: existing fields are never renumbered or repurposed.
- Implementations are compliant when they honor message shapes, state machine rules, and safety hooks defined here.

## Versioning
- `protocol_version` in `BusPacket` is used for wire negotiation. Current wire version: `1` (schema 1.0.0).
- Protobuf evolution is append-only: add new fields with new numbers; do not delete or reuse.
- Repository/SDK releases track implementation bits (Go/Python/Node/C++); pin to tags (current: `v2.0.0`) for reproducibility.
- Protocol vs SDK:
  - Protocol wire schema: 1.0.0 (stable).
  - Repository/SDK: 2.0.0 (may add helpers, docs, and generated stubs without wire breaks).

## Table of Contents
- [01 Overview](01-overview.md)
- [02 Envelope - BusPacket](02-envelope-buspacket.md)
- [03 Job Protocol](03-job-protocol.md)
- [04 Memory Pointer Spec](04-memory-pointer-spec.md)
- [04b Context and Memory Model](04b-context-and-memory.md)
- [05 Heartbeats](05-heartbeats.md)
- [06 Safety](06-safety.md)
- [07 State Machine](07-state-machine.md)
- [08 Workflows](08-workflows.md)
- [09 Transport Profile](09-transport-profile.md)
- [10 Security and Observability](10-security-observability.md)
- [11 Security Best Practices](11-security-best-practices.md)
- [12 Glossary](12-glossary.md)
