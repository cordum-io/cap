package client

import (
	"context"
	"fmt"

	"github.com/coretexos/cap/sdk/go"
	agentv1 "github.com/coretexos/cap/cortex/agent/v1"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Submit publishes a JobRequest to the submit subject.
func Submit(ctx context.Context, nc *nats.Conn, req *agentv1.JobRequest, traceID, senderID string) error {
	packet := &agentv1.BusPacket{
		TraceId:         traceID,
		SenderId:        senderID,
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload: &agentv1.BusPacket_JobRequest{
			JobRequest: req,
		},
	}
	data, err := proto.Marshal(packet)
	if err != nil {
		return fmt.Errorf("marshal submit packet: %w", err)
	}
	return nc.Publish(capsdk.SubjectSubmit, data)
}
