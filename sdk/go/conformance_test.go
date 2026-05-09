package capsdk

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve fixture path")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	return filepath.Join(root, "spec", "conformance", "fixtures", name)
}

func loadPacket(t *testing.T, name string) *agentv1.BusPacket {
	t.Helper()
	data, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(data, &pkt); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
	return &pkt
}

func loadPublicKey(t *testing.T) *ecdsa.PublicKey {
	t.Helper()
	data, err := os.ReadFile(fixturePath(t, "public_key.pem"))
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("public key PEM decode failed")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	key, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("public key is not ECDSA")
	}
	return key
}

func verifySignature(t *testing.T, pkt *agentv1.BusPacket, pub *ecdsa.PublicKey) {
	t.Helper()
	if len(pkt.GetSignature()) == 0 {
		t.Fatal("missing signature")
	}
	unsigned, err := MarshalUnsignedForSignature(pkt)
	if err != nil {
		t.Fatalf("marshal unsigned: %v", err)
	}
	hash := sha256.Sum256(unsigned)
	if !ecdsa.VerifyASN1(pub, hash[:], pkt.GetSignature()) {
		t.Fatal("signature verification failed")
	}
}

func assertTimestamp(t *testing.T, pkt *agentv1.BusPacket) {
	t.Helper()
	expected := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	if got := pkt.GetCreatedAt().AsTime().UTC(); !got.Equal(expected) {
		t.Fatalf("unexpected created_at: %s", got.Format(time.RFC3339Nano))
	}
	if pkt.GetProtocolVersion() != 1 {
		t.Fatalf("unexpected protocol version: %d", pkt.GetProtocolVersion())
	}
}

