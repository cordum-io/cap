package runtime

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHandleMessageRejectsInvalidSecurityEnvelopeBeforeHandler(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Agent, *agentv1.BusPacket)
	}{
		{name: "missing trace", mutate: func(_ *Agent, p *agentv1.BusPacket) { p.TraceId = "" }},
		{name: "missing sender", mutate: func(_ *Agent, p *agentv1.BusPacket) { p.SenderId = "" }},
		{name: "missing timestamp", mutate: func(_ *Agent, p *agentv1.BusPacket) { p.CreatedAt = nil }},
		{name: "unsupported version", mutate: func(_ *Agent, p *agentv1.BusPacket) { p.ProtocolVersion = 2 }},
		{name: "missing signature", mutate: func(a *Agent, _ *agentv1.BusPacket) {
			a.PublicKeys = map[string]*ecdsa.PublicKey{"scheduler-1": &key.PublicKey}
		}},
		{name: "invalid signature", mutate: func(a *Agent, p *agentv1.BusPacket) {
			a.PublicKeys = map[string]*ecdsa.PublicKey{"scheduler-1": &key.PublicKey}
			p.Signature = []byte("not-a-signature")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agent, spec, calls := newBoundaryFixture(t)
			packet := validBoundaryPacket()
			test.mutate(agent, packet)
			data, marshalErr := proto.Marshal(packet)
			if marshalErr != nil {
				t.Fatalf("marshal packet: %v", marshalErr)
			}
			agent.handleMessage(&nats.Msg{Data: data}, spec)
			if calls.Load() != 0 {
				t.Fatalf("invalid packet reached handler %d time(s)", calls.Load())
			}
		})
	}
}

func TestTrustModeInboundRequiresPinnedSchedulerIdentityAndSignature(t *testing.T) {
	for _, mode := range []HandshakeMode{HandshakeModeWarn, HandshakeModeEnforce} {
		t.Run(string(mode), func(t *testing.T) {
			testTrustModeInboundBoundary(t, mode)
		})
	}
}

func TestTrustModeInboundRequiresLiveSessionOnlyInEnforce(t *testing.T) {
	for _, test := range []struct {
		mode HandshakeMode
		want int32
	}{
		{mode: HandshakeModeWarn, want: 1},
		{mode: HandshakeModeEnforce, want: 0},
	} {
		t.Run(string(test.mode), func(t *testing.T) {
			agent, spec, calls := newBoundaryFixture(t)
			_, trustBus := newTrustTestAgent(t, test.mode)
			agent.HandshakeMode, agent.WorkerTrust = test.mode, trustBus.config
			packet := validBoundaryPacket()
			packet.SenderId = trustBus.config.ExpectedSchedulerID
			mustSignBoundaryPacket(t, packet, trustBus.schedulerKey)
			data, err := proto.Marshal(packet)
			if err != nil {
				t.Fatal(err)
			}
			agent.handleMessage(&nats.Msg{Data: data}, spec)
			if calls.Load() != test.want {
				t.Fatalf("handler calls=%d, want %d", calls.Load(), test.want)
			}
		})
	}
}

func testTrustModeInboundBoundary(t *testing.T, mode HandshakeMode) {
	t.Helper()
	_, trustBus := newTrustTestAgent(t, mode)
	attacker := testP256Key(t)
	tests := []struct {
		name   string
		mutate func(*Agent, *agentv1.BusPacket)
		want   int32
	}{
		{name: "unsigned expected sender"},
		{name: "wrong sender", mutate: func(_ *Agent, p *agentv1.BusPacket) {
			p.SenderId = "scheduler-attacker"
			mustSignBoundaryPacket(t, p, trustBus.schedulerKey)
		}},
		{name: "legacy key map cannot override trust pins", mutate: func(a *Agent, p *agentv1.BusPacket) {
			a.PublicKeys = map[string]*ecdsa.PublicKey{p.GetSenderId(): &attacker.PublicKey}
			mustSignBoundaryPacket(t, p, attacker)
		}},
		{name: "pinned scheduler", mutate: func(_ *Agent, p *agentv1.BusPacket) {
			mustSignBoundaryPacket(t, p, trustBus.schedulerKey)
		}, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agent, spec, calls := newBoundaryFixture(t)
			agent.HandshakeMode, agent.WorkerTrust = mode, trustBus.config
			agent.setSession("active-session", time.Now().Add(time.Hour))
			packet := validBoundaryPacket()
			packet.SenderId = trustBus.config.ExpectedSchedulerID
			if test.mutate != nil {
				test.mutate(agent, packet)
			}
			data, err := proto.Marshal(packet)
			if err != nil {
				t.Fatal(err)
			}
			agent.handleMessage(&nats.Msg{Data: data}, spec)
			if calls.Load() != test.want {
				t.Fatalf("handler calls=%d, want %d", calls.Load(), test.want)
			}
		})
	}
}

