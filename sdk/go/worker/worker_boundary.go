package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (w *Worker) validateStart() error {
	if w.NATS == nil {
		return errors.New("worker: NATS connection is nil")
	}
	if w.Subject == "" {
		return errors.New("worker: subject is required")
	}
	if w.Handler == nil {
		return errors.New("worker: handler is required")
	}
	if w.SenderID == "" {
		return errors.New("worker: SenderID is required")
	}
	if !w.AllowUnsigned && len(w.PublicKeys) == 0 {
		return errors.New("worker: public keys required; set AllowUnsigned for explicit unsigned legacy mode")
	}
	if !w.AllowUnsigned && w.PrivateKey == nil {
		return errors.New("worker: private key required; set AllowUnsigned for explicit unsigned legacy mode")
	}
	return nil
}

func (w *Worker) handleMessage(message *nats.Msg) {
	packet, request, err := w.decodeRequest(message.Data)
	if err != nil {
		w.Logger.Warn("worker: packet rejected", "error", err)
		return
	}
	w.Metrics.OnJobReceived(request.GetJobId(), request.GetTopic())
	started := time.Now()
	result := w.executeRequest(request)
	execution := time.Since(started).Milliseconds()
	normalizeResult(result, request.GetJobId(), w.SenderID, execution)
	w.recordResultMetric(result, execution)
	if err := w.publishResult(packet.GetTraceId(), result); err != nil {
		w.Logger.Error("worker: result publish failed", "job_id", request.GetJobId(), "error", err)
	}
}

func (w *Worker) decodeRequest(data []byte) (*agentv1.BusPacket, *agentv1.JobRequest, error) {
	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(data, packet); err != nil {
		return nil, nil, capsdk.NewMalformedPacketError(err.Error())
	}
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return nil, nil, err
	}
	if err := w.verifyPacket(packet); err != nil {
		return nil, nil, err
	}
	request := packet.GetJobRequest()
	if request == nil {
		return nil, nil, errors.New("worker: packet is not a job request")
	}
	return packet, request, nil
}

func (w *Worker) verifyPacket(packet *agentv1.BusPacket) error {
	if len(w.PublicKeys) == 0 {
		if w.AllowUnsigned {
			return nil
		}
		return capsdk.NewSignatureMissingError("worker: unsigned inbound packet is not enabled")
	}
	publicKey, ok := w.PublicKeys[packet.GetSenderId()]
	if !ok {
		return fmt.Errorf("worker: no public key for sender %q", packet.GetSenderId())
	}
	if len(packet.GetSignature()) == 0 {
		return capsdk.NewSignatureMissingError("packet from sender: " + packet.GetSenderId())
	}
	if err := capsdk.VerifyPacketSignature(packet, publicKey); err != nil {
		return capsdk.NewSignatureInvalidError("sender " + packet.GetSenderId() + ": " + err.Error())
	}
	return nil
}

func (w *Worker) executeRequest(request *agentv1.JobRequest) *agentv1.JobResult {
	handler := w.Handler
	for index := len(w.middlewares) - 1; index >= 0; index-- {
		handler = w.middlewares[index](handler)
	}
	result, err := handler(context.Background(), request)
	if err != nil {
		return &agentv1.JobResult{
			JobId: request.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_FAILED,
			ErrorMessage: err.Error(),
		}
	}
	if result == nil {
		return &agentv1.JobResult{
			JobId: request.GetJobId(), Status: agentv1.JobStatus_JOB_STATUS_FAILED,
			ErrorMessage: "handler returned nil",
		}
	}
	return result
}

func normalizeResult(result *agentv1.JobResult, jobID, workerID string, execution int64) {
	if result.JobId == "" {
		result.JobId = jobID
	}
	if result.WorkerId == "" {
		result.WorkerId = workerID
	}
	if result.ExecutionMs == 0 {
		result.ExecutionMs = execution
	}
}

func (w *Worker) recordResultMetric(result *agentv1.JobResult, execution int64) {
	if result.Status == agentv1.JobStatus_JOB_STATUS_FAILED {
		w.Metrics.OnJobFailed(result.JobId, result.ErrorMessage)
		return
	}
	w.Metrics.OnJobCompleted(result.JobId, execution, result.Status.String())
}

func (w *Worker) publishResult(traceID string, result *agentv1.JobResult) error {
	if w.PrivateKey == nil && !w.AllowUnsigned {
		return errors.New("worker: private key required to publish result")
	}
	packet := &agentv1.BusPacket{
		TraceId: traceID, SenderId: w.SenderID,
		ProtocolVersion: capsdk.DefaultProtocolVersion, CreatedAt: timestamppb.Now(),
		Payload: &agentv1.BusPacket_JobResult{JobResult: result},
	}
	data, err := marshalValidatedEnvelope(packet, w.PrivateKey)
	if err != nil {
		return err
	}
	return w.NATS.Publish(capsdk.SubjectResult, data)
}
