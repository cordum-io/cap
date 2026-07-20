# Cordum Agent Protocol - Specification Index

This folder contains the normative specification for the Cordum Agent Protocol (CAP). The protobuf definitions in `proto/` are the canonical wire format; the prose here defines semantics, expectations, and recommended behaviors.

## Conformance
- RFC 2119 keywords (MUST, SHOULD, MAY, etc.) are normative.
- Compatibility is append-only: existing fields are never renumbered or repurposed.
- Implementations are compliant when they honor message shapes, state machine rules, and safety hooks defined here.

## Conformance Suite
Tags: `conformance`, `fixtures`, `testing`, `signing`, `deterministic`.
- Binary fixtures live in `spec/conformance/fixtures`.
- SDK tests load fixtures to validate wire compatibility across languages.
- Fixtures are signed deterministically to keep bytes stable across Go patch releases.

## Versioning
- `protocol_version` in `BusPacket` is used for wire negotiation; protobuf evolution is append-only (add new field numbers; never delete or reuse).
- Repository/SDK releases track implementation and are pinned by tag for reproducibility.

<!-- cap-release:begin:release-status -->
- **Current release:** 2.15.0 (tag `v2.15.0`, 2026-07-20, channel stable)
- **Wire protocol:** 1 (compatible range 1–1)
- **Wire schema:** 1.0.0
- **Specifications:** 19 normative documents
<!-- cap-release:end -->

- For the full versioning policy, see [17 Versioning Policy](17-versioning-policy.md).

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
- [13 Error Codes](13-error-codes.md)
- [14 Capability Negotiation](14-capability-negotiation.md)
- [15 Conformance Levels](15-conformance-levels.md)
- [16 Protocol Errors](16-protocol-errors.md)
- [17 Versioning Policy](17-versioning-policy.md)
- [19 CAP-PRODUCTION Profile](19-cap-production-profile.md)
