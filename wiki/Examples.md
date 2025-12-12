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
	"log"

	"github.com/coretexos/cap/sdk/go/worker"
	agentv1 "github.com/coretexos/cap/go/coretex/agent/v1"
	"github.com/nats-io/nats.go"
)

func myHandler(ctx context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
	log.Printf("Received job: %s", req.JobId)
	return &agentv1.JobResult{
		Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
	}, nil
}

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	w := &worker.Worker{
		NATS:       nc,
		Subject:    "job.echo",
		Handler:    myHandler,
		SenderID:   "my-worker",
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
	"log"

	"github.com/coretexos/cap/sdk/go/client"
	agentv1 "github.com/coretexos/cap/go/coretex/agent/v1"
	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	req := &agentv1.JobRequest{
		JobId: "my-echo-job",
		Topic: "job.echo",
	}

	if err := client.Submit(context.Background(), nc, req, "my-trace-id", "my-client", nil); err != nil {
		log.Fatal(err)
	}

	log.Println("Job submitted")
}
```
