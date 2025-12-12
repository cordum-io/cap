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
