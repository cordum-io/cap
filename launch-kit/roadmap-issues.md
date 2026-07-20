# CAP GitHub Roadmap (Issues/Milestones Draft)

## Milestone: CAP v1.0 (tag)
- [ ] Finalize spec wording (RFC 2119 audit)
- [ ] Freeze protobuf field numbers for v1.0
- [ ] Add Python/Go/TS worker quickstarts to `examples/`
- [ ] Add SDK skeletons: cap-go, cap-python, cap-node (separate repos)
- [ ] Publish launch announcement and blog series
- [ ] Tag v1.0 release

## Milestone: SDKs + Integrations
- [ ] Publish cap-go crate/module with worker skeleton and gateway client
- [ ] Publish cap-python package (pypi)
- [ ] Publish cap-node package (npm)
- [ ] LangChain plugin for CAP as a tool-calling backend
- [ ] GitHub Actions / GitLab Runner CAP workers
- [ ] K8s ops worker (kubectl/helm) with safety guardrails

## Milestone: SDK Feature Parity & Language Coverage
Tracked in Moe epic `epic-c0e0dff4`.

### Heartbeat Helper Parity (HIGH)
- [ ] Add heartbeat helpers to Python SDK (HeartbeatPayload constructors, emit_heartbeat, heartbeat_loop)
- [ ] Add heartbeat helpers to Node SDK (same API surface, TypeScript idiomatic)

### Progress/Cancel Emission Helpers (HIGH)
- [ ] Add progress/cancel emission helpers to Go SDK (EmitProgress, EmitCancel)
- [ ] Add progress/cancel emission helpers to Python SDK (emit_progress, emit_cancel)
- [ ] Add progress/cancel emission helpers to Node SDK (emitProgress, emitCancel)

### C++ SDK Hardening (HIGH/MEDIUM)
- [ ] Add ECDSA packet signing/verification to C++ SDK (OpenSSL)
- [ ] Add deterministic serialization to C++ SDK (CodedOutputStream deterministic mode)
- [ ] Add runtime/middleware/metrics layer to C++ SDK (Agent lifecycle, middleware chain, MetricsHook)

### New Language SDKs (MEDIUM/LOW)
Each includes: runtime, worker, client, bus transport, signing, deterministic serialization, heartbeat/progress/cancel helpers, middleware, metrics, validation, testing utilities, generated protobuf types, and tests.
- [ ] Create Java SDK (io.cordum.cap.agent.v1, Maven/Gradle, nats.java)
- [ ] Create Rust SDK (Cargo crate, tokio, async-nats, prost)
- [ ] Create C#/.NET SDK (Cordum.Agent.V1, NuGet, NATS.Net, .NET 8+)
- [ ] Create PHP SDK (cordum\Agent\V1, Composer, php-nats)
- [ ] Create Ruby SDK (Cordum::Agent::V1, Gem, nats-pure)

## Milestone: Community + RFCs
- [x] Stand up `rfcs/` with template and process (see rfcs/README.md)
- [ ] Shepherd the first substantive RFCs (memory v2, agent discovery, streaming)
- [ ] Grow the maintainer roster toward the neutral-governance trigger (see MAINTAINERS.md); no working group or TSC exists today
- [ ] Add CAP to relevant awesome-lists
- [ ] Publish comparison paper MCP vs CAP

## Milestone: Observability + OTel
- [ ] Define CAP span/metric conventions for OpenTelemetry
- [ ] Provide OTel helpers for Go/Python/TS SDKs
- [ ] Sample dashboards (Grafana) for job states/latency/heartbeat health

## Milestone: Neutral-governance readiness (future; not scheduled, no foundation affiliation)
- [x] Governance doc, maintainer ladder, DCO, COI, code of conduct
- [ ] Security model and threat assessment
- [ ] Verifiable public adopters and independent implementations (tracked by governance/readiness.json; currently BLOCKED)
