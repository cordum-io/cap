package capsdk

import (
	"crypto/ecdsa"
	"errors"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

func TestTrustHandshakeRejectsUnsupportedProofAlgorithm(t *testing.T) {
	worker, scheduler := trustKey(10), trustKey(11)
	keys := map[string]*ecdsa.PublicKey{
		"worker-key": &worker.PublicKey, "scheduler-key": &scheduler.PublicKey,
	}
	algorithms := map[string]agentv1.WorkerHandshakeProofAlgorithm{
		"unspecified": agentv1.WorkerHandshakeProofAlgorithm_WORKER_HANDSHAKE_PROOF_ALGORITHM_UNSPECIFIED,
		"unknown":     agentv1.WorkerHandshakeProofAlgorithm(99),
	}
	for _, payload := range []string{"request", "challenge", "authenticate", "result"} {
		for name, algorithm := range algorithms {
			t.Run(payload+"/"+name, func(t *testing.T) {
				packet := trustPacket(payload)
				setTrustProofAlgorithm(packet, algorithm)
				if _, err := TrustHandshakeDigest(packet); !errors.Is(err, ErrTrustHandshakeAlgorithm) {
					t.Errorf("TrustHandshakeDigest() error=%v, want %v", err, ErrTrustHandshakeAlgorithm)
				}
				key := worker
				if payload == "challenge" || payload == "result" {
					key = scheduler
				}
				if err := SignTrustHandshake(packet, key); !errors.Is(err, ErrTrustHandshakeAlgorithm) {
					t.Errorf("SignTrustHandshake() error=%v, want %v", err, ErrTrustHandshakeAlgorithm)
				}
				if len(packet.Signature) != 0 {
					t.Error("SignTrustHandshake() mutated signature on rejected algorithm")
				}
				packet.Signature = []byte{1}
				if err := VerifyTrustHandshake(packet, keys); !errors.Is(err, ErrTrustHandshakeAlgorithm) {
					t.Errorf("VerifyTrustHandshake() error=%v, want %v", err, ErrTrustHandshakeAlgorithm)
				}
			})
		}
	}
}

func setTrustProofAlgorithm(packet *agentv1.BusPacket, algorithm agentv1.WorkerHandshakeProofAlgorithm) {
	switch payload := packet.GetPayload().(type) {
	case *agentv1.BusPacket_WorkerHandshakeChallengeRequest:
		payload.WorkerHandshakeChallengeRequest.ProofAlgorithm = algorithm
	case *agentv1.BusPacket_WorkerHandshakeChallenge:
		payload.WorkerHandshakeChallenge.ProofAlgorithm = algorithm
	case *agentv1.BusPacket_WorkerHandshakeAuthenticate:
		payload.WorkerHandshakeAuthenticate.Challenge.ProofAlgorithm = algorithm
	case *agentv1.BusPacket_WorkerHandshakeResult:
		payload.WorkerHandshakeResult.Challenge.ProofAlgorithm = algorithm
	}
}
