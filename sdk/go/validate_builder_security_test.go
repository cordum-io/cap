package capsdk_test

import (
	"context"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	capclient "github.com/cordum-io/cap/v2/sdk/go/client"
	capworker "github.com/cordum-io/cap/v2/sdk/go/worker"
	"google.golang.org/protobuf/proto"
)

type builderCapture struct {
	data []byte
}

func (capture *builderCapture) Publish(_ string, data []byte) error {
	capture.data = append([]byte(nil), data...)
	return nil
}

func TestPublicGoBuildersPassOwnValidator(t *testing.T) {
	builders := []struct {
		name  string
		build func() ([]byte, error)
	}{
		{name: "job_request", build: buildJobRequest},
		{name: "heartbeat", build: func() ([]byte, error) {
			return capworker.HeartbeatPayload("worker-1", "default", 0, 1, 0)
		}},
		{name: "progress", build: func() ([]byte, error) {
			return capworker.ProgressPayload("worker-1", "job-1", "step-1", 50, "working")
		}},
		{name: "cancel", build: func() ([]byte, error) {
			return capworker.CancelPayload("scheduler-1", "job-1", "operator request", "operator-1")
		}},
	}

	for _, builder := range builders {
		t.Run(builder.name, func(t *testing.T) {
			data, err := builder.build()
			if err != nil {
				t.Fatalf("build packet: %v", err)
			}
			packet := &agentv1.BusPacket{}
			if err := proto.Unmarshal(data, packet); err != nil {
				t.Fatalf("decode packet: %v", err)
			}
			if err := capsdk.ValidateBusPacket(packet); err != nil {
				t.Fatalf("builder emitted invalid packet: %v", err)
			}
		})
	}
}

func buildJobRequest() ([]byte, error) {
	capture := &builderCapture{}
	err := capclient.Submit(context.Background(), capture, &agentv1.JobRequest{
		JobId: "job-1",
		Topic: "job.security",
	}, "trace-1", "client-1", nil)
	return capture.data, err
}
