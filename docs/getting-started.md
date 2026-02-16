# Getting Started in 5 Minutes

Go from zero to a running CAP job in under 5 minutes. This guide covers the **low-level SDK** — no Redis needed.

## Prerequisites

1. **NATS server** — the message bus CAP uses by default:
   ```bash
   docker run -d --name nats -p 4222:4222 nats:latest
   ```

2. **Language toolchain** — pick your language:
   - **Go** 1.24+
   - **Python** 3.9+
   - **Node** 18+

---

## Go

### 1. Install

```bash
go get github.com/cordum-io/cap/v2@latest
```

### 2. Create the Worker

Create `worker/main.go`:

```go
package main

import (
	"context"
	"log"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/sdk/go/worker"
	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect("nats://127.0.0.1:4222")
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	w := &worker.Worker{
		NATS:     nc,
		Subject:  "job.echo",
		SenderID: "echo-worker",
		Handler: func(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			log.Printf("Received job %s", req.JobId)
			return &agentv1.JobResult{
				JobId:     req.JobId,
				Status:    agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
				ResultPtr: "redis://res/" + req.JobId,
				WorkerId:  "echo-worker",
			}, nil
		},
	}

	if err := w.Start(); err != nil {
		log.Fatal(err)
	}
	log.Println("Worker listening on job.echo")
	select {}
}
```

### 3. Create the Client

Create `client/main.go`:

```go
package main

import (
	"context"
	"log"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/sdk/go/client"
	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect("nats://127.0.0.1:4222")
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	req := &agentv1.JobRequest{
		JobId:      "my-echo-job",
		Topic:      "job.echo",
		ContextPtr: "redis://ctx/my-echo-job",
	}

	if err := client.Submit(context.Background(), nc, req, "my-trace-id", "my-client", nil); err != nil {
		log.Fatal(err)
	}
	log.Println("Job submitted!")
}
```

### 4. Run It

```bash
# Terminal 1 — start the worker
cd worker && go run .

# Terminal 2 — submit a job
cd client && go run .
```

You should see the worker log: `Received job my-echo-job`.

---

## Python

### 1. Install

```bash
pip install cap-sdk-python
```

### 2. Create the Worker

Create `worker.py`:

```python
import asyncio
from cap import worker
from cap.pb.cordum.agent.v1 import job_pb2


async def handle(req: job_pb2.JobRequest) -> job_pb2.JobResult:
    print(f"Received job {req.job_id}")
    return job_pb2.JobResult(
        job_id=req.job_id,
        status=job_pb2.JOB_STATUS_SUCCEEDED,
        result_ptr=f"redis://res/{req.job_id}",
        worker_id="echo-worker-py",
    )


async def main():
    await worker.run_worker(
        nats_url="nats://127.0.0.1:4222",
        subject="job.echo",
        handler=handle,
        sender_id="echo-worker-py",
    )


if __name__ == "__main__":
    asyncio.run(main())
```

### 3. Create the Client

Create `client.py`:

```python
import asyncio
import nats
from cap import client
from cap.pb.cordum.agent.v1 import job_pb2


async def main():
    nc = await nats.connect("nats://127.0.0.1:4222")

    req = job_pb2.JobRequest(
        job_id="my-echo-job",
        topic="job.echo",
        context_ptr="redis://ctx/my-echo-job",
    )
    await client.submit_job(nc, req, "my-trace-id", "my-client", None)
    print("Job submitted!")
    await nc.drain()


if __name__ == "__main__":
    asyncio.run(main())
```

### 4. Run It

```bash
# Terminal 1 — start the worker
python worker.py

# Terminal 2 — submit a job
python client.py
```

You should see the worker print: `Received job my-echo-job`.

---

## Node.js

### 1. Install

```bash
npm install cap-sdk-node
```

### 2. Create the Worker

Create `worker.js`:

```javascript
const { connectNATS, startWorker } = require("cap-sdk-node");

async function main() {
  const nc = await connectNATS({ url: "nats://127.0.0.1:4222" });

  await startWorker({
    nc,
    subject: "job.echo",
    senderId: "echo-worker-node",
    handler: async (req) => {
      console.log(`Received job ${req.jobId}`);
      return {
        jobId: req.jobId,
        status: "JOB_STATUS_SUCCEEDED",
        resultPtr: `redis://res/${req.jobId}`,
        workerId: "echo-worker-node",
      };
    },
  });

  console.log("Worker listening on job.echo");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
```

### 3. Create the Client

Create `client.js`:

```javascript
const { connectNATS, submitJob } = require("cap-sdk-node");

async function main() {
  const nc = await connectNATS({ url: "nats://127.0.0.1:4222" });

  await submitJob(
    nc,
    {
      jobId: "my-echo-job",
      topic: "job.echo",
      contextPtr: "redis://ctx/my-echo-job",
    },
    "my-trace-id",
    "my-client"
  );

  console.log("Job submitted!");
  await nc.drain();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
```

### 4. Run It

```bash
# Terminal 1 — start the worker
node worker.js

# Terminal 2 — submit a job
node client.js
```

You should see the worker log: `Received job my-echo-job`.

---

## What's Next

- **SDK READMEs** — deeper usage, signing, heartbeats, and configuration:
  - [Go SDK](../sdk/go/README.md)
  - [Python SDK](../sdk/python/README.md)
  - [Node SDK](../sdk/node/README.md)
- **High-Level Runtime** — typed handlers with automatic Redis pointer hydration (see examples in the [root README](../README.md#high-level-runtime-sdks))
- **Workflow Example** — parent/child job fan-out: [`examples/workflow-repo-review/`](../examples/workflow-repo-review/)
- **Protocol Spec** — the full CAP specification: [`spec/`](../spec/00-index.md)
- **Why CAP?** — how CAP compares to MCP and why it exists: [`docs/WHY_CAP.md`](WHY_CAP.md)
