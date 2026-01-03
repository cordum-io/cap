# Go SDK

The Go SDK for CAP is the reference implementation and provides a comprehensive set of tools for building CAP applications in Go.

## Installation

```bash
go get github.com/coretexos/cap/v2/sdk/go
```

## Client

The `client` package provides functions for submitting jobs to the CAP bus.

### `Submit`

The `Submit` function sends a `JobRequest` to the bus.

```go
func Submit(ctx context.Context, pub Publisher, req *agentv1.JobRequest, traceID, senderID string, privateKey *ecdsa.PrivateKey) error
```
Pass `nil` as the private key to send an unsigned envelope.

**Example:**

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

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	req := &agentv1.JobRequest{
		JobId:      "my-test-job",
		Topic:      "job.echo",
		ContextPtr: "redis://ctx/my-test-job",
	}

	if err := client.Submit(context.Background(), nc, req, "my-trace-id", "my-client", privateKey); err != nil {
		log.Fatal(err)
	}

	log.Println("Job submitted")
}
```

## Worker

The `worker` package provides tools for building workers that can process jobs.

### `Worker` Struct

The `Worker` struct defines a CAP worker.

```go
type Worker struct {
	NATS        NATSConn
	Subject     string
	Handler     Handler
	PublicKeys  map[string]*ecdsa.PublicKey
	PrivateKey  *ecdsa.PrivateKey
	SenderID    string
}
```

### `Handler` Function

The `Handler` function is where you define your job processing logic.

```go
type Handler func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error)
```

**Example:**

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

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	w := &worker.Worker{
		NATS:       nc,
		Subject:    "job.echo",
		Handler:    myHandler,
		SenderID:   "my-worker",
		PrivateKey: privateKey,
	}

	if err := w.Start(); err != nil {
		log.Fatal(err)
	}
}
```
