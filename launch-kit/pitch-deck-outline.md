# CAP Pitch Deck Outline

1) Title: “coretex Agent Protocol (CAP) — Open Control Plane for Distributed AI”
2) Problem: Multi-agent systems are bespoke; no standard for jobs, safety, heartbeats, workflows.
3) Solution: CAP — bus-native protocol with envelopes, pointers, safety, and state machine.
4) Why now: Explosion of AI agents; MCP/tool calling stops at single-model; infra needs a control-plane standard.
5) Product: Spec + Protos; Reference Implementation = coretexOS; SDKs (Go/Python/TS) roadmap.
6) Architecture diagram: client -> gateway -> sys.job.submit -> scheduler -> job.<pool> -> worker -> result_ptr -> scheduler -> client; safety + memory in-path.
7) Differentiation vs MCP/alternatives: distributed, safety-first, pointer-based, workflow-aware, observable.
8) Market: AI infra, agent frameworks, MLOps, ops automation.
9) Traction/plan: v1 spec, examples, early adopters, ecosystem integrations (LangChain, GitHub Actions, K8s ops).
10) Business: coretexOS Cloud as managed CAP control plane; support/certification.
11) Roadmap: SDKs, RFCs (v2: streaming, vector pointers, enclaves), CNCF Sandbox target.
12) Call to action: adopt spec, build workers, join working group.
