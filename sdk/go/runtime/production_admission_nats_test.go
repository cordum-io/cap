package runtime

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const productionNATSTestSubject = "job.production.admission.e2e"

type productionNATSFixture struct {
	t       *testing.T
	key     *ecdsa.PrivateKey
	client  *nats.Conn
	store   *InMemoryBlobStore
	handled chan string
}

func TestProductionAdmissionRealNATSRejectsUnsignedBeforeHandler(t *testing.T) {
	fixture := newProductionNATSFixture(t)
	raw, err := proto.Marshal(fixture.packet("job-unsigned", 1))
	if err != nil {
		t.Fatalf("marshal unsigned packet: %v", err)
	}

	handled := fixture.publishBeforeBarrier(raw)
	assertHandledJobs(t, handled, "job-barrier")
}

func TestProductionAdmissionRealNATSRejectsTamperingBeforeHandler(t *testing.T) {
	fixture := newProductionNATSFixture(t)
	raw := fixture.signedPacket("job-tampered", 2)
	tampered := tamperProductionPacket(t, raw, "job-tampered")

	handled := fixture.publishBeforeBarrier(tampered)
	assertHandledJobs(t, handled, "job-barrier")
}

func TestProductionAdmissionRealNATSRejectsReplayConflictBeforeHandler(t *testing.T) {
	fixture := newProductionNATSFixture(t)
	first := fixture.signedPacket("job-first", 3)
	conflict := fixture.signedPacket("job-conflict", 3)

	handled := fixture.publishBeforeBarrier(first, conflict)
	assertHandledJobs(t, handled, "job-first", "job-barrier")
}

func TestProductionAdmissionRealNATSIdenticalRedeliveryIsHarmless(t *testing.T) {
	fixture := newProductionNATSFixture(t)
	raw := fixture.signedPacket("job-redelivery", 4)

	handled := fixture.publishBeforeBarrier(raw, raw)
	assertHandledJobs(t, handled, "job-redelivery", "job-barrier")
}

func newProductionNATSFixture(t *testing.T) *productionNATSFixture {
	t.Helper()
	url := startProductionNATSServer(t)
	client := connectProductionNATSClient(t, url)
	key := productionNATSTestKey(t)
	proofKey := productionNATSTestKey(t)
	trust := capsdk.WorkerTrustConfig{
		WorkerID: "worker-production-e2e", ExpectedAgentID: "agent-production-e2e", TenantID: "tenant-a",
		Audience: capsdk.WorkerHandshakeAudience, ProofKeyID: "proof-production-e2e", ProofPrivateKey: proofKey,
		ExpectedSchedulerID: "scheduler-1",
		SchedulerPublicKeys: map[string]*ecdsa.PublicKey{"scheduler-runtime-key": &key.PublicKey},
		SDKVersion:          "cap-go/runtime-production-e2e",
	}
	installProductionTrustResponder(t, client, trust, key)
	store := NewInMemoryBlobStore()
	fixture := &productionNATSFixture{
		t: t, key: key, client: client, store: store, handled: make(chan string, 8),
	}
	agent := &Agent{
		NATSURL: url, Store: store, SenderID: trust.WorkerID, Logger: silentLogger(), PrivateKey: proofKey,
		HandshakeMode: HandshakeModeEnforce, WorkerTrust: trust,
		HandshakeTimeout: 2 * time.Second, HandshakeRetries: 1,
		Production: ProductionAdmission{Enabled: true, Replay: capsdk.NewInMemoryReplayStore(),
			Trust: capsdk.ProductionTrustStore{Audience: productionNATSTestSubject,
				Tenant: "tenant-a", Sender: "scheduler-1", MaxLifetime: 5 * time.Minute,
				PublicKeys: map[string]*ecdsa.PublicKey{"scheduler-key": &key.PublicKey}}},
	}
	Register(agent, productionNATSTestSubject, fixture.recordHandler)
	if err := agent.Start(); err != nil {
		t.Fatalf("start production agent: %v", err)
	}
	if token, expiry := agent.SessionToken(); token == "" || !expiry.After(time.Now()) {
		t.Fatal("real-NATS ENFORCE handshake did not install a live session")
	}
	t.Cleanup(func() { _ = agent.Close() })
	flushProductionSubscription(t, agent)
	return fixture
}

