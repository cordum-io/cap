# CAP SDKs

The Cordum Agent Protocol (CAP) has official SDKs for several popular programming languages. These SDKs provide a convenient way to interact with the CAP bus and build CAP-enabled applications.

## Available SDKs

*   **[Go](./Go-SDK.md):** The Go SDK is the reference implementation and is the most feature-complete.
*   **[Python](./Python-SDK.md):** The Python SDK is ideal for AI/ML applications and scripting.
*   **[Node.js](./Node-js-SDK.md):** The Node.js SDK is suitable for building web-based applications and services that interact with the CAP bus.
*   **[C++](./C---SDK.md):** The C++ SDK is designed for high-performance applications where low-level control is required.

## Planned SDKs

The following SDKs are planned under the **SDK Feature Parity & Language Coverage** epic. Proto options already declare packages for each language.

*   **Java** (`io.cordum.cap.agent.v1`) — Maven/Gradle, nats.java, protobuf-java
*   **Rust** — Cargo crate, tokio, async-nats, prost
*   **C#/.NET** (`Cordum.Agent.V1`) — NuGet, NATS.Net, Google.Protobuf, .NET 8+
*   **PHP** (`cordum\Agent\V1`) — Composer, php-nats, protobuf-php
*   **Ruby** (`Cordum::Agent::V1`) — Gem, nats-pure, google-protobuf

## SDK Feature Matrix

Each SDK should provide the following capabilities:

| Feature | Go | Python | Node | C++ | Java | Rust | C# | PHP | Ruby |
|---------|:--:|:------:|:----:|:---:|:----:|:----:|:--:|:---:|:----:|
| Bus transport (NATS) | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Planned | Planned | Planned | Planned | Planned |
| Worker / Client | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Planned | Planned | Planned | Planned | Planned |
| Packet signing/verification | :white_check_mark: | :white_check_mark: | :white_check_mark: | Planned | Planned | Planned | Planned | Planned | Planned |
| Deterministic serialization | :white_check_mark: | :white_check_mark: | :white_check_mark: | Planned | Planned | Planned | Planned | Planned | Planned |
| Heartbeat helpers | :white_check_mark: | Planned | Planned | Planned | Planned | Planned | Planned | Planned | Planned |
| Progress/cancel helpers | Planned | Planned | Planned | Planned | Planned | Planned | Planned | Planned | Planned |
| High-level runtime (Agent) | :white_check_mark: | :white_check_mark: | :white_check_mark: | Planned | Planned | Planned | Planned | Planned | Planned |
| Middleware chain | :white_check_mark: | :white_check_mark: | :white_check_mark: | Planned | Planned | Planned | Planned | Planned | Planned |
| Metrics hooks | :white_check_mark: | :white_check_mark: | :white_check_mark: | Planned | Planned | Planned | Planned | Planned | Planned |
| Validation | :white_check_mark: | :white_check_mark: | :white_check_mark: | — | Planned | Planned | Planned | Planned | Planned |
| Testing utilities | :white_check_mark: | :white_check_mark: | :white_check_mark: | — | Planned | Planned | Planned | Planned | Planned |

Click on the links above to find detailed documentation for each available SDK.
