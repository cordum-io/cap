package worker

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"strings"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Publisher publishes CAP envelopes to the message bus.
// This matches the Publisher interface in the client package.
type Publisher interface {
	Publish(subject string, data []byte) error
}

// DirectSubject returns the direct worker subject for a worker ID.
// Direct subjects allow sending jobs to a specific worker, bypassing pool routing.
func DirectSubject(workerID string) string {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return ""
	}
	return "worker." + workerID + ".jobs"
}

// PublishCancel builds a signed JobCancel BusPacket and publishes it.
func PublishCancel(pub Publisher, cancel *agentv1.JobCancel, traceID, senderID string, key *ecdsa.PrivateKey) error {
	if pub == nil {
		return errors.New("publisher required")
	}
	if cancel == nil {
		return errors.New("cancel required")
	}
	cancel.JobId = strings.TrimSpace(cancel.JobId)
	if cancel.JobId == "" {
		return errors.New("job id required")
	}
	senderID = strings.TrimSpace(senderID)
	if senderID == "" {
		return errors.New("sender id required")
	}
	if strings.TrimSpace(traceID) == "" {
		traceID = cancel.JobId
	}
	packet := &agentv1.BusPacket{
		TraceId:         traceID,
		SenderId:        senderID,
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload: &agentv1.BusPacket_JobCancel{
			JobCancel: cancel,
		},
	}
	return publishEnvelope(pub, capsdk.SubjectCancel, packet, key)
}

func publishEnvelope(pub Publisher, subject string, packet *agentv1.BusPacket, key *ecdsa.PrivateKey) error {
	if packet == nil {
		return errors.New("packet required")
	}
	if strings.TrimSpace(subject) == "" {
		return errors.New("subject required")
	}
	if key != nil {
		if err := capsdk.SignPacket(packet, key); err != nil {
			return fmt.Errorf("sign packet: %w", err)
		}
	}
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		return fmt.Errorf("marshal packet: %w", err)
	}
	if err := pub.Publish(subject, data); err != nil {
		return fmt.Errorf("publish packet: %w", err)
	}
	return nil
}
