# CAP Node/TypeScript SDK

Node/TS SDK with NATS helpers. Uses `protobufjs` to load CAP proto definitions at runtime.

## Quick Start
1. Install deps:
   ```bash
   cd sdk/node
   npm install
   ```
2. Run the sample worker:
   ```bash
   npm run build
   node dist/sample-worker.js
   ```

3. Submit a job (client):
   ```ts
   import { connectNATS } from "./bus";
   import { submitJob } from "./client";

   async function main() {
     const nc = await connectNATS({ url: "nats://127.0.0.1:4222" });
    await submitJob(
      nc,
      {
        jobId: "job-echo-1",
        topic: "job.echo",
        contextPtr: "redis://ctx/job-echo-1",
       },
      "trace-1",
      "client-node",
      "<PEM_PRIVATE_KEY>"
    );
     await nc.drain();
   }

   main().catch(console.error);
   ```

## Files
- `src/protos.ts` — loads CAP protos via protobufjs.
- `src/bus.ts` — NATS connector.
- `src/worker.ts` — worker skeleton.
- `src/client.ts` — submission helper.
- `src/sample-worker.ts` — minimal echo worker example.

## Runtime (High-Level SDK)
The runtime hides NATS/Redis plumbing and gives you typed handlers.

```ts
import { z } from "zod";
import { Agent } from "./runtime";

const Input = z.object({ prompt: z.string() });
const Output = z.object({ summary: z.string() });

const agent = new Agent({ retries: 2 });

agent.job("job.summarize", Input, async (_ctx, data) => {
  return { summary: data.prompt.slice(0, 140) };
}, { outputSchema: Output });

agent.run().catch(console.error);
```

### Environment
- `NATS_URL` (default `nats://127.0.0.1:4222`)
- `REDIS_URL` (default `redis://127.0.0.1:6379/0`)

## Observability

### Structured Logging
The runtime Agent and Worker accept a `Logger` for structured logging. All log calls include contextual fields (`jobId`, `traceId`, `topic`, `senderId`). Pass a custom logger or leave undefined for the built-in JSON logger:

```ts
import { Agent, Logger } from "cap-sdk-node";

const agent = new Agent({
  logger: {
    info(msg, fields) { console.info(JSON.stringify({ level: "info", msg, ...fields })); },
    warn(msg, fields) { console.warn(JSON.stringify({ level: "warn", msg, ...fields })); },
    error(msg, fields) { console.error(JSON.stringify({ level: "error", msg, ...fields })); },
  },
});
```

### MetricsHook
Implement `MetricsHook` to integrate with Prometheus, OpenTelemetry, or any metrics system:

```ts
import { MetricsHook } from "cap-sdk-node";

interface MetricsHook {
  onJobReceived(jobId: string, topic: string): void;
  onJobCompleted(jobId: string, durationMs: number, status: string): void;
  onJobFailed(jobId: string, errorMsg: string): void;
  onHeartbeatSent(workerId: string): void;
}
```

The default is `noopMetrics` (zero overhead). Example integration:

```ts
import { Agent } from "cap-sdk-node";

const metrics: MetricsHook = {
  onJobReceived(jobId, topic) { jobCounter.inc({ topic }); },
  onJobCompleted(jobId, durationMs, status) { durationHist.observe({ status }, durationMs); },
  onJobFailed(jobId, errorMsg) { failCounter.inc(); },
  onHeartbeatSent(workerId) {},
};

const agent = new Agent({ metrics });
```

The `traceId` is propagated through all log and metrics calls for distributed tracing correlation.

## Notes
- Subjects: `sys.job.submit`, `job.<pool>`, `sys.job.result`, `sys.heartbeat`.
- Protocol version: `1`.
- Field names use camelCase in protobufjs objects (e.g., `jobId`, `contextPtr`, `resultPtr`, `workerId`).
- Swap `bus.ts` for another transport if needed; keep message encoding via protobufjs or precompiled static modules (`pbjs/pbts`).
- Signing: `submitJob` and `startWorker` sign envelopes when given a PEM private key; set `publicKeyMap` to verify incoming packets. Signatures use deterministic protobuf serialization (map entries ordered by key) for cross-SDK verification. Generate a P-256 keypair with:
  ```bash
  node -e "const {generateKeyPairSync}=require('crypto');const {privateKey,publicKey}=generateKeyPairSync('ec',{namedCurve:'prime256v1',publicKeyEncoding:{type:'spki',format:'pem'},privateKeyEncoding:{type:'pkcs8',format:'pem'}});console.log(privateKey);console.log(publicKey);"
  ```
- If you do not want signature verification, omit `publicKeyMap` in `startWorker`.
- Pass `undefined` as the private key to `submitJob` to send unsigned envelopes.