func installProductionTrustResponder(
	t *testing.T,
	client *nats.Conn,
	config capsdk.WorkerTrustConfig,
	schedulerKey *ecdsa.PrivateKey,
) {
	t.Helper()
	builder := &trustTestBus{config: config, schedulerKey: schedulerKey}
	handler := func(message *nats.Msg) {
		response, err := builder.Request(message.Subject, message.Data, time.Second)
		if err != nil {
			t.Errorf("build worker-trust response on %s: %v", message.Subject, err)
			return
		}
		if response == nil {
			t.Errorf("worker-trust response on %s is nil", message.Subject)
			return
		}
		if err := message.Respond(response.Data); err != nil {
			t.Errorf("publish worker-trust response on %s: %v", message.Subject, err)
		}
	}
	for _, subject := range []string{
		capsdk.WorkerHandshakeChallengeSubject,
		capsdk.WorkerHandshakeAuthenticateSubject,
	} {
		subscription, err := client.Subscribe(subject, handler)
		if err != nil {
			t.Fatalf("subscribe worker-trust responder on %s: %v", subject, err)
		}
		t.Cleanup(func() { _ = subscription.Unsubscribe() })
	}
	if err := client.FlushTimeout(2 * time.Second); err != nil {
		t.Fatalf("flush worker-trust responder subscriptions: %v", err)
	}
}

func (f *productionNATSFixture) recordHandler(ctx Context, _ struct{}) (struct{}, error) {
	f.handled <- ctx.Job.GetJobId()
	return struct{}{}, nil
}

func (f *productionNATSFixture) packet(jobID string, messageNumber int) *agentv1.BusPacket {
	identity := &agentv1.IdentityBinding{TenantId: "tenant-a", PrincipalId: "principal-a"}
	return &agentv1.BusPacket{
		TraceId: "trace-" + jobID, SenderId: "scheduler-1", CreatedAt: timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion, Identity: identity,
		SignatureMetadata: &agentv1.SignatureMetadata{
			ProfileVersion: capsdk.ProductionProfileVersion, Algorithm: capsdk.ProductionAlgorithm,
			MessageId: []byte(fmt.Sprintf("%016d", messageNumber)), Audience: productionNATSTestSubject,
			ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)), KeyId: "scheduler-key",
		},
		Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{
			JobId: jobID, Topic: productionNATSTestSubject, ContextPtr: "redis://ctx:" + jobID,
			TenantId: "tenant-a", Identity: proto.Clone(identity).(*agentv1.IdentityBinding),
		}},
	}
}

func (f *productionNATSFixture) signedPacket(jobID string, messageNumber int) []byte {
	f.t.Helper()
	if err := f.store.Set(context.Background(), "ctx:"+jobID, []byte("{}")); err != nil {
		f.t.Fatalf("store context for %s: %v", jobID, err)
	}
	raw, err := capsdk.SignProductionPacket(f.packet(jobID, messageNumber), f.key)
	if err != nil {
		f.t.Fatalf("sign packet for %s: %v", jobID, err)
	}
	return raw
}

func (f *productionNATSFixture) publishBeforeBarrier(packets ...[]byte) []string {
	f.t.Helper()
	for _, raw := range packets {
		if err := f.client.Publish(productionNATSTestSubject, raw); err != nil {
			f.t.Fatalf("publish test packet: %v", err)
		}
	}
	barrier := f.signedPacket("job-barrier", 999)
	if err := f.client.Publish(productionNATSTestSubject, barrier); err != nil {
		f.t.Fatalf("publish barrier packet: %v", err)
	}
	if err := f.client.FlushTimeout(2 * time.Second); err != nil {
		f.t.Fatalf("flush publisher: %v", err)
	}
	return f.awaitBarrier()
}

func (f *productionNATSFixture) awaitBarrier() []string {
	f.t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	var handled []string
	for {
		select {
		case jobID := <-f.handled:
			handled = append(handled, jobID)
			if jobID == "job-barrier" {
				return handled
			}
		case <-timer.C:
			f.t.Fatalf("timed out waiting for barrier; handled=%v", handled)
		}
	}
}

func startProductionNATSServer(t *testing.T) string {
	t.Helper()
	ns, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true,
	})
	if err != nil {
		t.Fatalf("create embedded NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("embedded NATS server not ready")
	}
	t.Cleanup(ns.Shutdown)
	return ns.ClientURL()
}

func connectProductionNATSClient(t *testing.T, url string) *nats.Conn {
	t.Helper()
	client, err := nats.Connect(url, nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect NATS publisher: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func flushProductionSubscription(t *testing.T, agent *Agent) {
	t.Helper()
	client, ok := agent.NATS.(*nats.Conn)
	if !ok {
		t.Fatalf("agent NATS connection has type %T, want *nats.Conn", agent.NATS)
	}
	if err := client.FlushTimeout(2 * time.Second); err != nil {
		t.Fatalf("flush agent subscription: %v", err)
	}
}

func productionNATSTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}
	return key
}

func tamperProductionPacket(t *testing.T, raw []byte, marker string) []byte {
	t.Helper()
	tampered := append([]byte(nil), raw...)
	index := bytes.Index(tampered, []byte(marker))
	if index < 0 {
		t.Fatalf("signed packet does not contain marker %q", marker)
	}
	tampered[index] ^= 0x01
	return tampered
}

func assertHandledJobs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("handled jobs = %v, want %v", got, want)
	}
}
