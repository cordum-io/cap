# Contributing to CAP

Thank you for helping improve the coretex Agent Protocol.

## Ground Rules
- Protocol evolution is append-only. Do not renumber or repurpose existing protobuf fields.
- Use RFC 2119 language (MUST/SHOULD/MAY) in spec changes.
- Keep payloads off the wire—pointer semantics stay mandatory.
- Align spec prose with protobuf definitions; update both when changing behavior.

## Making Changes
1. Edit proto files under `proto/coretex/agent/v1/` and keep package/options consistent.
2. Update the matching spec page in `spec/` and link it from `00-index.md` if relevant.
3. Add or update examples in `examples/` when message shapes change.
4. Run `./tools/make_protos.sh` locally if you need generated stubs (optional, not checked in).

## Compatibility
- Additive changes only (new fields/enums); never delete or reuse field numbers.
- Preserve existing semantics; describe breaking changes in release notes if ever unavoidable.
- Prefer new enums or flags over reinterpreting existing ones.

## Style
- Markdown: wrap at ~100 chars, use fenced code blocks with info strings.
- Protos: keep comments concise, prefer snake_case for fields, SCREAMING_SNAKE_CASE for enums.

## Releases
- Tag versions when wire-level changes land; consumers should pin to tags for stability.
