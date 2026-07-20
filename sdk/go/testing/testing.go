// Package captesting provides test utilities for CAP handlers.
// It allows testing workers and runtime handlers without running NATS or Redis.
package captesting

import (
	"context"
	"sync"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/cordum-io/cap/v2/sdk/go/worker"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PublishedMsg represents a message published on the in-memory bus.
type PublishedMsg struct {
	Subject string
	Data    []byte
}

// InMemoryBus implements worker.NATSConn and runtime.NATSConn without NATS.
type InMemoryBus struct {
	mu        sync.Mutex
	subs      map[string]nats.MsgHandler
	Published []PublishedMsg
}

// NewInMemoryBus creates a new in-memory bus.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{subs: make(map[string]nats.MsgHandler)}
}

// Publish records a message and returns nil.
func (b *InMemoryBus) Publish(subject string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Published = append(b.Published, PublishedMsg{Subject: subject, Data: data})
	return nil
}

// QueueSubscribe registers a callback for the given subject.
func (b *InMemoryBus) QueueSubscribe(subj, _ string, cb nats.MsgHandler) (*nats.Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[subj] = cb
	return &nats.Subscription{}, nil
}

// Send injects a message into the bus, triggering the registered callback.
func (b *InMemoryBus) Send(subject string, data []byte) {
	b.mu.Lock()
	cb := b.subs[subject]
	b.mu.Unlock()
	if cb != nil {
		cb(&nats.Msg{Subject: subject, Data: data})
	}
}

// LastPublished returns the most recently published message.
func (b *InMemoryBus) LastPublished() (PublishedMsg, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.Published) == 0 {
		return PublishedMsg{}, false
	}
	return b.Published[len(b.Published)-1], true
}

// SubmitAndWait sends a JobRequest to a low-level worker handler and returns the JobResult.
// No NATS or Redis required.
func SubmitAndWait(handler worker.Handler, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
	bus := NewInMemoryBus()

	w := &worker.Worker{
		NATS:          bus,
		Subject:       req.GetTopic(),
		SenderID:      "test-worker",
		AllowUnsigned: true,
		Handler:       handler,
	}

	if err := w.Start(); err != nil {
		return nil, err
	}

	packet := &agentv1.BusPacket{
		TraceId:         "test-trace",
		SenderId:        "test-client",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		return nil, err
	}

	bus.Send(req.GetTopic(), data)

	msg, ok := bus.LastPublished()
	if !ok {
		return nil, nil
	}

	var out agentv1.BusPacket
	if err := proto.Unmarshal(msg.Data, &out); err != nil {
		return nil, err
	}

	return out.GetJobResult(), nil
}

// SubmitToRuntime sends a JobRequest to a runtime Agent and returns the result BusPacket.
// The agent must already have handlers registered and been started via agent.Start().
func SubmitToRuntime(bus *InMemoryBus, req *agentv1.JobRequest) (*agentv1.BusPacket, error) {
	packet := &agentv1.BusPacket{
		TraceId:         "test-trace",
		SenderId:        "test-client",
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt:       timestamppb.Now(),
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		return nil, err
	}

	bus.Send(req.GetTopic(), data)

	msg, ok := bus.LastPublished()
	if !ok {
		return nil, nil
	}

	var out agentv1.BusPacket
	if err := proto.Unmarshal(msg.Data, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// NewTestContext creates a minimal context.Context for testing handlers directly.
func NewTestContext() context.Context {
	return context.Background()
}
