package runtime

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type trustTestBus struct {
	mockNATS
	mu             sync.Mutex
	config         capsdk.WorkerTrustConfig
	schedulerKey   *ecdsa.PrivateKey
	order          []string
	purposes       []agentv1.WorkerHandshakePurpose
	traceIDs       []string
	authTokens     []string
	capability     *agentv1.Handshake
	unsignedResult bool
	nilResponse    bool
}

func newTrustTestAgent(t *testing.T, mode HandshakeMode) (*Agent, *trustTestBus) {
	t.Helper()
	proof := testP256Key(t)
	scheduler := testP256Key(t)
	config := capsdk.WorkerTrustConfig{
		WorkerID: "worker-runtime", ExpectedAgentID: "agent-runtime", TenantID: "tenant-runtime",
		Audience: capsdk.WorkerHandshakeAudience, ProofKeyID: "proof-runtime", ProofPrivateKey: proof,
		ExpectedSchedulerID: "scheduler-runtime",
		SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"scheduler-runtime-key": &scheduler.PublicKey},
		SDKVersion:          "cap-go/runtime-test",
	}
	bus := &trustTestBus{mockNATS: *newMockNATS(), config: config, schedulerKey: scheduler}
	agent := &Agent{
		NATS: bus, Store: NewInMemoryBlobStore(), HandshakeMode: mode, WorkerTrust: config,
		HandshakeTimeout: 200 * time.Millisecond, HandshakeRetries: 1, Logger: silentLogger(),
	}
	Register(agent, "job.runtime", func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	return agent, bus
}

func (b *trustTestBus) QueueSubscribe(subject, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
	b.record("subscribe:" + subject)
	return b.mockNATS.QueueSubscribe(subject, queue, cb)
}

func (b *trustTestBus) Request(subject string, data []byte, _ time.Duration) (*nats.Msg, error) {
	b.record("request:" + subject)
	b.mu.Lock()
	nilResponse := b.nilResponse
	b.mu.Unlock()
	if nilResponse {
		return nil, nil
	}
	packet, err := capsdk.UnmarshalWorkerTrustPacket(data)
	if err != nil {
		return nil, err
	}
	switch subject {
	case capsdk.WorkerHandshakeChallengeSubject:
		return b.challenge(packet)
	case capsdk.WorkerHandshakeAuthenticateSubject:
		return b.result(packet)
	default:
		return nil, errors.New("unexpected trust subject")
	}
}

func (b *trustTestBus) challenge(request *agentv1.BusPacket) (*nats.Msg, error) {
	if err := capsdk.VerifyTrustHandshake(request, map[string]*ecdsa.PublicKey{
		b.config.ProofKeyID: &b.config.ProofPrivateKey.PublicKey,
	}); err != nil {
		return nil, err
	}
	r := request.GetWorkerHandshakeChallengeRequest()
	b.mu.Lock()
	b.purposes = append(b.purposes, r.GetPurpose())
	b.traceIDs = append(b.traceIDs, request.GetTraceId())
	b.mu.Unlock()
	now := time.Now().UTC()
	challenge := &agentv1.WorkerHandshakeChallenge{
		RequestId: r.RequestId, ChallengeId: "challenge-runtime", TraceId: r.TraceId,
		WorkerId: r.WorkerId, AgentId: b.config.ExpectedAgentID, TenantId: b.config.TenantID,
		ProofKeyId: r.ProofKeyId, ProofAlgorithm: r.ProofAlgorithm, ServerKeyId: "scheduler-runtime-key",
		Audience: r.Audience, Purpose: r.Purpose, ClientNonce: bytes.Clone(r.ClientNonce),
		ServerNonce:     bytes.Repeat([]byte{0x52}, capsdk.WorkerHandshakeNonceSize),
		ProtocolVersion: r.ProtocolVersion, SdkVersion: r.SdkVersion,
		IssuedAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(30 * time.Second)),
	}
	response := trustPacket(r.TraceId, b.config.ExpectedSchedulerID)
	response.Payload = &agentv1.BusPacket_WorkerHandshakeChallenge{WorkerHandshakeChallenge: challenge}
	return b.signedMessage(response, false)
}

func (b *trustTestBus) result(authenticate *agentv1.BusPacket) (*nats.Msg, error) {
	if err := capsdk.VerifyTrustHandshake(authenticate, map[string]*ecdsa.PublicKey{
		b.config.ProofKeyID: &b.config.ProofPrivateKey.PublicKey,
	}); err != nil {
		return nil, err
	}
	b.mu.Lock()
	b.authTokens = append(b.authTokens, authenticate.GetAuthToken())
	b.traceIDs = append(b.traceIDs, authenticate.GetTraceId())
	b.capability = proto.Clone(authenticate.GetWorkerHandshakeAuthenticate().GetCapabilityHandshake()).(*agentv1.Handshake)
	unsigned := b.unsignedResult
	b.mu.Unlock()
	now := time.Now().UTC()
	challenge := authenticate.GetWorkerHandshakeAuthenticate().GetChallenge()
	response := trustPacket(challenge.GetTraceId(), b.config.ExpectedSchedulerID)
	response.Payload = &agentv1.BusPacket_WorkerHandshakeResult{WorkerHandshakeResult: &agentv1.WorkerHandshakeResult{
		Challenge: proto.Clone(challenge).(*agentv1.WorkerHandshakeChallenge), Accepted: true,
		IssuedAt: timestamppb.New(now), TokenExpiresAt: timestamppb.New(now.Add(time.Hour)),
	}}
	response.AuthToken = "session-" + challenge.GetPurpose().String()
	return b.signedMessage(response, unsigned)
}

func (b *trustTestBus) signedMessage(packet *agentv1.BusPacket, unsigned bool) (*nats.Msg, error) {
	if !unsigned {
		if err := capsdk.SignTrustHandshake(packet, b.schedulerKey); err != nil {
			return nil, err
		}
	}
	data, err := capsdk.MarshalWorkerTrustPacket(packet)
	return &nats.Msg{Data: data}, err
}

func (b *trustTestBus) record(value string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.order = append(b.order, value)
}

func trustPacket(traceID, senderID string) *agentv1.BusPacket {
	return &agentv1.BusPacket{TraceId: traceID, SenderId: senderID, CreatedAt: timestamppb.Now(),
		ProtocolVersion: capsdk.WorkerHandshakeProtocolVersion}
}

func testP256Key(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}
