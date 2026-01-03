# CAP Examples

This page provides practical examples and tutorials to help you get started with the coretex Agent Protocol (CAP).

## Simple Echo

The `simple-echo` example demonstrates a basic worker that receives a job and echoes back a "succeeded" status.

### Go Worker

Here is an example of a simple echo worker written in Go.

**`examples/simple-echo/go-worker/main.go`**
```go
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log"

	"github.com/coretexos/cap/v2/sdk/go/worker"
	agentv1 "github.com/coretexos/cap/v2/coretex/agent/v1"
	"github.com/nats-io/nats.go"
)

func myHandler(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
	log.Printf("Received job: %s", req.JobId)
	return &agentv1.JobResult{
		JobId:     req.JobId,
		Status:    agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
		ResultPtr: "redis://res/" + req.JobId,
		WorkerId:  "my-worker",
	}, nil
}

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	w := &worker.Worker{
		NATS:       nc,
		Subject:    "job.echo",
		Handler:    myHandler,
		SenderID:   "my-worker",
		PrivateKey: priv, // signs outgoing JobResult envelopes
	}

	if err := w.Start(); err != nil {
		log.Fatal(err)
	}

	log.Println("Worker listening on job.echo")
	select {} // block forever
}
```

### Go Client

Here is an example of a client that submits a job to the echo worker.

**`examples/simple-echo/go-client/main.go`**
```go
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log"

	"github.com/coretexos/cap/v2/sdk/go/client"
	agentv1 "github.com/coretexos/cap/v2/coretex/agent/v1"
	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	req := &agentv1.JobRequest{
		JobId:      "my-echo-job",
		Topic:      "job.echo",
		ContextPtr: "redis://ctx/my-echo-job",
	}

	if err := client.Submit(context.Background(), nc, req, "my-trace-id", "my-client", priv); err != nil {
		log.Fatal(err)
	}

log.Println("Job submitted")
}
```

### Python Worker

**`examples/simple-echo/python-worker/main.py`**
```python
import asyncio

from cap import worker
from cap.pb.coretex.agent.v1 import job_pb2


async def handle(req: job_pb2.JobRequest) -> job_pb2.JobResult:
    return job_pb2.JobResult(
        job_id=req.job_id,
        status=job_pb2.JOB_STATUS_SUCCEEDED,
        result_ptr=f"redis://res/{req.job_id}",
        worker_id="worker-echo-py",
    )


async def main():
    await worker.run_worker(
        nats_url="nats://127.0.0.1:4222",
        subject="job.echo",
        handler=handle,
        sender_id="worker-echo-py",
    )


if __name__ == "__main__":
    asyncio.run(main())
```

### Python Client

**`examples/simple-echo/python-client/main.py`**
```python
import asyncio

import nats
from cap import client
from cap.pb.coretex.agent.v1 import job_pb2


async def main():
    nc = await nats.connect("nats://127.0.0.1:4222")
    req = job_pb2.JobRequest(
        job_id="job-echo-1",
        topic="job.echo",
        context_ptr="redis://ctx/job-echo-1",
    )
    await client.submit_job(nc, req, "trace-1", "client-py", None)
    await nc.drain()


if __name__ == "__main__":
    asyncio.run(main())
```

### Node Worker

**`examples/simple-echo/node-worker/main.js`**
```js
const { connectNATS, startWorker } = require("../../../sdk/node/dist");

async function main() {
  const nc = await connectNATS({ url: "nats://127.0.0.1:4222" });
  await startWorker({
    nc,
    subject: "job.echo",
    senderId: "worker-echo-node",
    handler: async (req) => ({
      jobId: req.jobId,
      status: "JOB_STATUS_SUCCEEDED",
      resultPtr: `redis://res/${req.jobId}`,
      workerId: "worker-echo-node",
    }),
  });
  console.log("echo worker listening on job.echo");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
```

### Node Client

**`examples/simple-echo/node-client/main.js`**
```js
const { connectNATS, submitJob } = require("../../../sdk/node/dist");

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
    "client-node"
  );
  await nc.drain();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
```
