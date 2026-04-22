package runtime

// Adapter parity coverage for step-10. Framework adapters (LangChain,
// CrewAI, AutoGen, OpenAI Agents SDK) re-vend Agent.Start through a
// thin wrapper and sometimes pass explicit SDKVersion + Capabilities
// overrides. These tests confirm every override permutation flows
// through the Phase-2 handshake unchanged so adapter users get
// transparent behaviour: no user action required on upgrade.

import (
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

func inspectAdapterRequest(t *testing.T, data []byte) *capsdk.HandshakeRequest {
	t.Helper()
	req, err := capsdk.UnmarshalHandshakeRequest(data)
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	return req
}

func TestAdapterFlow_ExplicitSDKVersionOverride(t *testing.T) {
	// Adapters like cordum-adapters/crewai pass a custom sdk_version
	// so the scheduler's AgentIdentityStore can tell an MCP-bundled
	// agent from a raw cap-go worker. The request must round-trip
	// the caller's SDKVersion byte-for-byte.
	t.Parallel()
	var observed string
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		observed = inspectAdapterRequest(t, data).SDKVersion
		return acceptResponse(t, data, "token-adapter"), nil
	})
	a := newAgentFixture(t, bus, HandshakeModeWarn)
	a.SDKVersion = "cordum-crewai/0.2.0"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if observed != "cordum-crewai/0.2.0" {
		t.Fatalf("SDKVersion=%q want cordum-crewai/0.2.0", observed)
	}
}

func TestAdapterFlow_ManyCapabilitiesPreserveOrder(t *testing.T) {
	// Adapters that bundle many tools (LangChain-style agents with
	// N tool handlers) must emit a deterministic capability order
	// so the scheduler's audit events and approval workflows get
	// stable Extra.capabilities entries.
	t.Parallel()
	var caps []string
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		caps = append(caps[:0], inspectAdapterRequest(t, data).Capabilities...)
		return acceptResponse(t, data, "token-multi"), nil
	})
	a := &Agent{
		NATS:             bus,
		Store:            NewInMemoryBlobStore(),
		SenderID:         "adapter-langchain",
		Tenant:           "tenant-lc",
		SDKVersion:       "cordum-langchain/0.4.2",
		HandshakeMode:    HandshakeModeWarn,
		HandshakeTimeout: 200 * time.Millisecond,
		HandshakeRetries: 1,
		Logger:           silentLogger(),
	}
	for _, topic := range []string{
		"tool.search.web",
		"tool.files.read",
		"tool.calculator",
		"tool.files.write",
		"tool.shell",
	} {
		Register(a, topic, func(_ Context, _ struct{}) (struct{}, error) { return struct{}{}, nil })
	}
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := []string{"tool.calculator", "tool.files.read", "tool.files.write", "tool.search.web", "tool.shell"}
	if len(caps) != len(want) {
		t.Fatalf("caps len=%d want %d: %+v", len(caps), len(want), caps)
	}
	for i := range want {
		if caps[i] != want[i] {
			t.Errorf("caps[%d]=%q want %q", i, caps[i], want[i])
		}
	}
}

func TestAdapterFlow_TenantSurfacesInRequest(t *testing.T) {
	// OpenAI Agents SDK adapter programmatically sets Tenant. The
	// handshake must surface it verbatim so the scheduler's
	// AgentIdentityStore can look up the tenant-scoped record.
	t.Parallel()
	var observed string
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		observed = inspectAdapterRequest(t, data).Tenant
		return acceptResponse(t, data, "token-openai"), nil
	})
	a := newAgentFixture(t, bus, HandshakeModeWarn)
	a.Tenant = "acme-enterprise"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if observed != "acme-enterprise" {
		t.Fatalf("Tenant=%q want acme-enterprise", observed)
	}
}

func TestAdapterFlow_DownstreamPacketsCarryAdapterToken(t *testing.T) {
	// End-to-end: after the adapter Agent.Start installs a token,
	// every downstream JobResult publish must carry that token on
	// unknown field 18. Verifies the outbound middleware is
	// adapter-agnostic.
	t.Parallel()
	bus := newHandshakeBus(func(_ string, data []byte) ([]byte, error) {
		return acceptResponse(t, data, "adapter-token-42"), nil
	})
	a := newAgentFixture(t, bus, HandshakeModeWarn)
	a.SDKVersion = "cordum-autogen/0.3.1"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx := Context{
		Packet: &agentv1.BusPacket{TraceId: "trace-adapter"},
		Logger: silentLogger(),
	}
	a.publishResult(ctx, &agentv1.JobResult{
		JobId:  "job-adapter",
		Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
	})
	last, ok := bus.lastPublished()
	if !ok {
		t.Fatal("no packet published")
	}
	var decoded agentv1.BusPacket
	if err := proto.Unmarshal(last.data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := ExtractSessionToken(&decoded); got != "adapter-token-42" {
		t.Fatalf("published packet token=%q want adapter-token-42", got)
	}
}
