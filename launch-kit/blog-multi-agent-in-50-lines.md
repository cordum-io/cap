# Build a Multi-Agent System in ~50 Lines with CAP

CAP reduces multi-agent plumbing to a few standard messages. Here’s the minimal path.

## Ingredients
- A bus (NATS recommended).
- Redis (or any memory store) for `context_ptr` / `result_ptr`.
- CAP protobufs for your language.

## Steps (Python-ish pseudocode)
```python
# client
write(ctx_ptr, {"prompt": "Hello"})               # ctx_ptr = redis://ctx/job-1
publish("sys.job.submit", BusPacket(job_request={
    "job_id": "job-1",
    "topic": "job.echo",
    "priority": JOB_PRIORITY_INTERACTIVE,
    "context_ptr": ctx_ptr,
}))

# worker
msg = subscribe("job.echo").next()
ctx = read(msg.job_request.context_ptr)
write(res_ptr, {"echo": ctx["prompt"]})
publish("sys.job.result", BusPacket(job_result={
    "job_id": msg.job_request.job_id,
    "status": JOB_STATUS_SUCCEEDED,
    "result_ptr": res_ptr,
    "worker_id": "worker-echo-1",
}))

# client reads
result = read(res_ptr)
```

## Why It Works
- BusPacket gives you tracing and versioning.
- Pointers keep the bus small and secure.
- Heartbeats let schedulers route to healthy pools.
- Safety hook lets you block/allow before dispatch.

## Next
- Swap `job.echo` for a real tool agent.
- Add an orchestrator that emits child jobs and aggregates.
- Layer observability with `trace_id` and job states.
