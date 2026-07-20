package main

import (
	"encoding/base64"
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// buildPacket turns one language-neutral corpus case into a BusPacket using
// only the SDK's generated types. Each driver implements this independently;
// agreement between them is exactly what the matrix measures.
func buildPacket(c corpusCase, cor corpus, keyID string, createdAtUnix, expiresAtUnix int64) (*agentv1.BusPacket, error) {
	messageID, err := base64.StdEncoding.DecodeString(c.MessageID)
	if err != nil {
		return nil, fmt.Errorf("case %s: decode messageId: %w", c.Name, err)
	}
	pkt := &agentv1.BusPacket{
		TraceId:         c.TraceID,
		SenderId:        cor.SenderID,
		CreatedAt:       timestamppb.New(unixSecond(createdAtUnix)),
		ProtocolVersion: 1,
		SignatureMetadata: &agentv1.SignatureMetadata{
			ProfileVersion: "cap-production-v1",
			Algorithm:      "ECDSA-P256-SHA256",
			MessageId:      messageID,
			Audience:       cor.Audience,
			ExpiresAt:      timestamppb.New(unixSecond(expiresAtUnix)),
			KeyId:          keyID,
		},
		Identity: &agentv1.IdentityBinding{
			TenantId:     c.Identity.TenantID,
			PrincipalId:  c.Identity.PrincipalID,
			ActorId:      c.Identity.ActorID,
			DelegationId: c.Identity.DelegationID,
		},
	}
	if err := attachPayload(pkt, c); err != nil {
		return nil, err
	}
	return pkt, nil
}

func attachPayload(pkt *agentv1.BusPacket, c corpusCase) error {
	p := c.Payload
	switch kind, _ := p["kind"].(string); kind {
	case "jobRequest":
		pkt.Payload = &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{
			JobId:       str(p, "jobId"),
			Topic:       str(p, "topic"),
			TenantId:    str(p, "tenantId"),
			PrincipalId: str(p, "principalId"),
		}}
	case "jobResult":
		pkt.Payload = &agentv1.BusPacket_JobResult{JobResult: &agentv1.JobResult{
			JobId:       str(p, "jobId"),
			Status:      agentv1.JobStatus(num(p, "status")),
			WorkerId:    str(p, "workerId"),
			ExecutionMs: int64(num(p, "executionMs")),
			Dispatch:    dispatch(p),
		}}
	case "jobProgress":
		pkt.Payload = &agentv1.BusPacket_JobProgress{JobProgress: &agentv1.JobProgress{
			JobId:    str(p, "jobId"),
			StepId:   str(p, "stepId"),
			Percent:  int32(num(p, "percent")),
			Message:  str(p, "message"),
			Dispatch: dispatch(p),
		}}
	case "heartbeat":
		pkt.Payload = &agentv1.BusPacket_Heartbeat{Heartbeat: &agentv1.Heartbeat{
			WorkerId:   str(p, "workerId"),
			Region:     str(p, "region"),
			Type:       str(p, "type"),
			ActiveJobs: int32(num(p, "activeJobs")),
			Pool:       str(p, "pool"),
		}}
	default:
		return fmt.Errorf("case %s: unsupported payload kind %q", c.Name, kind)
	}
	return nil
}

func dispatch(p map[string]any) *agentv1.DispatchIdentity {
	raw, ok := p["dispatch"].(map[string]any)
	if !ok {
		return nil
	}
	return &agentv1.DispatchIdentity{
		DispatchId:       str(raw, "dispatchId"),
		Attempt:          uint64(num(raw, "attempt")),
		AssignedWorkerId: str(raw, "assignedWorkerId"),
	}
}

func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func num(m map[string]any, key string) float64 {
	v, _ := m[key].(float64)
	return v
}