func mustSignBoundaryPacket(t *testing.T, packet *agentv1.BusPacket, key *ecdsa.PrivateKey) {
	t.Helper()
	if err := capsdk.SignPacket(packet, key); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeLegacyHandshakeBuilderPassesValidator(t *testing.T) {
	bus := newMockNATS()
	agent := &Agent{NATS: bus, SenderID: "worker-builder", Logger: silentLogger()}
	Register(agent, "job.builder", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })

	agent.publishHandshake()
	message, ok := bus.lastPublished()
	if !ok {
		t.Fatal("legacy handshake was not published")
	}
	var packet agentv1.BusPacket
	if err := proto.Unmarshal(message.data, &packet); err != nil {
		t.Fatalf("decode handshake: %v", err)
	}
	if err := capsdk.ValidateBusPacket(&packet); err != nil {
		t.Fatalf("legacy handshake builder emitted invalid packet: %v", err)
	}
}

func TestRuntimeJobResultBuilderPassesValidator(t *testing.T) {
	bus := newMockNATS()
	agent := &Agent{NATS: bus, SenderID: "worker-builder", Logger: silentLogger()}
	ctx := Context{Packet: &agentv1.BusPacket{TraceId: "trace-builder"}, Logger: silentLogger()}
	result := &agentv1.JobResult{JobId: "job-builder", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED, WorkerId: "worker-builder"}

	agent.publishResult(ctx, result)
	message, ok := bus.lastPublished()
	if !ok {
		t.Fatal("job result was not published")
	}
	var packet agentv1.BusPacket
	if err := proto.Unmarshal(message.data, &packet); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if err := capsdk.ValidateBusPacket(&packet); err != nil {
		t.Fatalf("job result builder emitted invalid packet: %v", err)
	}
}

func TestRuntimeJobResultRejectsInvalidEnvelopeBeforePublish(t *testing.T) {
	bus := newMockNATS()
	agent := &Agent{NATS: bus, SenderID: "worker-builder", Logger: silentLogger()}
	ctx := Context{Packet: &agentv1.BusPacket{}, Logger: silentLogger()}
	result := &agentv1.JobResult{
		JobId: "job-builder", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED, WorkerId: "worker-builder",
	}
	agent.publishResult(ctx, result)
	if _, ok := bus.lastPublished(); ok {
		t.Fatal("invalid outbound envelope was published")
	}
}

func newBoundaryFixture(t *testing.T) (*Agent, handlerSpec, *atomic.Int32) {
	t.Helper()
	store := NewInMemoryBlobStore()
	payload, _ := json.Marshal(map[string]string{"prompt": "hello"})
	if err := store.Set(context.Background(), "ctx:security", payload); err != nil {
		t.Fatalf("seed context: %v", err)
	}
	calls := &atomic.Int32{}
	agent := &Agent{NATS: newMockNATS(), Store: store, SenderID: "worker-1", Logger: silentLogger(), Metrics: NoopMetrics}
	Register(agent, "job.security", func(_ Context, _ struct{ Prompt string }) (struct{}, error) {
		calls.Add(1)
		return struct{}{}, nil
	})
	return agent, agent.handlers["job.security"], calls
}

func validBoundaryPacket() *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId:         "trace-security",
		SenderId:        "scheduler-1",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{
			JobId: "job-security", Topic: "job.security", ContextPtr: "redis://ctx:security",
		}},
	}
}
