# CAP Governance

This document describes the governance model for the Cordum Agent Protocol (CAP).

## Project Governance

CAP uses a **maintainer model**. The project is led by a small group of maintainers who have final authority over the direction of the protocol, spec, and SDK implementations.

**Lead Maintainer:** [@yaront1111](https://github.com/yaront1111) (Cordum)

As CAP matures toward neutral governance (see [CNCF Aspirations](#cncf-aspirations)), we intend to expand the maintainer group to include contributors from outside Cordum.

## Decision Making

Most decisions are made through the standard PR review process described in [CONTRIBUTING.md](CONTRIBUTING.md). Larger changes follow an RFC process:

### Standard Changes
- Bug fixes, documentation, SDK improvements, and non-wire changes follow the normal PR workflow.
- A maintainer or committer reviews and merges.

### Spec and Protocol Changes (RFC)
- Open a [GitHub Discussion](https://github.com/cordum-io/cap/discussions) with the prefix **"RFC:"** in the title.
- Describe the problem, proposed change, alternatives considered, and backward compatibility impact.
- Allow a **7-day comment period** for community feedback.
- A maintainer makes the final decision and records the rationale in the discussion.

### Wire-Level Changes
- Changes that affect `BusPacket` wire format or require a `protocol_version` bump follow the RFC process with an extended **14-day review period**.
- Wire evolution is append-only — see [spec/17-versioning-policy.md](spec/17-versioning-policy.md) for the full policy.
- All wire changes require conformance fixture updates across all SDKs.

## Roles

### Maintainer
- Full merge rights on all branches.
- Release authority (tagging, publishing).
- Final say on spec changes and protocol direction.
- Responsible for security response (see [SECURITY.md](SECURITY.md)).

### Committer
- Merge rights for non-wire PRs (SDK features, docs, tooling, examples).
- Cannot merge spec or proto changes without maintainer approval.
- Nominated by a maintainer after sustained contributions.

### Contributor
- Anyone who opens PRs, files issues, reviews code, or participates in discussions.
- No special permissions required — all contributions are welcome.
- See [CONTRIBUTING.md](CONTRIBUTING.md) for how to get started.

## Becoming a Maintainer

Maintainers are invited by existing maintainers based on:

- Sustained, high-quality contributions over multiple releases.
- Deep understanding of the protocol spec and cross-SDK implications.
- Demonstrated good judgment in code review and design decisions.
- Alignment with the project's values: stability, safety, and interoperability.

There is no formal application process. If you're contributing regularly and making good decisions, we'll reach out.

## Release Process

- Releases are triggered by Git tags (e.g., `v2.0.20`) on the `main` branch.
- Every user-facing change requires a CHANGELOG entry.
- Wire-level changes get a `[WIRE]` prefix in the CHANGELOG and bump the wire version.
- All three SDKs (Go, Node, Python) must pass CI before a tag is applied.
- See [spec/17-versioning-policy.md](spec/17-versioning-policy.md) for the complete versioning policy, SDK version matrix, and migration checklist.

## Protocol Stability Pledge

The CAP v1 wire format is **frozen** — append-only evolution only. Existing fields will never be renumbered, removed, or repurposed. This means:

- Any conformant CAP v1 implementation will continue to interoperate with future versions.
- New capabilities are added via new fields and message types, not by changing existing ones.
- Wire version bumps (if ever needed) follow a strict migration process with backward compatibility.

## Licensing

- **Code** (SDKs, tools, examples): [Apache License 2.0](LICENSE)
- **Specification** (spec/): Apache License 2.0
- **"CAP"** and **"Cordum Agent Protocol"** are trademarks of Cordum. Use of these names in conformant implementations is permitted and encouraged.

## CNCF Aspirations

CAP aspires to become a CNCF project. Toward that goal, we are working to:

- Expand maintainership beyond Cordum to include independent contributors.
- Establish a neutral technical steering committee.
- Meet CNCF sandbox requirements for governance, security, and community health.
- Maintain a clear separation between the open protocol (CAP) and the commercial implementation (Cordum).

These are goals, not guarantees. We're building toward them transparently.

## Changes to This Document

Governance changes follow the RFC process with a 14-day review period. Propose changes via a GitHub Discussion with the prefix "RFC: Governance".
