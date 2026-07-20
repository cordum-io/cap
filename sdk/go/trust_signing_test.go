package capsdk

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"math/big"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestTrustHandshakePhaseSelection(t *testing.T) {
	tests := []struct {
		payload string
		phase   TrustHandshakePhase
		domain  string
	}{
		{"request", TrustHandshakePhaseChallengeRequest, TrustHandshakeChallengeRequestDomain},
		{"challenge", TrustHandshakePhaseChallenge, TrustHandshakeChallengeDomain},
		{"authenticate", TrustHandshakePhaseAuthenticate, TrustHandshakeAuthenticateDomain},
		{"result", TrustHandshakePhaseResult, TrustHandshakeResultDomain},
	}
	for _, test := range tests {
		t.Run(test.payload, func(t *testing.T) {
			phase, domain, err := SelectTrustHandshakePhase(trustPacket(test.payload))
			if err != nil || phase != test.phase || domain != test.domain {
				t.Fatalf("selection=(%v,%q,%v), want (%v,%q,nil)", phase, domain, err, test.phase, test.domain)
			}
		})
	}
}

func TestTrustHandshakePhaseSelectionRejectsWrongPayload(t *testing.T) {
	wrong := &agentv1.BusPacket{Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{}}}
	for name, packet := range map[string]*agentv1.BusPacket{"nil": nil, "absent": {}, "wrong": wrong} {
		t.Run(name, func(t *testing.T) {
			_, _, err := SelectTrustHandshakePhase(packet)
			if !errors.Is(err, ErrTrustHandshakePayload) {
				t.Fatalf("SelectTrustHandshakePhase() error=%v, want %v", err, ErrTrustHandshakePayload)
			}
		})
	}
}

func TestTrustHandshakeTranscriptClearsOnlySignature(t *testing.T) {
	packet := trustPacket("authenticate")
	packet.Signature = []byte("existing-signature")
	original := proto.Clone(packet).(*agentv1.BusPacket)

	first, err := MarshalTrustHandshakeTranscript(packet)
	if err != nil {
		t.Fatalf("MarshalTrustHandshakeTranscript: %v", err)
	}
	second, err := MarshalTrustHandshakeTranscript(packet)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("second transcript deterministic=%v error=%v", bytes.Equal(first, second), err)
	}
	if !proto.Equal(packet, original) {
		t.Fatal("transcript construction mutated the source packet")
	}
	assertTrustTranscript(t, first, TrustHandshakeAuthenticateDomain, packet.AuthToken, 21)
}

func TestTrustHandshakeDigestIsSHA256Transcript(t *testing.T) {
	packet := trustPacket("challenge")
	transcript, err := MarshalTrustHandshakeTranscript(packet)
	if err != nil {
		t.Fatalf("MarshalTrustHandshakeTranscript: %v", err)
	}
	want := sha256.Sum256(transcript)
	got, err := TrustHandshakeDigest(packet)
	if err != nil || got != want {
		t.Fatalf("TrustHandshakeDigest=%x,%v want %x,nil", got, err, want)
	}
}

func TestTrustHandshakeSignVerifyAllPhases(t *testing.T) {
	worker := trustKey(1)
	scheduler := trustKey(2)
	keys := map[string]*ecdsa.PublicKey{
		"worker-key":    &worker.PublicKey,
		"scheduler-key": &scheduler.PublicKey,
	}
	for _, payload := range []string{"request", "challenge", "authenticate", "result"} {
		t.Run(payload, func(t *testing.T) {
			packet := trustPacket(payload)
			key := worker
			if payload == "challenge" || payload == "result" {
				key = scheduler
			}
			if err := SignTrustHandshake(packet, key); err != nil {
				t.Fatalf("SignTrustHandshake: %v", err)
			}
			assertStrictDERSignature(t, packet.Signature)
			if err := VerifyTrustHandshake(packet, keys); err != nil {
				t.Fatalf("VerifyTrustHandshake: %v", err)
			}
		})
	}
}

