# Contributing to CAP

Thank you for helping improve the Cordum Agent Protocol. This guide covers everything you need to get started.

## Prerequisites

| Tool | Version | Notes |
| --- | --- | --- |
| Go | 1.24+ | Required for Go SDK and conformance fixture generation |
| Node.js | 20+ | Required for Node/TypeScript SDK |
| Python | 3.9+ | Required for Python SDK and cordum-guard |
| protoc | 3.21+ | Optional — only needed when editing `.proto` files |

## Dev Setup

```bash
# Clone the repo
git clone https://github.com/cordum-io/cap.git
cd cap

# Go SDK
go test ./sdk/go/...

# Node SDK
cd sdk/node && npm ci && npm test && cd ../..

# Python SDK
cd sdk/python && pip install -e . && python -m pytest tests/ && cd ../..

# Python Guard SDK
cd sdk/python-guard && pip install -e ".[dev]" && pytest && cd ../..
```

## Ground Rules

- Protocol evolution is **append-only**. Never renumber or repurpose existing protobuf fields.
- Use RFC 2119 language (MUST/SHOULD/MAY) in spec changes.
- Keep payloads off the wire — pointer semantics stay mandatory.
- Align spec prose with protobuf definitions; update both when changing behavior.

## PR Workflow

1. **Fork** the repo and create a branch from `main`:
   - `feat/` — new features or capabilities
   - `fix/` — bug fixes
   - `docs/` — documentation only
   - `ci/` — CI/CD changes
   - `chore/` — maintenance and cleanup
2. **Commit** with [Conventional Commits](https://www.conventionalcommits.org/) messages:
   - `feat: add validation helpers to Go SDK`
   - `fix: correct heartbeat pool field check`
   - `docs: update conformance README`
3. **Push** your branch and open a pull request against `main`.
4. **CI must pass** — all SDK tests run automatically.
5. **CODEOWNERS** will be assigned for review.

## Testing

All three SDKs must pass before a PR can merge:

```bash
# Run all tests
go test ./sdk/go/...          # Go
cd sdk/node && npm test        # Node
cd sdk/python && python -m pytest tests/  # Python
```

Additional checks:
- Conformance fixtures must not drift — if you change proto definitions, regenerate fixtures with `go run tools/conformance/generate_fixtures.go` and verify all SDKs still decode them.
- Proto files must compile: `protoc --proto_path=proto proto/cordum/agent/v1/*.proto --go_out=.`

## Proto Changes

Protobuf changes follow strict rules:

1. **Append-only** — add new fields/enums; never delete or reuse field numbers.
2. **Update the spec** — every proto change needs a matching spec update in `spec/`.
3. **Regenerate stubs** — rebuild Go (`protoc --go_out`), Python (`grpc_tools.protoc`), and Node stubs.
4. **Add conformance fixtures** — new message types need binary fixtures in `spec/conformance/fixtures/`.
5. **Cross-SDK tests** — verify all 3 SDKs decode the new fixtures correctly.
6. **CHANGELOG entry** — prefix wire-level changes with `[WIRE]`.

## Style

- **Markdown**: wrap at ~100 chars, use fenced code blocks with info strings.
- **Protos**: keep comments concise, `snake_case` for fields, `SCREAMING_SNAKE_CASE` for enums.
- **Go**: follow `gofmt` and standard library conventions.
- **TypeScript**: follow the existing `sdk/node` patterns.
- **Python**: follow PEP 8; use type hints for public APIs.

## Good First Issues

Look for issues labeled [`good-first-issue`](https://github.com/cordum-io/cap/labels/good-first-issue) — these are scoped tasks suitable for newcomers to the project.

## Releases

- Releases are triggered by Git tags (e.g., `v2.0.20`).
- Every user-facing change requires a CHANGELOG entry.
- Wire-level proto changes bump the wire version and get a `[WIRE]` prefix in the CHANGELOG.
- See `spec/17-versioning-policy.md` for the full versioning policy.

## Governance

For details on project governance, decision-making processes, roles, and the RFC process for spec changes, see [GOVERNANCE.md](GOVERNANCE.md).

## Community

- **Discord**: [Join us](https://discord.gg/U4NpXtjP)
- **GitHub Discussions**: [Ask questions](https://github.com/cordum-io/cap/discussions)
- **Email**: admin@cordum.io
