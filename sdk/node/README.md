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

### Validation
The Node SDK provides opt-in validation helpers for CAP protobuf messages. Each function returns an array of `ValidationError` objects; an empty array means the message is valid.

- `validateJobRequest(msg: any): ValidationError[]`
- `validateJobResult(msg: any): ValidationError[]`
- `validateBusPacket(msg: any): ValidationError[]` (recursively validates payloads)

```ts
import { validateJobRequest } from "cap-sdk-node";

const errors = validateJobRequest(decodedMsg);
if (errors.length > 0) {
  console.error("Invalid job request:", errors);
}
```

### Environment
- `NATS_URL` (default `nats://127.0.0.1:4222`)
- `REDIS_URL` (default `redis://127.0.0.1:6379/0`)

## Testing

The `testing` module lets you test handlers without running NATS or Redis.

```ts
import { testHandler } from "cap-sdk-node";

it("runs echo handler without NATS", async () => {
  const result = await testHandler(
    async (_ctx, data: { prompt: string }) => ({ echo: data.prompt }),
    { prompt: "hello" },
    { topic: "job.echo" }
  );
  expect(result.status).to.equal(5); // JOB_STATUS_SUCCEEDED
});
```

- `testHandler(handler, input, options?)` — runs a single handler invocation and returns the result.
- `createTestAgent(options?)` — returns `{ agent, bus, store }` pre-wired with `MockNatsConnection` + `InMemoryBlobStore`.
- `MockNatsConnection` — in-memory NATS mock for custom test setups.

## Middleware

Add cross-cutting concerns (logging, auth, metrics) without modifying handlers:

```ts
import { loggingMiddleware, Middleware } from "cap-sdk-node";

// Built-in logging middleware
agent.use(loggingMiddleware());

// Custom middleware
const timing: Middleware = async (ctx, next) => {
  const start = Date.now();
  const result = await next();
  console.log(`job ${ctx.jobId} took ${Date.now() - start}ms`);
  return result;
};
agent.use(timing);
```

Middleware executes in registration order (FIFO). Each can inspect context,
measure timing, or short-circuit by returning without calling `next`.

## Generating API Docs

Generate HTML API reference locally using [TypeDoc](https://typedoc.org/):

```bash
npm run docs
```

Output is written to `docs/` (gitignored). Open `docs/index.html` to browse.

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
