# Welcome to the Cordum Agent Protocol (CAP) Wiki!

The Cordum Agent Protocol (CAP) is a distributed job protocol for AI agents that communicates over a pub/sub bus. It standardizes envelopes, job messages, heartbeats, and workflow metadata so schedulers, workers, orchestrators, and gateways can interoperate.

This wiki provides a comprehensive guide to understanding, using, and contributing to the CAP project.

## Quick Links

*   **[Getting Started](./Getting-Started.md):** Your first stop for setting up your environment and running your first CAP application.
*   **[Protocol Specification](./Protocol-Specification.md):** A deep dive into the CAP protocol, its components, and its messages.
*   **[SDKs](./SDKs.md):** Detailed documentation for the official CAP SDKs.
*   **[Architecture](./Architecture.md):** An overview of the CAP system architecture.
*   **[Security](./Security.md):** Security best practices for building and deploying CAP applications.
*   **[Examples](./Examples.md):** Practical examples and tutorials to help you get started.
*   **[Contributing](./Contributing.md):** Learn how you can contribute to the CAP project.

## Release Status

<!-- cap-release:begin:release-status -->
- **Current verified published release:** 2.16.1 (tag `v2.16.1`, 2026-07-22, channel stable)
- **Wire protocol:** 1 (compatible range 1–1)
- **Wire schema:** 1.0.0
- **Specifications:** 20 normative documents
- **Release candidate (not published):** 2.17.0 (tag `v2.17.0`, channel stable)
<!-- cap-release:end -->

## SDK Support

<!-- cap-release:begin:sdk-table -->
| Component | Language | Kind | Tier | Registry | Package | Version | Toolchain |
| --- | --- | --- | --- | --- | --- | --- | --- |
| cap-go | go | sdk | stable | proxy.golang.org | github.com/cordum-io/cap/v2 | 2.16.1 | go1.25.12 |
| cap-node | node | sdk | stable | registry.npmjs.org | cap-sdk-node | 2.16.1 | node20 |
| cap-python | python | sdk | stable | pypi.org | cap-sdk-python | 2.16.1 | python3.11 |
| cordum-guard | python | extension | stable | pypi.org | cordum-guard | 2.16.1 | python3.11 |
| cap-cpp | cpp | sdk | experimental | - | - | - | - |
| cap-dotnet | dotnet | sdk | experimental | - | - | - | - |
| cap-java | java | sdk | experimental | - | - | - | - |
| cap-php | php | sdk | experimental | - | - | - | - |
| cap-ruby | ruby | sdk | experimental | - | - | - | - |
| cap-rust | rust | sdk | experimental | - | - | - | - |
<!-- cap-release:end -->
