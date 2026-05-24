package runtime

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type mockNATS struct {
	mu        sync.Mutex
	subs      map[string]nats.MsgHandler
	published []publishedMsg
}

type publishedMsg struct {
	subject string
	data    []byte
}

func newMockNATS() *mockNATS {
	return &mockNATS{subs: make(map[string]nats.MsgHandler)}
}

func (m *mockNATS) Publish(subject string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, publishedMsg{subject: subject, data: data})
	return nil
}

func (m *mockNATS) QueueSubscribe(subj, _queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[subj] = cb
	return &nats.Subscription{}, nil
}

func (m *mockNATS) send(subj string, data []byte) {
	m.mu.Lock()
	cb := m.subs[subj]
	m.mu.Unlock()
	if cb != nil {
		cb(&nats.Msg{Subject: subj, Data: data})
	}
}

func (m *mockNATS) lastPublished() (publishedMsg, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.published) == 0 {
		return publishedMsg{}, false
	}
	return m.published[len(m.published)-1], true
}

func TestRuntimeSuccess(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	jobID := "job-1"
	ctxKey := "ctx:" + jobID
	payload, _ := json.Marshal(map[string]string{"prompt": "hello"})
	_ = store.Set(nilContext(), ctxKey, payload)

	agent := &Agent{
		NATS:     mock,
		Store:    store,
		SenderID: "worker-1",
	}

	Register(agent, "job.test", func(_ Context, input struct{ Prompt string }) (map[string]string, error) {
		return map[string]string{"summary": input.Prompt}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      "job.test",
		ContextPtr: "redis://" + ctxKey,
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-1",
		SenderId:        "client-1",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	mock.send("job.test", data)

	msg, ok := mock.lastPublished()
	if !ok {
		t.Fatal("no result published")
	}
	if msg.subject != capsdk.SubjectResult {
		t.Fatalf("unexpected subject: %s", msg.subject)
	}

	var out agentv1.BusPacket
	if err := proto.Unmarshal(msg.data, &out); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if out.GetJobResult().GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("unexpected status: %v", out.GetJobResult().GetStatus())
	}

	resultData, err := store.Get(nilContext(), "res:"+jobID)
	if err != nil {
		t.Fatalf("result missing: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(resultData, &decoded); err != nil {
		t.Fatalf("decode result payload: %v", err)
	}
	if decoded["summary"] != "hello" {
		t.Fatalf("unexpected summary: %s", decoded["summary"])
	}
}

func TestRuntimeInvalidJSON(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	jobID := "job-2"
	ctxKey := "ctx:" + jobID
	_ = store.Set(nilContext(), ctxKey, []byte("{bad json"))

	agent := &Agent{
		NATS:     mock,
		Store:    store,
		SenderID: "worker-2",
	}

	Register(agent, "job.validate", func(_ Context, input struct{ Prompt string }) (map[string]string, error) {
		return map[string]string{"summary": input.Prompt}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      "job.validate",
		ContextPtr: "redis://" + ctxKey,
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-2",
		SenderId:        "client-2",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	mock.send("job.validate", data)

	msg, ok := mock.lastPublished()
	if !ok {
		t.Fatal("no result published")
	}
	var out agentv1.BusPacket
	if err := proto.Unmarshal(msg.data, &out); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if out.GetJobResult().GetStatus() != agentv1.JobStatus_JOB_STATUS_FAILED {
		t.Fatalf("unexpected status: %v", out.GetJobResult().GetStatus())
	}
}

func TestRuntimeRetries(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	jobID := "job-3"
	ctxKey := "ctx:" + jobID
	payload, _ := json.Marshal(map[string]string{"prompt": "retry"})
	_ = store.Set(nilContext(), ctxKey, payload)

	agent := &Agent{
		NATS:     mock,
		Store:    store,
		SenderID: "worker-3",
		Retries:  1,
	}

	attempts := 0
	Register(agent, "job.retry", func(_ Context, input struct{ Prompt string }) (map[string]string, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("boom")
		}
		return map[string]string{"summary": input.Prompt}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      "job.retry",
		ContextPtr: "redis://" + ctxKey,
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-3",
		SenderId:        "client-3",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	mock.send("job.retry", data)

	msg, ok := mock.lastPublished()
	if !ok {
		t.Fatal("no result published")
	}
	var out agentv1.BusPacket
	if err := proto.Unmarshal(msg.data, &out); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if out.GetJobResult().GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("unexpected status: %v", out.GetJobResult().GetStatus())
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestAgentStartPublishesHandshake(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	key := mustGenerateECDSAKey(t)

	agent := &Agent{
		NATS:       mock,
		Store:      store,
		SenderID:   "worker-handshake",
		PrivateKey: key,
	}

	Register(agent, "job.handshake", func(_ Context, _ struct{}) (struct{}, error) {
		return struct{}{}, nil
	})
	Register(agent, "job.handshake.secondary", func(_ Context, _ struct{}) (struct{}, error) {
		return struct{}{}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	msg, ok := mock.lastPublished()
	if !ok {
		t.Fatal("no handshake published")
	}
	if msg.subject != capsdk.SubjectHandshake {
		t.Fatalf("unexpected subject: %s", msg.subject)
	}

	var packet agentv1.BusPacket
	if err := proto.Unmarshal(msg.data, &packet); err != nil {
		t.Fatalf("decode handshake: %v", err)
	}

	handshake := packet.GetHandshake()
	if handshake == nil {
		t.Fatal("missing handshake payload")
	}
	if packet.GetSenderId() != "worker-handshake" {
		t.Fatalf("sender_id = %q, want %q", packet.GetSenderId(), "worker-handshake")
	}
	if handshake.GetComponentId() != "worker-handshake" {
		t.Fatalf("component_id = %q, want %q", handshake.GetComponentId(), "worker-handshake")
	}
	if handshake.GetRole() != agentv1.ComponentRole_COMPONENT_ROLE_WORKER {
		t.Fatalf("role = %v, want %v", handshake.GetRole(), agentv1.ComponentRole_COMPONENT_ROLE_WORKER)
	}
	if len(handshake.GetSupportedVersions()) != 1 || handshake.GetSupportedVersions()[0] != capsdk.DefaultProtocolVersion {
		t.Fatalf("supported_versions = %v, want [%d]", handshake.GetSupportedVersions(), capsdk.DefaultProtocolVersion)
	}
	if !handshake.GetCapabilities()["job.handshake"] {
		t.Fatalf("expected job.handshake capability, got %v", handshake.GetCapabilities())
	}
	if !handshake.GetCapabilities()["job.handshake.secondary"] {
		t.Fatalf("expected job.handshake.secondary capability, got %v", handshake.GetCapabilities())
	}
	if got, want := handshake.GetReadyTopics(), []string{"job.handshake", "job.handshake.secondary"}; len(got) != len(want) {
		t.Fatalf("ready_topics len = %d, want %d (%v)", len(got), len(want), got)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ready_topics[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	}
	if len(packet.GetSignature()) == 0 {
		t.Fatal("expected handshake signature")
	}
	if err := capsdk.VerifyPacketSignature(&packet, &key.PublicKey); err != nil {
		t.Fatalf("verify handshake signature: %v", err)
	}
}

func TestAgentHandshakeCarriesSanitizedAgentName(t *testing.T) {
	mock := newMockNATS()
	store := NewInMemoryBlobStore()

	agent := &Agent{
		NATS:     mock,
		Store:    store,
		SenderID: "worker-named",
		// Padded/dirty label proves the SDK sanitizes before advertising it.
		AgentName: "  Claude Code\n— Billing  ",
	}
	Register(agent, "job.named", func(_ Context, _ struct{}) (struct{}, error) {
		return struct{}{}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	msg, ok := mock.lastPublished()
	if !ok {
		t.Fatal("no handshake published")
	}
	var packet agentv1.BusPacket
	if err := proto.Unmarshal(msg.data, &packet); err != nil {
		t.Fatalf("decode handshake: %v", err)
	}
	handshake := packet.GetHandshake()
	if handshake == nil {
		t.Fatal("missing handshake payload")
	}
	if handshake.GetAgentName() != "Claude Code — Billing" {
		t.Fatalf("handshake agent_name = %q, want sanitized %q", handshake.GetAgentName(), "Claude Code — Billing")
	}
}

func mustGenerateECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestPingRedis_BadURL(t *testing.T) {
	if err := PingRedis("not-a-url"); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestValidateRedisURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// Valid auth
		{"password only", "redis://:password@localhost:6379", true},
		{"user and password", "redis://user:pass@localhost:6379", true},
		{"user only", "redis://admin@localhost:6379", true},
		{"rediss scheme", "rediss://user:pass@host:6379", true},
		{"url-encoded password", "redis://user:p%40ss%3Dw0rd@host:6379", true},
		{"with db", "redis://user:pass@host:6379/3", true},
		{"empty password explicit", "redis://user:@host:6379", true},
		{"ipv4", "redis://user:pass@192.168.1.1:6379", true},

		// No auth
		{"no auth", "redis://localhost:6379", false},
		{"no auth with db", "redis://127.0.0.1:6379/0", false},
		{"no auth with query", "redis://host:6379/0?timeout=5s", false},

		// @ in non-auth position (the bug this fixes)
		{"at in query param", "redis://host:6379/0?key=val@ue", false},

		// Bad scheme
		{"http scheme", "http://user:pass@host:6379", false},
		{"empty string", "", false},
		{"garbage", "not-a-url", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateRedisURL(tt.url); got != tt.want {
				t.Errorf("ValidateRedisURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func nilContext() context.Context {
	return context.Background()
}
