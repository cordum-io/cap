# Node.js SDK

The Node.js SDK for CAP is suitable for building web-based applications and services that interact with the CAP bus. It is written in TypeScript and provides type safety for your CAP applications.

## Installation

```bash
cd sdk/node
npm install
```

## Client

The `src/client.ts` module provides functions for submitting jobs.

### `submitJob`

The `submitJob` function sends a `JobRequest` to the bus.

```typescript
export async function submitJob(
  nc: NatsConnection,
  jobRequest: Record<string, unknown>,
  traceId: string,
  senderId: string,
  privateKey?: string
)
```
Pass `undefined` as the private key to send unsigned envelopes.
Use camelCase field names (`jobId`, `contextPtr`, `resultPtr`, `workerId`) when building objects for protobufjs.

**Example:**

```typescript
import { connect, NatsConnection } from "nats";
import { submitJob } from "./client";
import * as crypto from "crypto";

async function main() {
  const nc = await connect({ servers: "nats://localhost:4222" });

  const { privateKey } = crypto.generateKeyPairSync("ec", {
    namedCurve: "prime256v1",
    privateKeyEncoding: { type: "pkcs8", format: "pem" },
    publicKeyEncoding: { type: "spki", format: "pem" },
  });

  const req = {
    jobId: "my-test-job",
    topic: "job.echo",
    contextPtr: "redis://ctx/my-test-job",
  };

  await submitJob(nc, req, "my-trace-id", "my-client", privateKey);

  console.log("Job submitted");
  await nc.close();
}

main().catch((err) => console.error(err));
```

## Worker

The `src/worker.ts` module provides tools for building workers.

### `startWorker`

The `startWorker` function starts a worker that listens for jobs on a given subject.

```typescript
export async function startWorker(cfg: WorkerConfig): Promise<Subscription>
```

**`WorkerConfig` Interface:**

```typescript
export interface WorkerConfig {
  nc: NatsConnection;
  subject: string;
  queue?: string;
  handler: Handler;
  publicKeyMap?: { [senderId: string]: string };
  privateKey?: string;
  senderId: string;
}
```

**Example:**

```typescript
import { connect, NatsConnection } from "nats";
import { startWorker } from "./worker";
import * as crypto from "crypto";

async function main() {
  const nc = await connect({ servers: "nats://localhost:4222" });

  const { privateKey } = crypto.generateKeyPairSync("ec", {
    namedCurve: "prime256v1",
    privateKeyEncoding: { type: "pkcs8", format: "pem" },
    publicKeyEncoding: { type: "spki", format: "pem" },
  });

  await startWorker({
    nc,
    subject: "job.echo",
    senderId: "my-worker",
    privateKey,
    handler: async (req: any) => {
      console.log(`Received job: ${req.jobId}`);
      return {
        jobId: req.jobId,
        status: "JOB_STATUS_SUCCEEDED",
        resultPtr: `redis://res/${req.jobId}`,
        workerId: "my-worker",
      };
    },
  });

  console.log("Worker listening on job.echo");
}

main().catch((err) => console.error(err));
```
