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
