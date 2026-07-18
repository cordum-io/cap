package capsdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

// TrustHandshakePhase identifies the signed trust-handshake payload.
type TrustHandshakePhase uint8

const (
	TrustHandshakePhaseChallengeRequest TrustHandshakePhase = iota + 1
	TrustHandshakePhaseChallenge
	TrustHandshakePhaseAuthenticate
	TrustHandshakePhaseResult
)

const (
	TrustHandshakeChallengeRequestDomain = "CAP-WORKER-HANDSHAKE-CHALLENGE-REQUEST-V1"
	TrustHandshakeChallengeDomain        = "CAP-WORKER-HANDSHAKE-CHALLENGE-V1"
	TrustHandshakeAuthenticateDomain     = "CAP-WORKER-HANDSHAKE-AUTHENTICATE-V1"
	TrustHandshakeResultDomain           = "CAP-WORKER-HANDSHAKE-RESULT-V1"
)

var (
	// ErrTrustHandshakePayload indicates an absent or non-trust payload.
	ErrTrustHandshakePayload = errors.New("capsdk: invalid trust handshake payload")
	// ErrTrustHandshakeKeyID indicates the signed payload names no configured key.
	ErrTrustHandshakeKeyID = errors.New("capsdk: unknown trust handshake key id")
	// ErrTrustHandshakeKey indicates a nil, malformed, or non-P-256 key.
	ErrTrustHandshakeKey = errors.New("capsdk: trust handshake requires a valid P-256 key")
	// ErrTrustHandshakeAlgorithm indicates a payload does not declare P-256 SHA-256.
	ErrTrustHandshakeAlgorithm = errors.New("capsdk: unsupported trust handshake proof algorithm")
)

// SelectTrustHandshakePhase derives the phase and domain from the concrete oneof.
func SelectTrustHandshakePhase(packet *agentv1.BusPacket) (TrustHandshakePhase, string, error) {
	if packet == nil {
		return 0, "", ErrTrustHandshakePayload
	}
	switch payload := packet.GetPayload().(type) {
	case *agentv1.BusPacket_WorkerHandshakeChallengeRequest:
		if payload.WorkerHandshakeChallengeRequest != nil {
			return TrustHandshakePhaseChallengeRequest, TrustHandshakeChallengeRequestDomain, nil
		}
	case *agentv1.BusPacket_WorkerHandshakeChallenge:
		if payload.WorkerHandshakeChallenge != nil {
			return TrustHandshakePhaseChallenge, TrustHandshakeChallengeDomain, nil
		}
	case *agentv1.BusPacket_WorkerHandshakeAuthenticate:
		if payload.WorkerHandshakeAuthenticate != nil {
			return TrustHandshakePhaseAuthenticate, TrustHandshakeAuthenticateDomain, nil
		}
	case *agentv1.BusPacket_WorkerHandshakeResult:
		if payload.WorkerHandshakeResult != nil {
			return TrustHandshakePhaseResult, TrustHandshakeResultDomain, nil
		}
	}
	return 0, "", ErrTrustHandshakePayload
}

// MarshalTrustHandshakeTranscript returns domain || NUL || deterministic unsigned packet.
func MarshalTrustHandshakeTranscript(packet *agentv1.BusPacket) ([]byte, error) {
	if hasUnknownProtoFields(packet) {
		return nil, fmt.Errorf("%w: unknown fields", ErrWorkerHandshakePacket)
	}
	phase, domain, err := SelectTrustHandshakePhase(packet)
	if err != nil {
		return nil, err
	}
	if err := validateTrustHandshakeAlgorithm(packet, phase); err != nil {
		return nil, err
	}
	clone := proto.Clone(packet).(*agentv1.BusPacket)
	clone.Signature = nil
	unsigned, err := proto.MarshalOptions{Deterministic: true}.Marshal(clone)
	if err != nil {
		return nil, fmt.Errorf("capsdk: marshal trust handshake: %w", err)
	}
	transcript := make([]byte, 0, len(domain)+1+len(unsigned))
	transcript = append(transcript, domain...)
	transcript = append(transcript, 0)
	return append(transcript, unsigned...), nil
}

// TrustHandshakeDigest returns SHA-256 of the phase-domain transcript.
func TrustHandshakeDigest(packet *agentv1.BusPacket) ([sha256.Size]byte, error) {
	transcript, err := MarshalTrustHandshakeTranscript(packet)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(transcript), nil
}