func TestTrustHandshakeSignatureCoversAuthToken(t *testing.T) {
	key := trustKey(3)
	packet := trustPacket("authenticate")
	if err := SignTrustHandshake(packet, key); err != nil {
		t.Fatalf("SignTrustHandshake: %v", err)
	}
	packet.AuthToken = "tampered-session-token"
	keys := map[string]*ecdsa.PublicKey{"worker-key": &key.PublicKey}
	if err := VerifyTrustHandshake(packet, keys); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("VerifyTrustHandshake() error=%v, want %v", err, ErrInvalidSignature)
	}
}

func TestTrustHandshakeRejectsInvalidInputs(t *testing.T) {
	p256 := trustKey(4)
	other := trustKey(5)
	p384, err := ecdsa.GenerateKey(elliptic.P384(), bytes.NewReader(bytes.Repeat([]byte{7}, 256)))
	if err != nil {
		t.Fatalf("generate P-384 key: %v", err)
	}
	signed := trustPacket("request")
	if err := SignTrustHandshake(signed, p256); err != nil {
		t.Fatalf("SignTrustHandshake: %v", err)
	}
	tests := []struct {
		name   string
		packet *agentv1.BusPacket
		keys   map[string]*ecdsa.PublicKey
		want   error
	}{
		{"nil packet", nil, nil, ErrTrustHandshakePayload},
		{"missing signature", trustPacket("request"), map[string]*ecdsa.PublicKey{"worker-key": &p256.PublicKey}, ErrMissingSignature},
		{"unknown key id", signed, map[string]*ecdsa.PublicKey{}, ErrTrustHandshakeKeyID},
		{"wrong key", signed, map[string]*ecdsa.PublicKey{"worker-key": &other.PublicKey}, ErrInvalidSignature},
		{"non P-256 key", signed, map[string]*ecdsa.PublicKey{"worker-key": &p384.PublicKey}, ErrTrustHandshakeKey},
		{"wrong payload", &agentv1.BusPacket{Signature: []byte{1}, Payload: &agentv1.BusPacket_JobResult{}}, nil, ErrTrustHandshakePayload},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := VerifyTrustHandshake(test.packet, test.keys); !errors.Is(err, test.want) {
				t.Fatalf("VerifyTrustHandshake() error=%v, want %v", err, test.want)
			}
		})
	}

	trailing := proto.Clone(signed).(*agentv1.BusPacket)
	trailing.Signature = append(trailing.Signature, 0)
	if err := VerifyTrustHandshake(trailing, map[string]*ecdsa.PublicKey{"worker-key": &p256.PublicKey}); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("trailing DER error=%v, want %v", err, ErrInvalidSignature)
	}
}

func TestTrustHandshakeNeverUsesPacketKeyMaterial(t *testing.T) {
	attacker := trustKey(6)
	packet := trustPacket("request")
	request := packet.GetWorkerHandshakeChallengeRequest()
	request.ProofKeyId = "-----BEGIN PUBLIC KEY-----packet-controlled-----END PUBLIC KEY-----"
	request.SdkVersion = hex.EncodeToString(elliptic.Marshal(elliptic.P256(), attacker.X, attacker.Y))
	if err := SignTrustHandshake(packet, attacker); err != nil {
		t.Fatalf("SignTrustHandshake: %v", err)
	}
	if err := VerifyTrustHandshake(packet, map[string]*ecdsa.PublicKey{}); !errors.Is(err, ErrTrustHandshakeKeyID) {
		t.Fatalf("VerifyTrustHandshake() error=%v, want %v", err, ErrTrustHandshakeKeyID)
	}
}

func TestSignTrustHandshakeRejectsInvalidInputs(t *testing.T) {
	p384, err := ecdsa.GenerateKey(elliptic.P384(), bytes.NewReader(bytes.Repeat([]byte{9}, 256)))
	if err != nil {
		t.Fatalf("generate P-384 key: %v", err)
	}
	missingKeyID := trustPacket("request")
	missingKeyID.GetWorkerHandshakeChallengeRequest().ProofKeyId = ""
	for _, test := range []struct {
		name   string
		packet *agentv1.BusPacket
		key    *ecdsa.PrivateKey
		want   error
	}{
		{"nil packet", nil, trustKey(7), ErrTrustHandshakePayload},
		{"nil key", trustPacket("request"), nil, ErrTrustHandshakeKey},
		{"non P-256 key", trustPacket("request"), p384, ErrTrustHandshakeKey},
		{"missing key ID", missingKeyID, trustKey(8), ErrTrustHandshakeKeyID},
		{"wrong payload", &agentv1.BusPacket{Payload: &agentv1.BusPacket_JobResult{}}, trustKey(9), ErrTrustHandshakePayload},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := SignTrustHandshake(test.packet, test.key); !errors.Is(err, test.want) {
				t.Fatalf("SignTrustHandshake() error=%v, want %v", err, test.want)
			}
		})
	}
}

