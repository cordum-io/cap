# Why MCP Is Not Enough for Distributed AI — Introducing CAP

MCP standardizes tool calls for a single model. It does not cover the realities of multi-agent clusters: scheduling, safety, heartbeats, memory pointers, and state reconciliation. CAP fills that gap.

## The Gaps MCP Does Not Solve
- **Scheduling**: MCP assumes one runtime; CAP defines job submission, priorities, and pool routing.
- **Safety**: MCP lacks a policy hook; CAP mandates a Safety Kernel (allow/deny/human/throttle).
- **Memory**: MCP inlines payloads; CAP uses `context_ptr` and `result_ptr` to keep buses lean.
- **State**: MCP has no lifecycle; CAP standardizes job states and transitions.
- **Workflows**: MCP is single-call; CAP supports orchestrated parent/child jobs.

## What CAP Adds
- BusPacket envelope with trace, sender, and negotiated protocol version.
- JobRequest/JobResult with workflow metadata.
- Heartbeats for liveness, capacity, and pool membership.
- Transport profile for NATS/Kafka and competing consumers.
- Security/observability guidance baked into the spec.

## Quick Start
1) Write input to memory, get a `context_ptr`.  
2) Publish `BusPacket{JobRequest}` to `sys.job.submit`.  
3) Scheduler calls Safety, dispatches to `job.<pool>`.  
4) Worker reads pointer, executes, writes `result_ptr`, publishes `JobResult`.  
5) Client reads result from memory.

## Takeaway
Use MCP inside a worker if you like, but use CAP for the cluster control plane. Without CAP, multi-agent systems reinvent brittle, proprietary protocols.