// SignTrustHandshake signs a trust transcript with ECDSA P-256 ASN.1 DER.
func SignTrustHandshake(packet *agentv1.BusPacket, key *ecdsa.PrivateKey) error {
	keyID, err := trustHandshakeKeyID(packet)
	if err != nil {
		return err
	}
	if keyID == "" {
		return fmt.Errorf("%w: %q", ErrTrustHandshakeKeyID, keyID)
	}
	if !validP256PrivateKey(key) {
		return ErrTrustHandshakeKey
	}
	digest, err := TrustHandshakeDigest(packet)
	if err != nil {
		return err
	}
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		return fmt.Errorf("capsdk: sign trust handshake: %w", err)
	}
	packet.Signature = signature
	return nil
}

// VerifyTrustHandshake verifies with the configured key selected by payload key ID.
func VerifyTrustHandshake(packet *agentv1.BusPacket, keys map[string]*ecdsa.PublicKey) error {
	keyID, err := trustHandshakeKeyID(packet)
	if err != nil {
		return err
	}
	if len(packet.GetSignature()) == 0 {
		return ErrMissingSignature
	}
	key, ok := keys[keyID]
	if keyID == "" || !ok {
		return fmt.Errorf("%w: %q", ErrTrustHandshakeKeyID, keyID)
	}
	if !validP256PublicKey(key) {
		return ErrTrustHandshakeKey
	}
	digest, err := TrustHandshakeDigest(packet)
	if err != nil {
		return err
	}
	if !ecdsa.VerifyASN1(key, digest[:], packet.GetSignature()) {
		return ErrInvalidSignature
	}
	return nil
}

func trustHandshakeKeyID(packet *agentv1.BusPacket) (string, error) {
	phase, _, err := SelectTrustHandshakePhase(packet)
	if err != nil {
		return "", err
	}
	if err := validateTrustHandshakeAlgorithm(packet, phase); err != nil {
		return "", err
	}
	switch phase {
	case TrustHandshakePhaseChallengeRequest:
		return packet.GetWorkerHandshakeChallengeRequest().GetProofKeyId(), nil
	case TrustHandshakePhaseChallenge:
		return packet.GetWorkerHandshakeChallenge().GetServerKeyId(), nil
	case TrustHandshakePhaseAuthenticate:
		challenge := packet.GetWorkerHandshakeAuthenticate().GetChallenge()
		if challenge != nil {
			return challenge.GetProofKeyId(), nil
		}
	case TrustHandshakePhaseResult:
		challenge := packet.GetWorkerHandshakeResult().GetChallenge()
		if challenge != nil {
			return challenge.GetServerKeyId(), nil
		}
	}
	return "", ErrTrustHandshakeKeyID
}

func validateTrustHandshakeAlgorithm(packet *agentv1.BusPacket, phase TrustHandshakePhase) error {
	var algorithm agentv1.WorkerHandshakeProofAlgorithm
	switch phase {
	case TrustHandshakePhaseChallengeRequest:
		algorithm = packet.GetWorkerHandshakeChallengeRequest().GetProofAlgorithm()
	case TrustHandshakePhaseChallenge:
		algorithm = packet.GetWorkerHandshakeChallenge().GetProofAlgorithm()
	case TrustHandshakePhaseAuthenticate:
		challenge := packet.GetWorkerHandshakeAuthenticate().GetChallenge()
		if challenge == nil {
			return ErrTrustHandshakeAlgorithm
		}
		algorithm = challenge.GetProofAlgorithm()
	case TrustHandshakePhaseResult:
		challenge := packet.GetWorkerHandshakeResult().GetChallenge()
		if challenge == nil {
			return ErrTrustHandshakeAlgorithm
		}
		algorithm = challenge.GetProofAlgorithm()
	default:
		return ErrTrustHandshakeAlgorithm
	}
	want := agentv1.WorkerHandshakeProofAlgorithm_WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256
	if algorithm != want {
		return fmt.Errorf("%w: %d", ErrTrustHandshakeAlgorithm, algorithm)
	}
	return nil
}

func validP256PublicKey(key *ecdsa.PublicKey) bool {
	if key == nil || key.Curve != elliptic.P256() || key.X == nil || key.Y == nil {
		return false
	}
	return elliptic.P256().IsOnCurve(key.X, key.Y)
}

func validP256PrivateKey(key *ecdsa.PrivateKey) bool {
	if key == nil || key.D == nil || !validP256PublicKey(&key.PublicKey) {
		return false
	}
	curve := elliptic.P256()
	if key.D.Sign() <= 0 || key.D.Cmp(curve.Params().N) >= 0 {
		return false
	}
	x, y := curve.ScalarBaseMult(key.D.Bytes())
	return x.Cmp(key.X) == 0 && y.Cmp(key.Y) == 0
}