func TestConformanceFixtures(t *testing.T) {
	pub := loadPublicKey(t)

	t.Run("job_request", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_job_request.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		req := pkt.GetJobRequest()
		if req == nil {
			t.Fatal("missing job request")
		}
		if req.GetJobId() != "job-req-1" || req.GetTopic() != "job.tools" {
			t.Fatalf("unexpected job request fields: %#v", req)
		}
		if req.GetPriority() != agentv1.JobPriority_JOB_PRIORITY_INTERACTIVE {
			t.Fatalf("unexpected priority: %v", req.GetPriority())
		}
		if req.GetContextPtr() != "redis://ctx:job-req-1" {
			t.Fatalf("unexpected context_ptr: %s", req.GetContextPtr())
		}
		if req.GetEnv()["region"] != "us-east-1" || req.GetEnv()["sandbox"] != "true" {
			t.Fatalf("unexpected env map: %#v", req.GetEnv())
		}
		if req.GetLabels()["env"] != "prod" || req.GetLabels()["team"] != "platform" {
			t.Fatalf("unexpected labels map: %#v", req.GetLabels())
		}
		meta := req.GetMeta()
		if meta == nil || meta.GetIdempotencyKey() != "idem-123" || meta.GetCapability() != "repo.review" {
			t.Fatalf("unexpected meta: %#v", meta)
		}
		if meta.GetLabels()["source"] != "conformance" {
			t.Fatalf("unexpected meta labels: %#v", meta.GetLabels())
		}
		comp := req.GetCompensation()
		if comp == nil || comp.GetTopic() != "job.rollback" {
			t.Fatalf("unexpected compensation: %#v", comp)
		}
		if comp.GetLabels()["rollback"] != "true" {
			t.Fatalf("unexpected compensation labels: %#v", comp.GetLabels())
		}
	})

	t.Run("job_result", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_job_result.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		res := pkt.GetJobResult()
		if res == nil {
			t.Fatal("missing job result")
		}
		if res.GetJobId() != "job-res-1" || res.GetWorkerId() != "worker-1" {
			t.Fatalf("unexpected job result fields: %#v", res)
		}
		if res.GetStatus() != agentv1.JobStatus_JOB_STATUS_FAILED_RETRYABLE {
			t.Fatalf("unexpected job status: %v", res.GetStatus())
		}
		if res.GetErrorCode() != "E_TEMP" {
			t.Fatalf("unexpected error code: %s", res.GetErrorCode())
		}
		if len(res.GetArtifactPtrs()) != 2 {
			t.Fatalf("unexpected artifact_ptrs: %#v", res.GetArtifactPtrs())
		}
	})

	t.Run("heartbeat", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_heartbeat.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		hb := pkt.GetHeartbeat()
		if hb == nil {
			t.Fatal("missing heartbeat")
		}
		if hb.GetWorkerId() != "worker-1" || hb.GetPool() != "job.tools" {
			t.Fatalf("unexpected heartbeat fields: %#v", hb)
		}
		if hb.GetLabels()["zone"] != "us-east-1a" {
			t.Fatalf("unexpected heartbeat labels: %#v", hb.GetLabels())
		}
		if hb.GetAuthToken() != "attest-worker-1" {
			t.Fatalf("unexpected heartbeat auth_token: %q", hb.GetAuthToken())
		}
	})

	t.Run("auth_token", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_auth_token.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		if got := pkt.GetAuthToken(); got != "session-token-fixture" {
			t.Fatalf("unexpected buspacket auth_token: %q", got)
		}
		field := pkt.ProtoReflect().Descriptor().Fields().ByName("auth_token")
		if field == nil || field.Number() != 18 {
			t.Fatalf("auth_token descriptor=%v want field 18", field)
		}
		hb := pkt.GetHeartbeat()
		if hb == nil || hb.GetWorkerId() != "worker-session-1" {
			t.Fatalf("unexpected auth-token fixture heartbeat: %#v", hb)
		}
		if hb.GetAuthToken() != "" {
			t.Fatalf("heartbeat auth_token=%q want empty", hb.GetAuthToken())
		}
	})

	t.Run("job_progress", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_job_progress.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		progress := pkt.GetJobProgress()
		if progress == nil {
			t.Fatal("missing job progress")
		}
		if progress.GetJobId() != "job-prog-1" || progress.GetPercent() != 50 {
			t.Fatalf("unexpected job progress fields: %#v", progress)
		}
		if progress.GetStatus() != agentv1.JobStatus_JOB_STATUS_RUNNING {
			t.Fatalf("unexpected job progress status: %v", progress.GetStatus())
		}
	})

	t.Run("job_cancel", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_job_cancel.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		cancel := pkt.GetJobCancel()
		if cancel == nil {
			t.Fatal("missing job cancel")
		}
		if cancel.GetJobId() != "job-cancel-1" || cancel.GetRequestedBy() != "user-7" {
			t.Fatalf("unexpected job cancel fields: %#v", cancel)
		}
	})

	t.Run("alert", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_alert.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		alert := pkt.GetAlert()
		if alert == nil {
			t.Fatal("missing alert")
		}
		if alert.GetLevel() != "WARN" || alert.GetComponent() != "scheduler" {
			t.Fatalf("unexpected alert fields: %#v", alert)
		}
	})

	t.Run("handshake", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_handshake.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		hs := pkt.GetHandshake()
		if hs == nil {
			t.Fatal("missing handshake")
		}
		if hs.GetComponentId() != "worker-1" {
			t.Fatalf("unexpected component_id: %s", hs.GetComponentId())
		}
		if hs.GetRole() != agentv1.ComponentRole_COMPONENT_ROLE_WORKER {
			t.Fatalf("unexpected role: %v", hs.GetRole())
		}
		if len(hs.GetSupportedVersions()) != 1 || hs.GetSupportedVersions()[0] != 1 {
			t.Fatalf("unexpected supported_versions: %v", hs.GetSupportedVersions())
		}
		caps := hs.GetCapabilities()
		if !caps["signatures"] || !caps["progress"] || !caps["cancel"] || caps["compensation"] {
			t.Fatalf("unexpected capabilities: %v", caps)
		}
		if hs.GetSdkVersion() != "2.0.19" {
			t.Fatalf("unexpected sdk_version: %s", hs.GetSdkVersion())
		}
		if got, want := hs.GetReadyTopics(), []string{"job.tools", "job.tools.bulk"}; len(got) != len(want) {
			t.Fatalf("unexpected ready_topics len: %v", got)
		} else {
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("unexpected ready_topics[%d]: got %q want %q", i, got[i], want[i])
				}
			}
		}
	})

	t.Run("alert_enhanced", func(t *testing.T) {
		pkt := loadPacket(t, "buspacket_alert_enhanced.bin")
		verifySignature(t, pkt, pub)
		assertTimestamp(t, pkt)
		alert := pkt.GetAlert()
		if alert == nil {
			t.Fatal("missing alert")
		}
		// Legacy fields
		if alert.GetLevel() != "CRITICAL" || alert.GetComponent() != "scheduler" || alert.GetCode() != "SIGNATURE_INVALID" {
			t.Fatalf("unexpected legacy alert fields: level=%s component=%s code=%s", alert.GetLevel(), alert.GetComponent(), alert.GetCode())
		}
		// Enhanced fields
		if alert.GetSeverity() != agentv1.AlertSeverity_ALERT_SEVERITY_CRITICAL {
			t.Fatalf("unexpected severity: %v", alert.GetSeverity())
		}
		if alert.GetErrorCodeEnum() != agentv1.ErrorCode_ERROR_CODE_PROTOCOL_SIGNATURE_INVALID {
			t.Fatalf("unexpected error_code_enum: %v", alert.GetErrorCodeEnum())
		}
		if alert.GetSourceComponent() != "scheduler-1" {
			t.Fatalf("unexpected source_component: %s", alert.GetSourceComponent())
		}
		details := alert.GetDetails()
		if details["sender"] != "worker-bad" || details["subject"] != "sys.job.result" {
			t.Fatalf("unexpected details: %v", details)
		}
		if alert.GetTraceId() != "trace-offending-packet" {
			t.Fatalf("unexpected trace_id: %s", alert.GetTraceId())
		}
	})
}

