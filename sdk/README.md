# CAP SDKs (Go, Python, Node/TypeScript)

This folder contains production-ready starter SDKs for CAP. Each SDK is bus-agnostic but ships NATS helpers as a sane default.

## Contents
- `go/` — Go SDK with NATS worker/client helpers.
- `python/` — Python SDK with asyncio NATS helpers.
- `node/` — Node/TypeScript SDK using NATS and protobufjs loaders.

## Proto Stubs
Generate language stubs from `proto/` before building:
- Go: `./tools/make_protos.sh` (outputs to `generated/go`) or run the commands in `sdk/go/README.md`.
- Python: run `./tools/make_protos.sh` or the `grpc_tools.protoc` command in `sdk/python/README.md` to place stubs under `sdk/python/cap/pb`.
- Node: install deps then run `npm run build` in `sdk/node` (protobufjs will load `.proto` at runtime by default; you can also precompile using `pbjs/pbts` if desired).

## Bus Choice
The helpers default to NATS. You can swap in Kafka or another pub/sub by replacing the bus adapter while keeping the same message shapes.
