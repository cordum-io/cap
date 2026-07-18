package worker

import (
	"context"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func TestManagedWorkerBuildersPassOwnValidator(t *testing.T) {
	_, natsURL := startTestNATS(t)
	nc := testNATSConn(t, natsURL)
	handshakes := make(chan *agentv1.BusPacket, 1)
	heartbeats := make(chan *agentv1.BusPacket, 4)

	handshakeSub, err := nc.Subscribe(capsdk.SubjectHandshake, captureManagedPacket(handshakes))
	if err != nil {
		t.Fatalf("subscribe handshake: %v", err)
	}
	defer handshakeSub.Unsubscribe()
	heartbeatSub, err := nc.Subscribe(capsdk.SubjectHeartbeat, captureManagedPacket(heartbeats))
	if err != nil {
		t.Fatalf("subscribe heartbeat: %v", err)
	}
	defer heartbeatSub.Unsubscribe()
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush subscriptions: %v", err)
	}

	managed, err := NewManagedWorker(ManagedConfig{
		NatsURL: natsURL, WorkerID: "worker-builder", Subjects: []string{"job.builder"}, HeartbeatEvery: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new managed worker: %v", err)
	}
	defer managed.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = managed.Run(ctx, func(_ context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
			return &agentv1.JobResult{JobId: req.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}, nil
		})
	}()

	packets := map[string]*agentv1.BusPacket{
		"legacy_handshake": awaitManagedPacket(t, handshakes),
		"heartbeat":        awaitManagedPacket(t, heartbeats),
	}
	for name, packet := range packets {
		t.Run(name, func(t *testing.T) {
			if err := capsdk.ValidateBusPacket(packet); err != nil {
				t.Fatalf("managed builder emitted invalid packet: %v", err)
			}
		})
	}
}

func captureManagedPacket(out chan<- *agentv1.BusPacket) nats.MsgHandler {
	return func(message *nats.Msg) {
		packet := &agentv1.BusPacket{}
		if proto.Unmarshal(message.Data, packet) == nil {
			select {
			case out <- packet:
			default:
			}
		}
	}
}

func awaitManagedPacket(t *testing.T, packets <-chan *agentv1.BusPacket) *agentv1.BusPacket {
	t.Helper()
	select {
	case packet := <-packets:
		return packet
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for managed worker packet")
		return nil
	}
}