func trustPacket(payload string) *agentv1.BusPacket {
	challenge := &agentv1.WorkerHandshakeChallenge{
		RequestId: "request-1", ChallengeId: "challenge-1", TraceId: "trace-1",
		WorkerId: "worker-1", ProofKeyId: "worker-key", ServerKeyId: "scheduler-key",
		ProofAlgorithm: agentv1.WorkerHandshakeProofAlgorithm_WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256,
	}
	packet := &agentv1.BusPacket{TraceId: "trace-1", SenderId: "worker-1", ProtocolVersion: 1}
	switch payload {
	case "request":
		packet.Payload = &agentv1.BusPacket_WorkerHandshakeChallengeRequest{WorkerHandshakeChallengeRequest: &agentv1.WorkerHandshakeChallengeRequest{
			ProofKeyId: "worker-key", ProofAlgorithm: agentv1.WorkerHandshakeProofAlgorithm_WORKER_HANDSHAKE_PROOF_ALGORITHM_ECDSA_P256_SHA256,
		}}
	case "challenge":
		packet.Payload = &agentv1.BusPacket_WorkerHandshakeChallenge{WorkerHandshakeChallenge: challenge}
	case "authenticate":
		packet.AuthToken = "active-session-token"
		packet.Payload = &agentv1.BusPacket_WorkerHandshakeAuthenticate{WorkerHandshakeAuthenticate: &agentv1.WorkerHandshakeAuthenticate{Challenge: challenge}}
	case "result":
		packet.AuthToken = "new-session-token"
		packet.Payload = &agentv1.BusPacket_WorkerHandshakeResult{WorkerHandshakeResult: &agentv1.WorkerHandshakeResult{Challenge: challenge, Accepted: true}}
	}
	return packet
}

func trustKey(value int64) *ecdsa.PrivateKey {
	curve := elliptic.P256()
	d := big.NewInt(value)
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}, D: d}
}

func assertTrustTranscript(t *testing.T, transcript []byte, domain, authToken string, payloadTag protowire.Number) {
	t.Helper()
	prefix := append([]byte(domain), 0)
	if !bytes.HasPrefix(transcript, prefix) {
		t.Fatalf("transcript prefix=%q want %q", transcript[:min(len(transcript), len(prefix))], prefix)
	}
	var unsigned agentv1.BusPacket
	if err := proto.Unmarshal(transcript[len(prefix):], &unsigned); err != nil {
		t.Fatalf("unmarshal unsigned packet: %v", err)
	}
	if len(unsigned.Signature) != 0 || unsigned.AuthToken != authToken {
		t.Fatalf("unsigned signature=%x auth_token=%q", unsigned.Signature, unsigned.AuthToken)
	}
	body := transcript[len(prefix):]
	authIndex := bytes.Index(body, protowire.AppendTag(nil, 18, protowire.BytesType))
	payloadIndex := bytes.Index(body, protowire.AppendTag(nil, payloadTag, protowire.BytesType))
	if authIndex < 0 || payloadIndex < 0 || authIndex > payloadIndex {
		t.Fatalf("auth tag index=%d payload tag %d index=%d", authIndex, payloadTag, payloadIndex)
	}
}

func assertStrictDERSignature(t *testing.T, signature []byte) {
	t.Helper()
	var decoded struct{ R, S *big.Int }
	rest, err := asn1.Unmarshal(signature, &decoded)
	if err != nil || len(rest) != 0 || decoded.R == nil || decoded.S == nil {
		t.Fatalf("signature is not strict ASN.1 DER: rest=%x decoded=%+v error=%v", rest, decoded, err)
	}
}
