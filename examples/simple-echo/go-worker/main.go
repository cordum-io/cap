package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/sdk/go/worker"
	"github.com/nats-io/nats.go"
)

const (
	workerID      = "simple-echo-go-worker"
	workerSubject = "job.echo"
)

func main() {
	if err := run(); err != nil {
		log.Printf("CAP simple echo worker failed: %v", err)
		os.Exit(1)
	}
}

func run() (err error) {
	nc, err := nats.Connect(natsURL(), nats.Timeout(5*time.Second), nats.DrainTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer func() {
		if cleanupErr := cleanupNATS(nc); err == nil {
			err = cleanupErr
		}
	}()
	w := &worker.Worker{
		NATS:     nc,
		Subject:  workerSubject,
		Handler:  echo,
		SenderID: workerID,
	}
	if err := w.Start(); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	if err := nc.FlushTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("propagate worker subscription: %w", err)
	}
	log.Print("WARNING: unsigned local-development worker; no authenticated identity or policy enforcement")
	log.Printf("CAP_SIMPLE_ECHO_WORKER_READY subject=%s", workerSubject)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return nil
}

func echo(_ context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
	log.Printf("received job %q", req.GetJobId())
	return &agentv1.JobResult{
		JobId:     req.GetJobId(),
		Status:    agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
		ResultPtr: "demo://result/" + req.GetJobId(),
		WorkerId:  workerID,
	}, nil
}

func natsURL() string {
	if value := strings.TrimSpace(os.Getenv("CAP_NATS_URL")); value != "" {
		return value
	}
	return nats.DefaultURL
}

func cleanupNATS(nc *nats.Conn) error {
	if err := nc.Drain(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
		nc.Close()
		return fmt.Errorf("drain NATS: %w", err)
	}
	nc.Close()
	return nil
}
