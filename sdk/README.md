# CAP SDKs (Go, Python, Node/TypeScript, C++)

This folder contains production-ready starter SDKs for CAP. Each SDK is bus-agnostic but ships NATS helpers as a sane default.

## Contents
- `go/` — Go SDK with NATS worker/client helpers.
- `python/` — Python SDK with asyncio NATS helpers.
- `node/` — Node/TypeScript SDK using NATS and protobufjs loaders.
- `cpp/` — C++ SDK with a small bus interface and helper wrappers.

## Examples
- `examples/simple-echo/` contains Go/Python/Node client+worker pairs aligned with these SDKs.

## Proto Stubs
Generate language stubs from `proto/` before building:
- Go: `./tools/make_protos.sh` (outputs to `/coretex/agent/v1`) or run the commands in `sdk/go/README.md`.
- Python: run `./tools/make_protos.sh` (set `CAP_RUN_PY=1`; outputs to `/python`) or the `grpc_tools.protoc` command in `sdk/python/README.md` to place stubs under `sdk/python/cap/pb`.
- C++: `./tools/make_protos.sh` (outputs to `/cpp`) or let `sdk/cpp` CMake generate on build.
- Node: `./tools/make_protos.sh` (outputs CommonJS stubs to `/node`), or load `.proto` at runtime via protobufjs (see `sdk/node/src/protos.ts`).
- Node: install deps then run `npm run build` in `sdk/node` (protobufjs will load `.proto` at runtime by default; you can also precompile using `pbjs/pbts` if desired).

## Bus Choice
The helpers default to NATS. You can swap in Kafka or another pub/sub by replacing the bus adapter while keeping the same message shapes.

## Signing and Verification
- SDK helpers sign `BusPacket` envelopes when given a private key; pass a key to enforce verification, or omit it to send unsigned packets.
- Example ECDSA keypair (P-256):
  ```bash
  openssl ecparam -name prime256v1 -genkey -noout -out cap-priv.pem
  openssl ec -in cap-priv.pem -pubout -out cap-pub.pem
  ```
- Go: `client.Submit(..., privateKey)` and `worker.Worker{PrivateKey: ..., PublicKeys: map[senderID]pub}`.
- Node: provide PEM keys to `submitJob` / `WorkerConfig.privateKey` and `publicKeyMap`.
- Python: pass an `ec.EllipticCurvePrivateKey` to `submit_job` / `run_worker`; set `public_keys` for verification.
