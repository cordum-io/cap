package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log"

	agentv1 "github.com/coretexos/cap/v2/go/coretex/agent/v1"
	"github.com/coretexos/cap/sdk/go/client"
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