// Policy Studio (epic-d9a6c0a1) — exercises the standalone-message
// fixtures from spec/conformance/fixtures/policy_*.bin. These are NOT
// BusPacket-wrapped (Rule/Decision/Bundle are higher-layer API shapes,
// not bus payloads); the test verifies binary roundtrip parity for the
// new types + the appended DecisionType enum values (QUARANTINE,
// REDACT) + the new EdgeMode enum.
func TestPolicyShapesConformance(t *testing.T) {
	t.Run("rule", func(t *testing.T) {
		data, err := os.ReadFile(fixturePath(t, "policy_rule.bin"))
		if err != nil {
			t.Fatalf("read policy_rule.bin: %v", err)
		}
		var rule agentv1.Rule
		if err := proto.Unmarshal(data, &rule); err != nil {
			t.Fatalf("unmarshal Rule: %v", err)
		}
		if rule.GetId() != "rule-input-secrets" {
			t.Fatalf("Rule.id = %q, want rule-input-secrets", rule.GetId())
		}
		if rule.GetType() != agentv1.RuleType_RULE_TYPE_INPUT {
			t.Fatalf("Rule.type = %v, want INPUT", rule.GetType())
		}
		if rule.GetScope().GetKind() != agentv1.RuleScopeKind_RULE_SCOPE_KIND_TENANT {
			t.Fatalf("Rule.scope.kind = %v, want TENANT", rule.GetScope().GetKind())
		}
		if rule.GetStatus() != agentv1.RuleStatus_RULE_STATUS_PUBLISHED {
			t.Fatalf("Rule.status = %v, want PUBLISHED", rule.GetStatus())
		}
		if rule.GetMatch() == nil || rule.GetDecide() == nil {
			t.Fatal("Rule.match and Rule.decide must be non-nil structpb")
		}
	})

	t.Run("decision_quarantine", func(t *testing.T) {
		data, err := os.ReadFile(fixturePath(t, "policy_decision.bin"))
		if err != nil {
			t.Fatalf("read policy_decision.bin: %v", err)
		}
		var dec agentv1.Decision
		if err := proto.Unmarshal(data, &dec); err != nil {
			t.Fatalf("unmarshal Decision: %v", err)
		}
		if dec.GetType() != agentv1.DecisionType_DECISION_TYPE_QUARANTINE {
			t.Fatalf("Decision.type = %v, want QUARANTINE (6)", dec.GetType())
		}
		if dec.GetSource() != agentv1.DecisionSource_DECISION_SOURCE_JOB {
			t.Fatalf("Decision.source = %v, want JOB", dec.GetSource())
		}
		if got := len(dec.GetTrace()); got != 1 {
			t.Fatalf("Decision.trace len = %d, want 1", got)
		}
		if dec.GetTrace()[0].GetDecisionType() != agentv1.DecisionType_DECISION_TYPE_QUARANTINE {
			t.Fatalf("TraceStep.decision_type = %v, want QUARANTINE", dec.GetTrace()[0].GetDecisionType())
		}
	})

	t.Run("bundle_edge_mode", func(t *testing.T) {
		data, err := os.ReadFile(fixturePath(t, "policy_bundle.bin"))
		if err != nil {
			t.Fatalf("read policy_bundle.bin: %v", err)
		}
		var b agentv1.Bundle
		if err := proto.Unmarshal(data, &b); err != nil {
			t.Fatalf("unmarshal Bundle: %v", err)
		}
		if b.GetMetadata().GetEdgeMode() != agentv1.EdgeMode_EDGE_MODE_ENTERPRISE_STRICT {
			t.Fatalf("Bundle.metadata.edge_mode = %v, want ENTERPRISE_STRICT", b.GetMetadata().GetEdgeMode())
		}
		if b.GetScopeBinding().GetKind() != agentv1.RuleScopeKind_RULE_SCOPE_KIND_EDGE_FLEET {
			t.Fatalf("Bundle.scope_binding.kind = %v, want EDGE_FLEET", b.GetScopeBinding().GetKind())
		}
		if got := len(b.GetVersions()); got != 1 {
			t.Fatalf("Bundle.versions len = %d, want 1", got)
		}
	})

	t.Run("decision_type_redact_round_trip", func(t *testing.T) {
		// Build + marshal + unmarshal a DecisionType=REDACT instance to
		// exercise the wire path for the second new enum value (the
		// QUARANTINE path is exercised by decision_quarantine above).
		dec := &agentv1.Decision{
			Source:    agentv1.DecisionSource_DECISION_SOURCE_JOB,
			RuleId:    "rule-redact",
			Type:      agentv1.DecisionType_DECISION_TYPE_REDACT,
			Timestamp: nil,
		}
		raw, err := proto.Marshal(dec)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var back agentv1.Decision
		if err := proto.Unmarshal(raw, &back); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if back.GetType() != agentv1.DecisionType_DECISION_TYPE_REDACT {
			t.Fatalf("round-trip Decision.type = %v, want REDACT (7)", back.GetType())
		}
	})
}
