# SDK Comparison Matrix

CAP ships three protocol SDKs (Go, Node, Python) that speak directly to the NATS bus. **Guard** is a separate extension for safety policy enforcement via HTTP, not a protocol SDK.

## Support Matrix

<!-- cap-release:begin:sdk-table -->
| Component | Language | Kind | Tier | Registry | Package | Version | Toolchain |
| --- | --- | --- | --- | --- | --- | --- | --- |
| cap-go | go | sdk | stable | proxy.golang.org | github.com/cordum-io/cap/v2 | 2.14.0 | go1.25.12 |
| cap-node | node | sdk | stable | registry.npmjs.org | cap-sdk-node | 2.14.0 | node20 |
| cap-python | python | sdk | stable | pypi.org | cap-sdk-python | 2.14.0 | python3.11 |
| cordum-guard | python | extension | stable | pypi.org | cordum-guard | 2.14.0 | python3.11 |
| cap-cpp | cpp | sdk | experimental | - | - | - | - |
| cap-dotnet | dotnet | sdk | experimental | - | - | - | - |
| cap-java | java | sdk | experimental | - | - | - | - |
| cap-php | php | sdk | experimental | - | - | - | - |
| cap-ruby | ruby | sdk | experimental | - | - | - | - |
| cap-rust | rust | sdk | experimental | - | - | - | - |
<!-- cap-release:end -->

## At a Glance

| Feature | Go | Node/TypeScript | Python | Guard (Python) |
|---|---|---|---|---|
| **Use case** | Backend services | Backend / serverless | ML / data pipelines | Safety layer for any Python function |
| **Transport** | NATS | NATS | NATS | HTTP (Cordum gateway) |
| **Async model** | Goroutines | async/await | asyncio | sync + async |
| **Validation** | Struct tags | Zod schemas | Pydantic models | Risk tags / labels |
| **Runtime layer** | Yes | Yes | Yes | N/A (decorator) |
| **ManagedWorker** | Yes | No | No | N/A |
| **Message validation** | No | Yes (opt-in) | No | N/A |
| **TLS helpers** | Yes (env-based) | No | Redis only | N/A |
| **Signing** | ECDSA P-256 | ECDSA P-256 | ECDSA P-256 | API key |
| **Blob store** | Redis | Redis | Redis | N/A |
| **Install** | `go get github.com/cordum-io/cap/v2` | `npm install cap-sdk-node` | `pip install cap-sdk-python` | `pip install cordum-guard` |
| **Framework integrations** | — | — | — | LangChain, LlamaIndex |

## When to Use Each SDK

**Go SDK** — Best for high-throughput backend workers and orchestrators where goroutine-based concurrency and static typing matter.

**Node/TypeScript SDK** — Best for serverless functions, API backends, and teams already in the Node ecosystem. Uses Zod for runtime schema validation.

**Python SDK** — Best for ML/data pipelines, research workloads, and teams using asyncio. Uses Pydantic for typed input/output models.

**Guard SDK** — Best for adding safety governance to *existing* Python code without rewriting it. Wraps any function with a `@guard` decorator that evaluates Cordum safety policies before execution.

> **Guard is NOT a replacement for the Python CAP SDK.** The Guard SDK enforces safety policies via HTTP to the Cordum gateway — it does not connect to NATS, submit jobs, or run workers. Use the Python CAP SDK for CAP protocol operations and Guard for safety decoration on top of existing code.

## Detailed Comparison

### CAP Protocol SDKs (Go, Node, Python)

All three CAP SDKs share the same architecture:

- **Low-level layer**: Direct NATS pub/sub with protobuf `BusPacket` envelopes. Worker/client helpers handle serialization, signing, and subject routing.
- **High-level runtime**: Typed job handlers with automatic Redis pointer hydration. Register handlers by topic and the runtime manages NATS subscriptions, context loading, and result storage.
- **Signing**: Optional ECDSA P-256 envelope signing for authenticity. Pass `nil`/`None`/`undefined` to skip signing in development.
- **Heartbeats**: Built-in liveness and capacity reporting on `sys.heartbeat`.

### Guard SDK (Python)

The Guard SDK is architecturally different:

- **No NATS connection** — communicates with the Cordum gateway over HTTP.
- **No job lifecycle** — does not submit or process CAP jobs.
- **Decorator-based** — wraps existing functions with `@guard(client, policy="...", risk_tags=[...])`.
- **Safety decisions** — allow, deny, require human approval, or throttle.
- **Framework wrappers** — `CordumToolGuard` for LangChain and LlamaIndex tool wrapping.

## SDK READMEs

- [Go SDK](../sdk/go/README.md)
- [Node/TypeScript SDK](../sdk/node/README.md)
- [Python SDK](../sdk/python/README.md)
- [Guard SDK](../sdk/python-guard/README.md)
