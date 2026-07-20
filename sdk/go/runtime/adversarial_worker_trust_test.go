package runtime

import (
	"errors"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func TestAgentZeroHandshakeModePreservesLegacyOff(t *testing.T) {
	t.Setenv(EnvHandshakeMode, "")
	agent := &Agent{}
	mode, err := agent.resolveHandshakeMode()
	if err != nil || mode != HandshakeModeOff {
		t.Fatalf("resolveHandshakeMode()=(%q,%v), want (%q,nil)", mode, err, HandshakeModeOff)
	}
}

func TestHandshakeRejectedErrorRetainsLegacyFields(t *testing.T) {
	err := &HandshakeRejectedError{
		Reason: capsdk.HandshakeRejectReplay, RequestID: "request-legacy", AgentID: "agent-legacy",
	}
	if err.Reason != capsdk.HandshakeRejectReplay || err.RequestID != "request-legacy" || err.AgentID != "agent-legacy" {
		t.Fatalf("legacy rejection fields changed: %#v", err)
	}
	want := "cap-runtime: handshake rejected: reason=replay_detected agent_id=agent-legacy request_id=request-legacy"
	if got := err.Error(); got != want {
		t.Fatalf("Error()=%q, want %q", got, want)
	}
}

func TestAgentRejectionPreservesLegacyCorrelationFields(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeWarn)
	bus.rejectResult = true
	err := agent.Start()
	var rejected *HandshakeRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("Start() error=%v, want HandshakeRejectedError", err)
	}
	if rejected.Reason != capsdk.HandshakeRejectReplay || rejected.RequestID == "" ||
		rejected.AgentID != bus.config.ExpectedAgentID || rejected.WorkerID != bus.config.WorkerID {
		t.Fatalf("rejection fields=%#v", rejected)
	}
}

func TestAgentWarnOperationalTransportFailureContinuesTokenless(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeWarn)
	bus.requestErr = errors.New("trust transport unavailable")
	if err := agent.Start(); err != nil {
		t.Fatalf("operational WARN failure blocked startup: %v", err)
	}
	t.Cleanup(func() { _ = agent.Close() })
	if token, _ := agent.SessionToken(); token != "" {
		t.Fatalf("operational failure installed token %q", token)
	}
	if got := len(bus.subs); got != 1 {
		t.Fatalf("subscriptions=%d, want 1", got)
	}
}

func TestAgentWarnDoesNotLaunderTypedSecurityErrorsFromRequester(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeWarn)
	bus.requestErr = capsdk.ErrInvalidSignature
	if err := agent.Start(); err == nil {
		t.Fatal("WARN laundered a typed requester security failure")
	}
	if got := len(bus.subs); got != 0 {
		t.Fatalf("security failure installed %d subscriptions", got)
	}
}

func TestAgentSecurityFailureIsTerminalWithoutRetry(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeWarn)
	agent.HandshakeRetries = 3
	bus.unsignedResult = true
	if err := agent.Start(); err == nil {
		t.Fatal("WARN accepted a security failure")
	}
	if got := len(bus.purposes); got != 1 {
		t.Fatalf("security failure attempted %d handshakes, want 1", got)
	}
}

func TestWarnRenewSecurityFailureClearsSessionAndAdmissions(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeWarn)
	if err := agent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agent.Close() })
	bus.unsignedResult = true
	if obtained, err := agent.performRenew(nil); err == nil || obtained {
		t.Fatalf("security renew=(%v,%v), want (false,error)", obtained, err)
	}
	if token, _ := agent.SessionToken(); token != "" {
		t.Fatalf("security renewal retained token %q", token)
	}
	if len(agent.subs) != 0 {
		t.Fatalf("security renewal retained %d job admissions", len(agent.subs))
	}
}

func TestAgentFreezesHandshakeModeAfterStart(t *testing.T) {
	agent, _ := newTrustTestAgent(t, HandshakeModeEnforce)
	if err := agent.Start(); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	t.Cleanup(func() { _ = agent.Close() })

	agent.HandshakeMode = HandshakeModeOff
	if got := agent.activeHandshakeMode(); got != HandshakeModeEnforce {
		t.Fatalf("active mode after caller mutation=%q, want %q", got, HandshakeModeEnforce)
	}
}

func TestAgentFreezesEnvironmentHandshakeModeAfterStart(t *testing.T) {
	t.Setenv(EnvHandshakeMode, string(HandshakeModeEnforce))
	agent, _ := newTrustTestAgent(t, HandshakeModeEnforce)
	agent.HandshakeMode = ""
	if err := agent.Start(); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	t.Cleanup(func() { _ = agent.Close() })
	t.Setenv(EnvHandshakeMode, string(HandshakeModeOff))
	if got := agent.activeHandshakeMode(); got != HandshakeModeEnforce {
		t.Fatalf("active mode after environment mutation=%q, want %q", got, HandshakeModeEnforce)
	}
}

func TestAgentTrustModesDoNotPublishTokenlessResults(t *testing.T) {
	for _, mode := range []HandshakeMode{HandshakeModeWarn, HandshakeModeEnforce} {
		t.Run(string(mode), func(t *testing.T) {
			bus := newMockNATS()
			agent := &Agent{NATS: bus, SenderID: "worker-secure", HandshakeMode: mode, Logger: silentLogger()}
			ctx := Context{Packet: &agentv1.BusPacket{TraceId: "trace-secure"}, Logger: silentLogger()}
			result := &agentv1.JobResult{JobId: "job-secure", WorkerId: "worker-secure", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}

			agent.publishResult(ctx, result)
			if message, ok := bus.lastPublished(); ok {
				t.Fatalf("published tokenless result on %s in %s mode", message.subject, mode)
			}
		})
	}
}

func TestAgentCloseClearsSessionAfterShutdown(t *testing.T) {
	agent, _ := newTrustTestAgent(t, HandshakeModeEnforce)
	if err := agent.Start(); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	if token, _ := agent.SessionToken(); token == "" {
		t.Fatal("Start() did not establish a session")
	}
	if err := agent.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if token, expiry := agent.SessionToken(); token != "" || !expiry.IsZero() {
		t.Fatalf("session after Close()=(%q,%v), want empty", token, expiry)
	}
}

func TestAgentEnforceResultStopsAfterRenewalClearsSession(t *testing.T) {
	agent, bus := newTrustTestAgent(t, HandshakeModeEnforce)
	agent.setSession("session-before-renew", time.Now().Add(time.Hour))
	agent.clearSession()
	ctx := Context{Packet: &agentv1.BusPacket{TraceId: "trace-renew"}, Logger: silentLogger()}
	result := &agentv1.JobResult{JobId: "job-renew", WorkerId: agent.WorkerTrust.WorkerID, Status: agentv1.JobStatus_JOB_STATUS_FAILED}

	agent.publishResult(ctx, result)
	for _, message := range bus.published {
		if message.subject == capsdk.SubjectResult {
			t.Fatal("renewal-cleared session emitted a tokenless result")
		}
	}
}
