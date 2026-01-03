package worker

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/coretexos/cap/v2/sdk/go"
	agentv1 "github.com/coretexos/cap/v2/coretex/agent/v1"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler processes a JobRequest and returns a JobResult.
type Handler func(context.Context, *agentv1.JobRequest) (*agentv1.JobResult, error)

// NATSConn is an interface that represents a NATS connection.
type NATSConn interface {
	Publish(subject string, data []byte) error
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
}

// Worker subscribes to a pool subject and handles jobs.
type Worker struct {
	NATS        NATSConn
	Subject     string
	Handler     Handler
	PublicKeys  map[string]*ecdsa.PublicKey
	PrivateKey  *ecdsa.PrivateKey
	SenderID    string
}

// Start begins consuming and handling JobRequests. It returns after the subscription is created.
func (w *Worker) Start() error {
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
	_, err := w.NATS.QueueSubscribe(w.Subject, w.Subject, func(msg *nats.Msg) {
		ctx := context.Background()
		var packet agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &packet); err != nil {
			log.Printf("worker: could not decode packet: %v", err)
			return
		}

		// Verify the signature
		if w.PublicKeys != nil {
			pubKey, ok := w.PublicKeys[packet.GetSenderId()]
			if !ok {
				log.Printf("worker: no public key found for sender: %s", packet.GetSenderId())
				return
			}

			signature := packet.Signature
			packet.Signature = nil
			unsignedData, err := proto.Marshal(&packet)
			if err != nil {
				log.Printf("worker: could not marshal unsigned packet for verification: %v", err)
				return
			}
			hash := sha256.Sum256(unsignedData)
			if !ecdsa.VerifyASN1(pubKey, hash[:], signature) {
				log.Printf("worker: invalid signature for packet from sender: %s", packet.GetSenderId())
				return
			}
		}

		req := packet.GetJobRequest()
		if req == nil {
			return
		}
		res, err := w.Handler(ctx, req)
		if err != nil {
			res = &agentv1.JobResult{
				JobId:        req.GetJobId(),
				Status:       agentv1.JobStatus_JOB_STATUS_FAILED,
				ErrorMessage: err.Error(),
			}
		}
		if res == nil {
			res = &agentv1.JobResult{
				JobId:        req.GetJobId(),
				Status:       agentv1.JobStatus_JOB_STATUS_FAILED,
				ErrorMessage: "handler returned nil",
			}
		}
		// Fill required metadata if missing
		if res.JobId == "" {
			res.JobId = req.GetJobId()
		}
		if res.WorkerId == "" {
			res.WorkerId = w.SenderID
		}
		if res.ExecutionMs == 0 {
			res.ExecutionMs = 0
		}
		out := &agentv1.BusPacket{
			TraceId:         packet.GetTraceId(),
			SenderId:        w.SenderID,
			ProtocolVersion: capsdk.DefaultProtocolVersion,
			CreatedAt:       timestamppb.Now(),
			Payload: &agentv1.BusPacket_JobResult{
				JobResult: res,
			},
		}

		// Sign the outgoing packet
		if w.PrivateKey != nil {
			unsignedData, err := proto.Marshal(out)
			if err != nil {
				log.Printf("worker: could not marshal outgoing packet for signing: %v", err)
				return
			}
			hash := sha256.Sum256(unsignedData)
			signature, err := ecdsa.SignASN1(rand.Reader, w.PrivateKey, hash[:])
			if err != nil {
				log.Printf("worker: could not sign outgoing packet: %v", err)
				return
			}
			out.Signature = signature
		}

		data, mErr := proto.Marshal(out)
		if mErr != nil {
			return
		}
		_ = w.NATS.Publish(capsdk.SubjectResult, data)
	})
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	return nil
}

// HeartbeatPayload returns a protobuf-encoded heartbeat envelope.
func HeartbeatPayload(workerID, pool string, activeJobs, maxParallel int, cpuLoad float32) ([]byte, error) {
	hb := &agentv1.BusPacket{
		TraceId:         "",
		SenderId:        workerID,
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload: &agentv1.BusPacket_Heartbeat{
			Heartbeat: &agentv1.Heartbeat{
				WorkerId:        workerID,
				Pool:            pool,
				ActiveJobs:      int32(activeJobs),
				MaxParallelJobs: int32(maxParallel),
				CpuLoad:         cpuLoad,
			},
		},
	}
	return proto.Marshal(hb)
}

// EmitHeartbeat publishes a heartbeat once. Call repeatedly on a ticker.
func EmitHeartbeat(nc *nats.Conn, payload []byte) error {
	return nc.Publish(capsdk.SubjectHeartbeat, payload)
}

// HeartbeatLoop emits heartbeats until ctx is done.
func HeartbeatLoop(ctx context.Context, nc *nats.Conn, payloadFn func() ([]byte, error)) {
	ticker := time.NewTicker(capsdk.DefaultHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload, err := payloadFn()
			if err == nil {
				_ = EmitHeartbeat(nc, payload)
			}
		}
	}
}
