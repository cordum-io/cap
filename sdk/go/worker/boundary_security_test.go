package worker

import (
	"context"
	"sync/atomic"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLowLevelPacketBuildersRejectInvalidEnvelopes(t *testing.T) {
	tests := []struct {
		name  string
		build func() ([]byte, error)
	}{
		{"heartbeat missing worker identity", func() ([]byte, error) {
			return HeartbeatPayload("", "pool", 0, 1, 0)
		}},
		{"progress missing sender identity", func() ([]byte, error) {
			return ProgressPayload("", "job-1", "step-1", 1, "starting")
		}},
		{"cancel missing sender identity", func() ([]byte, error) {
			return CancelPayload("", "job-1", "stop", "operator")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.build(); err == nil {
				t.Fatal("invalid builder input must be rejected before marshal")
			}
		})
	}
}

func TestLowLevelWorkerRejectsInvalidEnvelopeBeforeHandler(t *testing.T) {
	conn := &mockNATSConn{published: make(chan *nats.Msg, 1), subscriptions: map[string]nats.MsgHandler{}}
	var calls atomic.Int32
	worker := &Worker{
		NATS: conn, Subject: "job.secure", SenderID: "worker-secure",
		Handler: func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error) {
			calls.Add(1)
			return &agentv1.JobResult{Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		},
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	packet := validLowLevelJobPacket()
	packet.ProtocolVersion = capsdk.DefaultProtocolVersion + 1
	raw, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal invalid packet: %v", err)
	}
	conn.subscriptions[worker.Subject](&nats.Msg{Data: raw})
	if calls.Load() != 0 {
		t.Fatalf("invalid envelope reached handler %d time(s)", calls.Load())
	}
}

func validLowLevelJobPacket() *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId: "trace-secure", SenderId: "scheduler-1",
		ProtocolVersion: capsdk.DefaultProtocolVersion, CreatedAt: timestamppb.Now(),
		Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{
			JobId: "job-1", Topic: "job.secure",
		}},
	}
}
