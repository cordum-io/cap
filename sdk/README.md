# CAP SDKs

This folder contains production-ready SDKs for CAP. Each SDK is bus-agnostic but ships NATS helpers as a sane default.

## Available SDKs
- `go/` — Go SDK (reference implementation) with NATS worker/client, heartbeat helpers, signing, runtime, middleware, metrics.
- `python/` — Python SDK with asyncio NATS helpers, signing, runtime, middleware, metrics.
- `node/` — Node/TypeScript SDK using NATS, protobufjs, signing, runtime, middleware, metrics.
- `cpp/` — C++ SDK with a bus interface and helper wrappers. Signing, deterministic serialization, and runtime/middleware are in progress.

## Planned SDKs
- `java/` — Java SDK (`io.cordum.cap.agent.v1`). Maven/Gradle, nats.java, protobuf-java.
- `rust/` — Rust SDK. Cargo crate, tokio, async-nats, prost.
- `dotnet/` — C#/.NET SDK (`Cordum.Agent.V1`). NuGet, NATS.Net, Google.Protobuf, .NET 8+.
- `php/` — PHP SDK (`cordum\Agent\V1`). Composer, php-nats, protobuf-php.
- `ruby/` — Ruby SDK (`Cordum::Agent::V1`). Gem, nats-pure, google-protobuf.

High-level runtime layers (typed handlers + Redis pointer hydration) live alongside each SDK under `sdk/go/runtime`, `sdk/python/cap/runtime.py`, and `sdk/node/src/runtime.ts`.

## Examples
- `examples/simple-echo/` contains Go/Python/Node client+worker pairs aligned with these SDKs.

## Proto Stubs
Generate language stubs from `proto/` before building:
- Go: `./tools/make_protos.sh` (outputs to `/cordum/agent/v1`) or run the commands in `sdk/go/README.md`.
- Python: `./tools/make_protos.sh` (set `CAP_RUN_PY=1`; outputs to `/python`) or the `grpc_tools.protoc` command in `sdk/python/README.md` to place stubs under `sdk/python/cap/pb`.
- C++: `./tools/make_protos.sh` (outputs to `/cpp`) or let `sdk/cpp` CMake generate on build.
- Node: `./tools/make_protos.sh` (outputs CommonJS stubs to `/node`), or load `.proto` at runtime via protobufjs (see `sdk/node/src/protos.ts`). Install deps then run `npm run build` in `sdk/node`.
- Java: `./tools/make_protos.sh` (set `CAP_RUN_JAVA=1`) or use the Maven/Gradle protobuf plugin.
- C#: `./tools/make_protos.sh` (set `CAP_RUN_CSHARP=1`) or use the `Grpc.Tools` NuGet package during `dotnet build`.
- PHP: `./tools/make_protos.sh` (set `CAP_RUN_PHP=1`).
- Ruby: `./tools/make_protos.sh` (set `CAP_RUN_RUBY=1`).
- Rust: Uses `prost-build` in `build.rs` — proto compilation happens automatically during `cargo build`.

## Bus Choice
The helpers default to NATS. You can swap in Kafka or another pub/sub by replacing the bus adapter while keeping the same message shapes.

## Signing and Verification
- SDK helpers sign `BusPacket` envelopes when given a private key. In Go,
  omitting keys is an error unless `AllowUnsigned` is explicit (or the caller
  uses `SubmitUnsigned`); unsigned transport must never result from a missing
  key by accident.
- Example ECDSA keypair (P-256):
  ```bash
  openssl ecparam -name prime256v1 -genkey -noout -out cap-priv.pem
  openssl ec -in cap-priv.pem -pubout -out cap-pub.pem
  ```
- Go: `client.Submit(..., privateKey)` and `worker.Worker{PrivateKey: ..., PublicKeys: map[senderID]pub}`.
- Node: provide PEM keys to `submitJob` / `WorkerConfig.privateKey` and `publicKeyMap`.
- Python: pass an `ec.EllipticCurvePrivateKey` to `submit_job` / `run_worker`; set `public_keys` for verification.
