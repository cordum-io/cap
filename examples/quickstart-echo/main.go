// Command quickstart-echo is a standalone, self-contained CAP echo round-trip
// that runs against a plain NATS server using only PUBLIC cap-sdk imports. It is
// the reference for the Go registry quickstart snippet.
//
// Topology: this uses the DIRECT-POOL "development lab" wiring — the worker
// subscribes to the submit subject directly, with no Scheduler or Safety Kernel
// in the path. PRODUCTION deployments must submit through the governed
// Scheduler/Safety path (see docs/reference.md), not this direct wiring.
//
// It exits 0 only after correlating the terminal JobResult by job id and
// asserting SUCCEEDED; it exits non-zero on timeout, no worker, or error.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/cordum-io/cap/v2/sdk/go/client"
	"github.com/cordum-io/cap/v2/sdk/go/worker"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "quickstart-echo:", err)
		os.Exit(1)
	}
	fmt.Println("quickstart-echo: OK")
}

func run() error {
	url := os.Getenv("CAP_NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}
	nc, err := nats.Connect(url, nats.Timeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect NATS at %s: %w", url, err)
	}
	defer nc.Close()

	// Dev-lab worker: consume submitted jobs directly and echo the job id.
	// AllowUnsigned is the explicit, intentional opt-out required since the
	// CAP-PRODUCTION signing default landed (v2.15.0): this quickstart's
	// direct-pool wiring has no scheduler/safety-kernel path to hold or
	// verify signing keys, matching its own "development lab" doc comment.
	w := &worker.Worker{
		NATS:          nc,
		Subject:       capsdk.SubjectSubmit,
		SenderID:      "echo-worker",
		AllowUnsigned: true,
		Handler: func(_ context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{
				JobId:     req.JobId,
				Status:    agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
				ResultPtr: "echo://" + req.JobId,
				WorkerId:  "echo-worker",
			}, nil
		},
	}
	if err := w.Start(); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	// Observe results before submitting; Flush is the server-side barrier that
	// guarantees the SUB is registered before the job is published.
	jobID := "echo-1"
	results := make(chan *agentv1.JobResult, 1)
	if _, err := nc.Subscribe(capsdk.SubjectResult, func(msg *nats.Msg) {
		var pkt agentv1.BusPacket
		if proto.Unmarshal(msg.Data, &pkt) == nil {
			if r := pkt.GetJobResult(); r != nil && r.JobId == jobID {
				results <- r
			}
		}
	}); err != nil {
		return fmt.Errorf("subscribe results: %w", err)
	}
	if err := nc.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	req := &agentv1.JobRequest{JobId: jobID, Topic: "job.echo"}
	if err := client.SubmitUnsigned(context.Background(), nc, req, jobID, "echo-client"); err != nil {
		return fmt.Errorf("submit: %w", err)
	}

	select {
	case r := <-results:
		if r.Status != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
			return fmt.Errorf("job %s ended %s, want SUCCEEDED", r.JobId, r.Status)
		}
		if r.ResultPtr != "echo://"+jobID {
			return fmt.Errorf("job %s payload %q, want echo://%s", r.JobId, r.ResultPtr, jobID)
		}
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for JobResult for %s (no worker?)", jobID)
	}
}
