package main

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type mutationProof struct {
	signatureValid bool
	tamperRejected bool
}

func exerciseNegative(connection *nats.Conn, config *clientConfig) (mutationProof, error) {
	createdAt := time.Now().UTC()
	if config.testCase == "skew" {
		createdAt = createdAt.Add(61 * time.Second)
	}
	request, err := buildRequest(config.trust, issuePurpose(), createdAt)
	if err != nil {
		return mutationProof{}, err
	}
	if config.testCase == "replay" {
		if _, err := requestPacket(connection, capsdk.WorkerHandshakeChallengeSubject, request); err != nil {
			return mutationProof{}, err
		}
		return mutationProof{}, expectRejected(connection, request)
	}
	if config.testCase != "impersonation" {
		if err := applyNegativeMutation(config, request); err != nil {
			return mutationProof{}, err
		}
	}
	proof, err := proveMutationSignature(config.testCase, request, config.trust)
	if err != nil {
		return mutationProof{}, err
	}
	return proof, expectRejected(connection, request)
}

func proveMutationSignature(testCase string, packet *agentv1.BusPacket, trust *capsdk.WorkerTrustConfig) (mutationProof, error) {
	if testCase != "wrong_audience" && testCase != "tamper" {
		return mutationProof{}, nil
	}
	keys := map[string]*ecdsa.PublicKey{trust.ProofKeyID: &trust.ProofPrivateKey.PublicKey}
	err := capsdk.VerifyTrustHandshake(packet, keys)
	if testCase == "wrong_audience" {
		if err != nil {
			return mutationProof{}, fmt.Errorf("re-signed mutation has an invalid signature: %w", err)
		}
		return mutationProof{signatureValid: true}, nil
	}
	if err == nil {
		return mutationProof{}, errors.New("tampered packet retained a valid signature")
	}
	if !errors.Is(err, capsdk.ErrInvalidSignature) {
		return mutationProof{}, fmt.Errorf("tamper signature check failed unexpectedly: %w", err)
	}
	return mutationProof{tamperRejected: true}, nil
}

func applyNegativeMutation(config *clientConfig, packet *agentv1.BusPacket) error {
	if err := mutateRequest(config.testCase, packet); err != nil {
		return err
	}
	if config.testCase == "wrong_audience" {
		return capsdk.SignTrustHandshake(packet, config.trust.ProofPrivateKey)
	}
	return nil
}

func mutateRequest(testCase string, packet *agentv1.BusPacket) error {
	request := packet.GetWorkerHandshakeChallengeRequest()
	if request == nil {
		return errors.New("challenge request payload unavailable")
	}
	switch testCase {
	case "skew":
		return nil
	case "wrong_audience":
		request.Audience = "other-scheduler"
	case "missing_identity":
		packet.SenderId = ""
	case "missing_trace":
		packet.TraceId = ""
	case "unsupported_version":
		packet.ProtocolVersion = 2
	case "tamper":
		request.Audience = "tampered-after-signing"
	default:
		return fmt.Errorf("unsupported negative case %q", testCase)
	}
	return nil
}

func expectRejected(connection *nats.Conn, packet *agentv1.BusPacket) error {
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(packet)
	if err != nil {
		return err
	}
	message, err := connection.Request(capsdk.WorkerHandshakeChallengeSubject, data, 750*time.Millisecond)
	if message != nil {
		return errors.New("negative request received a reply")
	}
	if !errors.Is(err, nats.ErrTimeout) {
		return fmt.Errorf("negative request error=%w, want timeout", err)
	}
	return nil
}
