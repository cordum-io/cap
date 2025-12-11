# Heartbeats and Pools

## Heartbeat Semantics
- `worker_id`: unique identifier of the emitting worker/agent.
- `region`: optional location hint (zone/cluster/region).
- `type`: capability class (e.g., `cpu`, `gpu`, `cpu-tools`).
- `cpu_load` / `gpu_utilization`: load indicators (0–100).
- `active_jobs`: current in-flight jobs on this worker.
- `capabilities`: freeform strings describing skills or tools.
- `pool`: logical pool name this worker serves (often matches bus subject prefix, e.g., `job.chat.simple`).
- `max_parallel_jobs`: advertised concurrency limit for scheduling.

Schedulers can use these fields to pick the least-loaded worker in a pool, to segment by region, or to enforce capacity limits.

## Pools
- A pool is a logical grouping of workers behind a subject such as `job.chat.simple`, `job.code.llm`, `job.repo.scan`.
- Heartbeats advertise pool membership; schedulers aggregate heartbeats by pool to make routing decisions.
- CAP does not prescribe a scheduling algorithm. Common patterns include least-active-jobs, weighted random, or region-aware selection.

## Recommended Usage
- Send heartbeats on a fixed interval (e.g., every 5s) to a subject like `sys.heartbeat.<pool>` or `sys.heartbeat.*`.
- Consumers should tolerate late or missing heartbeats and expire workers after a grace period.
- Include capabilities only if they are meaningful for routing; avoid unbounded lists.
