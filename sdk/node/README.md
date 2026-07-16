# CAP Node/TypeScript SDK

Node/TypeScript SDK with NATS helpers and runtime-loaded CAP protobuf definitions.

## Install

```bash
npm install cap-sdk-node
```

The released npm artifact bundles the CAP schemas required by `loadRoot()`.
Application consumers do not need `protoc`, generated stubs, or a CAP repository clone.

The package is CommonJS and supports both CommonJS and native ESM consumers:

```js
const { connectNATS, loadRoot, submitJob } = require("cap-sdk-node");
```

```ts
import { connectNATS, loadRoot, submitJob } from "cap-sdk-node";
```

`loadRoot()` is asynchronous; await it before looking up protobuf types.

## Submit a job

```ts
import { connectNATS, submitJob } from "cap-sdk-node";

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
    "<PEM_PRIVATE_KEY>",
  );
  await nc.drain();
}

main().catch(console.error);
```

## Develop this SDK from source

```bash
cd sdk/node
npm ci
npm run build
node dist/sample-worker.js
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
import { Agent } from "cap-sdk-node";

const Input = z.object({ prompt: z.string() });
const Output = z.object({ summary: z.string() });

const agent = new Agent({ retries: 2 });

agent.job("job.summarize", Input, async (_ctx, data) => {
  return { summary: data.prompt.slice(0, 140) };
}, { outputSchema: Output });

agent.run().catch(console.error);
```

### Validation
The Node SDK provides opt-in validation helpers for CAP protobuf messages. Each function
returns an array of `ValidationError` objects; an empty array means the message is valid.

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

### Signature verification

Inbound envelope validation always runs before a worker handler. Signature verification
has two explicit modes on both `startWorker` and `Agent`:

- `publicKeyMap: undefined` (or omitting the option) is the sole legacy opt-out from
  signature verification.
- Supplying any map enables strict verification. The sender ID and signature must be
  nonempty, and the map must own a nonempty PEM public key for that sender. An empty map
  (`{}`) therefore denies every sender.

Unsigned packets, missing or unknown senders, malformed envelopes, and invalid or tampered
signatures are rejected before the registered handler is called. Do not use `{}` to request
unsigned compatibility.

### Environment
- `NATS_URL` (default `nats://127.0.0.1:4222`)
- `REDIS_URL` (default `redis://127.0.0.1:6379/0`)

### Shutdown and resource ownership

`startWorker()` is the low-level API and returns its NATS `Subscription`. The caller owns
that subscription and the supplied NATS connection; drain or unsubscribe the subscription
before draining the connection.

`Agent` owns the NATS connection and blob store configured for it, including injected
implementations. `await agent.close()` stops new intake, waits for in-flight handlers and
result publication, drains the NATS connection, and closes the blob store. Repeated `close()`
calls share the same shutdown operation.

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
- `createTestAgent(options?)` — returns `{ agent, bus, store }` pre-wired with
  `MockNatsConnection` and `InMemoryBlobStore`.
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

The runtime Agent and Worker accept a `Logger` for structured logging. Operational logs add
job, trace, topic, or sender context when available. Security-rejection logs intentionally
omit untrusted packet payloads and sender metadata. Pass a custom logger or omit it to use
the standard `console` logger:

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

Job-scoped runtime logs include `traceId`. `MetricsHook` receives the job, topic, duration,
status, error, and worker fields shown above; add trace correlation in your adapter if needed.

## Notes
- Subjects: `sys.job.submit`, `job.<pool>`, `sys.job.result`, `sys.heartbeat`.
- Protocol version: `1`.
- Field names use camelCase in protobufjs objects (for example, `jobId`, `contextPtr`,
  `resultPtr`, and `workerId`).
- The runtime transport covered by this SDK is NATS; publish protobuf-encoded CAP
  `BusPacket` envelopes on CAP subjects.
- Signing: `submitJob` and `startWorker` sign envelopes when given a PEM private key. Signatures use
  deterministic protobuf serialization (map entries ordered by key) for cross-SDK
  verification. Generate a P-256 keypair with:
  ```bash
  node -e "const {generateKeyPairSync}=require('crypto');const {privateKey,publicKey}=generateKeyPairSync('ec',{namedCurve:'prime256v1',publicKeyEncoding:{type:'spki',format:'pem'},privateKeyEncoding:{type:'pkcs8',format:'pem'}});console.log(privateKey);console.log(publicKey);"
  ```
- Pass `undefined` as the private key to `submitJob` to send unsigned envelopes.
