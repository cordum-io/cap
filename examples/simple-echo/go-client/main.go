package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultResultTimeout = 15 * time.Second
	directSubject        = "job.echo"
	resultTimeoutEnv     = "CAP_RESULT_TIMEOUT_SECONDS"
	successMarker        = "CAP_SIMPLE_ECHO_SUCCESS"
)

func main() {
	if err := run(); err != nil {
		log.Printf("CAP simple echo failed: %v", err)
		os.Exit(1)
	}
}

func run() (err error) {
	timeout, err := resultTimeout()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	nc, err := nats.Connect(natsURL(), nats.Timeout(timeout), nats.DrainTimeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	sub, err := nc.SubscribeSync(capsdk.SubjectResult)
	if err != nil {
		nc.Close()
		return fmt.Errorf("subscribe for results: %w", err)
	}
	defer func() {
		if cleanupErr := cleanupNATS(nc, sub); err == nil {
			err = cleanupErr
		}
	}()
	if err := flushBefore(nc, deadline); err != nil {
		return err
	}
	req, packet, err := newRequest()
	if err != nil {
		return err
	}
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		return fmt.Errorf("encode request packet: %w", err)
	}
	// DEVELOPMENT ONLY: this raw publish bypasses the Gateway, Scheduler,
	// Safety Kernel, authorization, policy, retries, and durable state.
	log.Print("WARNING: direct local-development publish; correlation is not authentication")
	if err := nc.Publish(req.GetTopic(), data); err != nil {
		return fmt.Errorf("publish request: %w", err)
	}
	if err := flushBefore(nc, deadline); err != nil {
		return err
	}
	result, err := waitForResult(sub, deadline, packet.GetTraceId(), req.GetJobId())
	if err != nil {
		return err
	}
	if result.GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		return fmt.Errorf("job %q finished with %s: %s", req.GetJobId(), result.GetStatus(), result.GetErrorMessage())
	}
	fmt.Println(successMarker)
	return nil
}

func newRequest() (*agentv1.JobRequest, *agentv1.BusPacket, error) {
	suffix, err := randomSuffix()
	if err != nil {
		return nil, nil, fmt.Errorf("create correlation IDs: %w", err)
	}
	jobID := "simple-echo-go-" + suffix
	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      directSubject,
		ContextPtr: "demo://context/" + jobID,
	}
	if err := validateDirectRequest(req); err != nil {
		return nil, nil, err
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-simple-echo-go-" + suffix,
		SenderId:        "simple-echo-go-client",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return nil, nil, fmt.Errorf("validate request packet: %w", err)
	}
	return req, packet, nil
}

func validateDirectRequest(req *agentv1.JobRequest) error {
	if err := capsdk.ValidateJobRequest(req); err != nil {
		return fmt.Errorf("validate request: %w", err)
	}
	subject := strings.TrimSpace(req.GetTopic())
	if subject == "" || strings.ContainsAny(subject, "*>\t\r\n ") {
		return fmt.Errorf("direct subject %q must be nonblank and contain no wildcards or whitespace", req.GetTopic())
	}
	for _, token := range strings.Split(subject, ".") {
		if token == "" {
			return fmt.Errorf("direct subject %q must contain no empty tokens", req.GetTopic())
		}
	}
	return nil
}

func waitForResult(sub *nats.Subscription, deadline time.Time, traceID, jobID string) (*agentv1.JobResult, error) {
	for {
		wait, err := remaining(deadline)
		if err != nil {
			return nil, fmt.Errorf("wait for job %q: %w", jobID, err)
		}
		msg, err := sub.NextMsg(wait)
		if errors.Is(err, nats.ErrTimeout) {
			return nil, fmt.Errorf("wait for job %q: result timeout", jobID)
		}
		if err != nil {
			return nil, fmt.Errorf("read result: %w", err)
		}
		var packet agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &packet); err != nil {
			continue
		}
		result := packet.GetJobResult()
		if result == nil || packet.GetTraceId() != traceID || result.GetJobId() != jobID {
			continue
		}
		if err := capsdk.ValidateBusPacket(&packet); err != nil {
			return nil, fmt.Errorf("matching result packet is invalid: %w", err)
		}
		if err := capsdk.ValidateJobResult(result); err != nil {
			return nil, fmt.Errorf("matching job result is invalid: %w", err)
		}
		switch result.GetStatus() {
		case agentv1.JobStatus_JOB_STATUS_PENDING,
			agentv1.JobStatus_JOB_STATUS_SCHEDULED,
			agentv1.JobStatus_JOB_STATUS_DISPATCHED,
			agentv1.JobStatus_JOB_STATUS_RUNNING:
			continue
		case agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
			agentv1.JobStatus_JOB_STATUS_FAILED,
			agentv1.JobStatus_JOB_STATUS_CANCELLED,
			agentv1.JobStatus_JOB_STATUS_DENIED,
			agentv1.JobStatus_JOB_STATUS_TIMEOUT,
			agentv1.JobStatus_JOB_STATUS_FAILED_RETRYABLE,
			agentv1.JobStatus_JOB_STATUS_FAILED_FATAL:
			return result, nil
		default:
			return nil, fmt.Errorf("matching job result has unknown status %d", result.GetStatus())
		}
	}
}

func resultTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(resultTimeoutEnv))
	if raw == "" {
		return defaultResultTimeout, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", resultTimeoutEnv, raw)
	}
	return time.Duration(seconds) * time.Second, nil
}

func natsURL() string {
	if value := strings.TrimSpace(os.Getenv("CAP_NATS_URL")); value != "" {
		return value
	}
	return nats.DefaultURL
}

func randomSuffix() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func flushBefore(nc *nats.Conn, deadline time.Time) error {
	wait, err := remaining(deadline)
	if err != nil {
		return err
	}
	if err := nc.FlushTimeout(wait); err != nil {
		return fmt.Errorf("flush NATS: %w", err)
	}
	return nil
}

func remaining(deadline time.Time) (time.Duration, error) {
	wait := time.Until(deadline)
	if wait <= 0 {
		return 0, errors.New("result deadline exceeded")
	}
	return wait, nil
}

func cleanupNATS(nc *nats.Conn, sub *nats.Subscription) error {
	var cleanupErr error
	if err := sub.Unsubscribe(); err != nil && !errors.Is(err, nats.ErrBadSubscription) {
		cleanupErr = fmt.Errorf("unsubscribe results: %w", err)
	}
	if err := nc.Drain(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("drain NATS: %w", err))
	}
	nc.Close()
	return cleanupErr
}
