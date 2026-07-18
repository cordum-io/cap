package capsdk

import (
	"fmt"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestValidateBusPacketRequiresExactProtocolVersion(t *testing.T) {
	for _, version := range []int32{-1, 0, 2, 999} {
		t.Run(fmt.Sprintf("version_%d", version), func(t *testing.T) {
			packet := validBusPacket(validJobRequest())
			packet.ProtocolVersion = version
			assertValidationError(t, ValidateBusPacket(packet), "protocol_version")
		})
	}
}

func TestValidateBusPacketBindsPayloadIdentityToSender(t *testing.T) {
	tests := []struct {
		name  string
		field string
		body  *agentv1.BusPacket
	}{
		{
			name:  "legacy handshake component mismatch",
			field: "handshake.component_id",
			body:  handshakeSecurityPacket("sender-a", "sender-b"),
		},
		{
			name:  "heartbeat worker mismatch",
			field: "heartbeat.worker_id",
			body:  heartbeatSecurityPacket("sender-a", "sender-b"),
		},
		{
			name:  "job result worker mismatch",
			field: "job_result.worker_id",
			body:  resultSecurityPacket("sender-a", "sender-b"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertValidationError(t, ValidateBusPacket(test.body), test.field)
		})
	}
}

func TestValidateBusPacketRejectsLegacyHandshakeWithoutTrace(t *testing.T) {
	packet := handshakeSecurityPacket("worker-1", "worker-1")
	packet.TraceId = ""
	assertValidationError(t, ValidateBusPacket(packet), "trace_id")
}

func securityPacket(sender string) *agentv1.BusPacket {
	return &agentv1.BusPacket{
		TraceId:         "trace-security",
		SenderId:        sender,
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: DefaultProtocolVersion,
	}
}

func handshakeSecurityPacket(sender, component string) *agentv1.BusPacket {
	packet := securityPacket(sender)
	packet.Payload = &agentv1.BusPacket_Handshake{Handshake: &agentv1.Handshake{
		ComponentId:       component,
		Role:              agentv1.ComponentRole_COMPONENT_ROLE_WORKER,
		SupportedVersions: []int32{DefaultProtocolVersion},
	}}
	return packet
}

func heartbeatSecurityPacket(sender, worker string) *agentv1.BusPacket {
	packet := securityPacket(sender)
	packet.Payload = &agentv1.BusPacket_Heartbeat{Heartbeat: &agentv1.Heartbeat{
		WorkerId: worker,
		Pool:     "default",
	}}
	return packet
}

func resultSecurityPacket(sender, worker string) *agentv1.BusPacket {
	packet := securityPacket(sender)
	packet.Payload = &agentv1.BusPacket_JobResult{JobResult: &agentv1.JobResult{
		JobId:    "job-1",
		Status:   agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
		WorkerId: worker,
	}}
	return packet
}
