package runtime

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMiddlewareExecutionOrder(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	jobID := "job-mw-order"
	ctxKey := "ctx:" + jobID
	payload, _ := json.Marshal(map[string]string{"prompt": "hello"})
	_ = store.Set(nilContext(), ctxKey, payload)

	agent := &Agent{
		HandshakeMode: HandshakeModeOff,
		NATS:          mock,
		Store:         store,
		SenderID:      "worker-mw",
	}

	var order []string

	agent.Use(func(next HandlerFunc) HandlerFunc {
		return func(ctx Context, data any) (any, error) {
			order = append(order, "A-before")
			out, err := next(ctx, data)
			order = append(order, "A-after")
			return out, err
		}
	})
	agent.Use(func(next HandlerFunc) HandlerFunc {
		return func(ctx Context, data any) (any, error) {
			order = append(order, "B-before")
			out, err := next(ctx, data)
			order = append(order, "B-after")
			return out, err
		}
	})

	Register(agent, "job.mw", func(_ Context, input struct{ Prompt string }) (map[string]string, error) {
		order = append(order, "handler")
		return map[string]string{"summary": input.Prompt}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      "job.mw",
		ContextPtr: "redis://" + ctxKey,
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-mw",
		SenderId:        "client-mw",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	mock.send("job.mw", data)

	expected := []string{"A-before", "B-before", "handler", "B-after", "A-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], expected[i])
		}
	}
}

func TestMiddlewareShortCircuit(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	jobID := "job-mw-short"
	ctxKey := "ctx:" + jobID
	payload, _ := json.Marshal(map[string]string{"prompt": "hello"})
	_ = store.Set(nilContext(), ctxKey, payload)

	agent := &Agent{
		HandshakeMode: HandshakeModeOff,
		NATS:          mock,
		Store:         store,
		SenderID:      "worker-short",
	}

	handlerCalled := false

	// Middleware that short-circuits (never calls next)
	agent.Use(func(next HandlerFunc) HandlerFunc {
		return func(ctx Context, data any) (any, error) {
			return map[string]string{"shortcut": "yes"}, nil
		}
	})

	Register(agent, "job.short", func(_ Context, input struct{ Prompt string }) (map[string]string, error) {
		handlerCalled = true
		return map[string]string{"summary": input.Prompt}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      "job.short",
		ContextPtr: "redis://" + ctxKey,
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-short",
		SenderId:        "client-short",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	mock.send("job.short", data)

	if handlerCalled {
		t.Fatal("handler should not have been called")
	}

	// Verify result was stored with shortcutted value
	resultData, err := store.Get(nilContext(), "res:"+jobID)
	if err != nil {
		t.Fatalf("result missing: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(resultData, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if decoded["shortcut"] != "yes" {
		t.Fatalf("unexpected result: %v", decoded)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	store := NewInMemoryBlobStore()
	mock := newMockNATS()
	jobID := "job-mw-log"
	ctxKey := "ctx:" + jobID
	payload, _ := json.Marshal(map[string]string{"prompt": "hello"})
	_ = store.Set(nilContext(), ctxKey, payload)

	var buf bytes.Buffer
	testLogger := log.New(&buf, "", 0)

	agent := &Agent{
		HandshakeMode: HandshakeModeOff,
		NATS:          mock,
		Store:         store,
		SenderID:      "worker-log",
	}

	agent.Use(LoggingMiddleware(testLogger))

	Register(agent, "job.log", func(_ Context, input struct{ Prompt string }) (map[string]string, error) {
		return map[string]string{"summary": input.Prompt}, nil
	})

	if err := agent.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      "job.log",
		ContextPtr: "redis://" + ctxKey,
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-log",
		SenderId:        "client-log",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, _ := capsdk.MarshalDeterministic(packet)
	mock.send("job.log", data)

	logOutput := buf.String()
	if logOutput == "" {
		t.Fatal("logging middleware produced no output")
	}
	if !bytes.Contains([]byte(logOutput), []byte("job="+jobID)) {
		t.Fatalf("log missing job ID: %s", logOutput)
	}
	if !bytes.Contains([]byte(logOutput), []byte("ok")) {
		t.Fatalf("log missing 'ok': %s", logOutput)
	}
}
