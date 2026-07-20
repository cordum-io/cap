# 17 — Versioning Policy

This document defines the versioning scheme for the Cordum Agent Protocol (CAP), its SDK implementations, and the relationship between them.

## Wire Version

The `protocol_version` field in `BusPacket` carries the wire version as a positive integer.

| Property | Value |
|----------|-------|
| Current wire version | **1** |
| Field | `BusPacket.protocol_version` (int32) |
| Bump trigger | Breaking wire changes only |

### Current Release and Wire Status

<!-- cap-release:begin:version-policy -->
- **Wire protocol version:** 1. Wire evolution is append-only within the compatible range 1–1.
- **Current published release:** 2.14.0 (tag `v2.14.0`). SDK and repository releases track implementation and are pinned by tag.
- **Source versus release:** development source may carry an in-progress version distinct from the latest published artifact; the release manifest is the authority on what is published.
- **Release candidate (not published):** 2.15.1 (tag `v2.15.1`, channel stable).
<!-- cap-release:end -->

### What constitutes a breaking wire change

A wire version bump is required when:

- An existing field is renumbered, removed, or repurposed.
- The semantics of an existing field change in a way that makes old consumers misinterpret new packets.
- A new field is made mandatory for correct packet processing (i.e., old consumers would silently drop critical data).

A wire version bump is **NOT** required for:

- Adding new fields with new field numbers (append-only evolution).
- Adding new `oneof` variants.
- Adding new enum values to existing enums.
- Adding new message types.
- Updating default values for optional fields.

### Compatibility guarantees

Implementations MUST:
- Accept packets where `protocol_version` matches their supported version.
- Ignore unknown fields (standard protobuf behavior).

Implementations SHOULD:
- Accept packets from one version below their own (version N-1), ignoring unknown fields.

Implementations MAY:
- Reject packets with a higher wire version by responding with `ERROR_CODE_PROTOCOL_VERSION_MISMATCH`.

## SDK Version

Each SDK follows [Semantic Versioning 2.0.0](https://semver.org/):

| Component | Bump when |
|-----------|-----------|
| MAJOR | Public API surface breaks (removed exports, changed function signatures) |
| MINOR | New features added (new validation helpers, new constants, new runtime capabilities) |
| PATCH | Bug fixes, documentation updates, dependency bumps |

All three SDKs (Go, Node, Python) currently share the same version tag since they are released from the same repository. This MAY change if SDKs are split into separate repositories in the future.

### Current versions

| SDK | Package | Current Version |
|-----|---------|-----------------|
| Go | `github.com/cordum-io/cap/v2/sdk/go` | v2.0.19 |
| Node | `cap-sdk-node` | 2.0.18 |
| Python | `cap-sdk-python` | 0.1.0 |
| Python Guard | `cordum-guard` | 0.1.0 |

## Wire Version ↔ SDK Version Matrix

| Wire Version | SDK Versions | Notes |
|-------------|-------------|-------|
| 1 | v0.1.0 – current | Initial stable wire format |

## Repository Tags

Repository tags follow the pattern `v{version}` (e.g., `v2.0.19`). Tags are applied to the `main` branch after all SDKs are updated and CI passes.

## Migration Checklist

When a wire version bump is required, follow this checklist:

1. **Proto changes**: Update the protobuf definitions with the breaking change. Increment the wire version constant in each SDK.
2. **Spec updates**: Document the breaking change in the relevant spec section. Update this versioning policy's compatibility matrix.
3. **SDK updates**: Update all three SDKs to handle both the old and new wire versions during the transition period. Update `DefaultProtocolVersion` / `DEFAULT_PROTOCOL_VERSION` constants.
4. **Conformance fixtures**: Generate new fixtures for the new wire version. Ensure old fixtures still parse (backward compat).
5. **CHANGELOG**: Add an entry prefixed with `[WIRE]` describing the breaking change and migration steps.
6. **Tag release**: Tag a new MAJOR or MINOR SDK version depending on whether SDK APIs also changed.
7. **Deprecation**: Mark the old wire version as deprecated. Remove support after at least two MINOR SDK releases.
