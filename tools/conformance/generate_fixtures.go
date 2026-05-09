package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testPrivateKey() *ecdsa.PrivateKey {
	curve := elliptic.P256()
	d := big.NewInt(1)
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}, D: d}
}

func marshalDeterministic(msg proto.Message) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(msg)
}

func signPacket(pkt *agentv1.BusPacket, priv *ecdsa.PrivateKey) error {
	clone := proto.Clone(pkt).(*agentv1.BusPacket)
	clone.Signature = nil
	unsigned, err := marshalDeterministic(clone)
	if err != nil {
		return fmt.Errorf("marshal unsigned: %w", err)
	}
	hash := sha256.Sum256(unsigned)
	sig, err := priv.Sign(nil, hash[:], crypto.SHA256)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	pkt.Signature = sig
	return nil
}

func writePacket(path string, pkt *agentv1.BusPacket, priv *ecdsa.PrivateKey) error {
	if err := signPacket(pkt, priv); err != nil {
		return err
	}
	data, err := marshalDeterministic(pkt)
	if err != nil {
		return fmt.Errorf("marshal packet: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writePublicKey(path string, key *ecdsa.PublicKey) error {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return fmt.Errorf("marshal public key: %w", err)
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	out := pem.EncodeToMemory(block)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}
	return nil
}

func main() {
	root := repoRoot()
	outDir := filepath.Join(root, "spec", "conformance", "fixtures")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	priv := testPrivateKey()
	if err := writePublicKey(filepath.Join(outDir, "public_key.pem"), &priv.PublicKey); err != nil {
		fatal(err)
	}

	timestamp := timestamppb.New(time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC))

	fixtures := []struct {
		Name   string
		Packet *agentv1.BusPacket
	}{
		{
			Name: "buspacket_job_request.bin",
			Packet: &agentv1.BusPacket{
				TraceId:         "trace-job-request",
				SenderId:        "client-1",
				CreatedAt:       timestamp,
				ProtocolVersion: 1,
				Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{
					JobId:      "job-req-1",
					Topic:      "job.tools",
					Priority:   agentv1.JobPriority_JOB_PRIORITY_INTERACTIVE,
					ContextPtr: "redis://ctx:job-req-1",
					AdapterId:  "adapter-default",
					Env: map[string]string{
						"region":  "us-east-1",
						"sandbox": "true",
					},
					ParentJobId: "parent-0",
					WorkflowId:  "wf-1",
					StepIndex:   2,
					MemoryId:    "mem-abc",
					ContextHints: &agentv1.ContextHints{
						MaxInputTokens:     2048,
						AllowSummarization: true,
						AllowRetrieval:     true,
						Tags:               []string{"repo", "docs"},
					},
					Budget: &agentv1.Budget{
						MaxInputTokens:  4000,
						MaxOutputTokens: 1000,
						MaxTotalTokens:  5000,
						DeadlineMs:      60000,
					},
					TenantId:    "tenant-1",
					PrincipalId: "user-99",
					Labels: map[string]string{
						"env":  "prod",
						"team": "platform",
					},
					Meta: &agentv1.JobMetadata{
						TenantId:       "tenant-1",
						ActorId:        "user-99",
						ActorType:      agentv1.ActorType_ACTOR_TYPE_HUMAN,
						IdempotencyKey: "idem-123",
						Capability:     "repo.review",
						RiskTags:       []string{"code", "secrets"},
						Requires:       []string{"git", "network-egress"},
						PackId:         "pack-1",
						Labels: map[string]string{
							"source": "conformance",
						},
					},
					Compensation: &agentv1.Compensation{
						Topic:      "job.rollback",
						ContextPtr: "redis://ctx:job-req-1:rollback",
						Priority:   agentv1.JobPriority_JOB_PRIORITY_BATCH,
						AdapterId:  "adapter-rollback",
						Env: map[string]string{
							"reason": "failure",
						},
						MemoryId: "mem-rollback",
						ContextHints: &agentv1.ContextHints{
							MaxInputTokens:     512,
							AllowSummarization: false,
							AllowRetrieval:     false,
							Tags:               []string{"audit"},
						},
						Budget: &agentv1.Budget{
							MaxTotalTokens: 1000,
							DeadlineMs:     120000,
						},
						TenantId:    "tenant-1",
						PrincipalId: "svc-1",
						Labels: map[string]string{
							"rollback": "true",
						},
						Meta: &agentv1.JobMetadata{
							TenantId:       "tenant-1",
							ActorId:        "svc-1",
							ActorType:      agentv1.ActorType_ACTOR_TYPE_SERVICE,
							IdempotencyKey: "idem-rollback",
							Capability:     "rollback",
							RiskTags:       []string{"write"},
							Requires:       []string{"git"},
							PackId:         "pack-rollback",
						},
					},
				}},
			},
		},
		{
			Name: "buspacket_job_result.bin",
			Packet: &agentv1.BusPacket{
				TraceId:         "trace-job-result",
				SenderId:        "worker-1",
				CreatedAt:       timestamp,
				ProtocolVersion: 1,
				Payload: &agentv1.BusPacket_JobResult{JobResult: &agentv1.JobResult{
					JobId:        "job-res-1",
					Status:       agentv1.JobStatus_JOB_STATUS_FAILED_RETRYABLE,
					ResultPtr:    "redis://res:job-res-1",
					WorkerId:     "worker-1",
					ExecutionMs:  1234,
					ErrorCode:    "E_TEMP",
					ErrorMessage: "temporary failure",
					ArtifactPtrs: []string{"redis://art:log-1", "redis://art:diff-2"},
				}},
			},
		},
		{
			Name: "buspacket_heartbeat.bin",
			Packet: &agentv1.BusPacket{
				TraceId:         "trace-heartbeat",
				SenderId:        "worker-1",
				CreatedAt:       timestamp,
				ProtocolVersion: 1,
				Payload: &agentv1.BusPacket_Heartbeat{Heartbeat: &agentv1.Heartbeat{
					WorkerId:        "worker-1",
					Region:          "us-east-1",
					Type:            "cpu-tools",
					CpuLoad:         33.5,
					GpuUtilization:  0,
					ActiveJobs:      2,
					Capabilities:    []string{"git", "docker"},
					Pool:            "job.tools",
					MaxParallelJobs: 4,
					Labels: map[string]string{
						"env":  "prod",
						"zone": "us-east-1a",
					},
					MemoryLoad:  45.2,
					ProgressPct: 60,
					LastMemo:    "step-3",
					AuthToken:   "attest-worker-1",
				}},
			},
		},
		{
			Name: "buspacket_job_progress.bin",
			Packet: &agentv1.BusPacket{
				TraceId:         "trace-progress",
				SenderId:        "worker-1",
				CreatedAt:       timestamp,
				ProtocolVersion: 1,
				Payload: &agentv1.BusPacket_JobProgress{JobProgress: &agentv1.JobProgress{
					JobId:        "job-prog-1",
					StepId:       "step-2",
					Percent:      50,
					Message:      "halfway",
					ResultPtr:    "redis://res:job-prog-1",
					ArtifactPtrs: []string{"redis://art:partial-1"},
					Status:       agentv1.JobStatus_JOB_STATUS_RUNNING,
				}},
			},
		},
		{
			Name: "buspacket_job_cancel.bin",
			Packet: &agentv1.BusPacket{
				TraceId:         "trace-cancel",
				SenderId:        "scheduler-1",
				CreatedAt:       timestamp,
				ProtocolVersion: 1,
				Payload: &agentv1.BusPacket_JobCancel{JobCancel: &agentv1.JobCancel{
					JobId:       "job-cancel-1",
					Reason:      "user_request",
					RequestedBy: "user-7",
				}},
			},
		},
		{
			Name: "buspacket_alert.bin",
			Packet: &agentv1.BusPacket{
				TraceId:         "trace-alert",
				SenderId:        "scheduler-1",
				CreatedAt:       timestamp,
				ProtocolVersion: 1,
				Payload: &agentv1.BusPacket_Alert{Alert: &agentv1.SystemAlert{
					Level:     "WARN",
					Message:   "high queue depth",
					Component: "scheduler",
					Code:      "QUEUE_DEPTH",
				}},
			},
		},
		{
			Name: "buspacket_handshake.bin",
			Packet: &agentv1.BusPacket{
				TraceId:         "trace-handshake",
				SenderId:        "worker-1",
				CreatedAt:       timestamp,
				ProtocolVersion: 1,
				Payload: &agentv1.BusPacket_Handshake{Handshake: &agentv1.Handshake{
					ComponentId:       "worker-1",
					Role:              agentv1.ComponentRole_COMPONENT_ROLE_WORKER,
					SupportedVersions: []int32{1},
					Capabilities: map[string]bool{
						"signatures":   true,
						"progress":     true,
						"cancel":       true,
						"compensation": false,
					},
					SdkVersion:  "2.0.19",
					ReadyTopics: []string{"job.tools", "job.tools.bulk"},
				}},
			},
		},
		{
			Name: "buspacket_alert_enhanced.bin",
			Packet: &agentv1.BusPacket{
				TraceId:         "trace-alert-enhanced",
				SenderId:        "scheduler-1",
				CreatedAt:       timestamp,
				ProtocolVersion: 1,
				Payload: &agentv1.BusPacket_Alert{Alert: &agentv1.SystemAlert{
					Level:           "CRITICAL",
					Message:         "invalid signature from worker",
					Component:       "scheduler",
					Code:            "SIGNATURE_INVALID",
					Severity:        agentv1.AlertSeverity_ALERT_SEVERITY_CRITICAL,
					ErrorCodeEnum:   agentv1.ErrorCode_ERROR_CODE_PROTOCOL_SIGNATURE_INVALID,
					SourceComponent: "scheduler-1",
					Details: map[string]string{
						"sender":  "worker-bad",
						"subject": "sys.job.result",
					},
					TraceId: "trace-offending-packet",
				}},
			},
		},
	}

	for _, fixture := range fixtures {
		outPath := filepath.Join(outDir, fixture.Name)
		if err := writePacket(outPath, fixture.Packet, priv); err != nil {
			fatal(err)
		}
	}

	// Policy Studio (epic-d9a6c0a1) — standalone message fixtures. These are
	// not BusPacket payloads (Rule/Decision/Bundle live in higher-layer APIs,
	// not on the cap bus); the fixtures exercise binary roundtrip parity for
	// the seven new message types + the appended DecisionType enum values
	// across all SDKs.
	policyTimestamp := timestamppb.New(time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC))
	matchPayload, err := structpb.NewStruct(map[string]any{
		"tenants":   []any{"acme"},
		"topics":    []any{"job.acme.*"},
		"scanners":  []any{"secret_leak"},
	})
	if err != nil {
		fatal(fmt.Errorf("build match struct: %w", err))
	}
	decidePayload, err := structpb.NewStruct(map[string]any{
		"decision": "deny",
		"reason":   "secret_leak",
		"severity": "critical",
	})
	if err != nil {
		fatal(fmt.Errorf("build decide struct: %w", err))
	}
	rule := &agentv1.Rule{
		Id:      "rule-input-secrets",
		Name:    "Block secret leaks in input",
		Type:    agentv1.RuleType_RULE_TYPE_INPUT,
		Scope:   &agentv1.RuleScope{Kind: agentv1.RuleScopeKind_RULE_SCOPE_KIND_TENANT, Value: "acme"},
		Status:  agentv1.RuleStatus_RULE_STATUS_PUBLISHED,
		Version: "v1",
		Audit: &agentv1.AuditMetadata{
			CreatedAt: policyTimestamp,
			CreatedBy: "yaron@cordum.io",
		},
		Match:       matchPayload,
		Decide:      decidePayload,
		Description: "Conformance fixture for RuleType=INPUT.",
	}
	traceConstraints, err := structpb.NewStruct(map[string]any{
		"budgets": map[string]any{"max_runtime_ms": 30000},
	})
	if err != nil {
		fatal(fmt.Errorf("build trace constraints: %w", err))
	}
	decision := &agentv1.Decision{
		Source:        agentv1.DecisionSource_DECISION_SOURCE_JOB,
		RuleId:        "rule-input-secrets",
		BundleId:      "bundle-acme-default",
		BundleVersion: "v12",
		Type:          agentv1.DecisionType_DECISION_TYPE_QUARANTINE,
		Trace: []*agentv1.TraceStep{{
			RuleId:       "rule-input-secrets",
			BundleId:     "bundle-acme-default",
			DecisionType: agentv1.DecisionType_DECISION_TYPE_QUARANTINE,
			Reason:       "secret_leak",
			Timestamp:    policyTimestamp,
			Constraints:  traceConstraints,
		}},
		InputRef:  "blob://acme/input/r1",
		OutputRef: "blob://acme/output/r1",
		AuditHash: "0xabcdef",
		Timestamp: policyTimestamp,
	}
	bundle := &agentv1.Bundle{
		Id:           "bundle-edge-prod",
		Name:         "Production edge bundle",
		RuleIds:      []string{"rule-edge-fs-write-deny"},
		ScopeBinding: &agentv1.RuleScope{Kind: agentv1.RuleScopeKind_RULE_SCOPE_KIND_EDGE_FLEET, Value: "fleet-prod"},
		Versions: []*agentv1.BundleVersion{{
			Version:    "v3",
			DeployedAt: policyTimestamp,
			AuditHash:  "0xdeadbeef",
		}},
		Metadata: &agentv1.BundleMetadata{EdgeMode: agentv1.EdgeMode_EDGE_MODE_ENTERPRISE_STRICT},
	}
	rawFixtures := []struct {
		Name string
		Msg  proto.Message
	}{
		{Name: "policy_rule.bin", Msg: rule},
		{Name: "policy_decision.bin", Msg: decision},
		{Name: "policy_bundle.bin", Msg: bundle},
	}
	for _, fx := range rawFixtures {
		data, err := marshalDeterministic(fx.Msg)
		if err != nil {
			fatal(fmt.Errorf("marshal %s: %w", fx.Name, err))
		}
		outPath := filepath.Join(outDir, fx.Name)
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			fatal(fmt.Errorf("write %s: %w", outPath, err))
		}
	}
}

func repoRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fatal(fmt.Errorf("resolve caller path"))
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(filename)))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
