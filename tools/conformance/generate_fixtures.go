package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type detReader struct {
	seed    [32]byte
	counter uint64
}

func (d *detReader) Read(p []byte) (int, error) {
	buf := make([]byte, 0, len(p))
	for len(buf) < len(p) {
		block := sha256.New()
		block.Write(d.seed[:])
		var ctrBytes [8]byte
		binary.LittleEndian.PutUint64(ctrBytes[:], d.counter)
		block.Write(ctrBytes[:])
		sum := block.Sum(nil)
		need := len(p) - len(buf)
		if need > len(sum) {
			need = len(sum)
		}
		buf = append(buf, sum[:need]...)
		d.counter++
	}
	copy(p, buf[:len(p)])
	return len(p), nil
}

func seededReader(name string) *detReader {
	sum := sha256.Sum256([]byte(name))
	return &detReader{seed: sum}
}

func testPrivateKey() *ecdsa.PrivateKey {
	curve := elliptic.P256()
	d := big.NewInt(1)
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}, D: d}
}

func marshalDeterministic(msg proto.Message) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(msg)
}

func signPacket(pkt *agentv1.BusPacket, priv *ecdsa.PrivateKey, reader *detReader) error {
	clone := proto.Clone(pkt).(*agentv1.BusPacket)
	clone.Signature = nil
	unsigned, err := marshalDeterministic(clone)
	if err != nil {
		return fmt.Errorf("marshal unsigned: %w", err)
	}
	hash := sha256.Sum256(unsigned)
	sig, err := ecdsa.SignASN1(reader, priv, hash[:])
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	pkt.Signature = sig
	return nil
}

func writePacket(path string, pkt *agentv1.BusPacket, priv *ecdsa.PrivateKey, reader *detReader) error {
	if err := signPacket(pkt, priv, reader); err != nil {
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
	}

	for _, fixture := range fixtures {
		outPath := filepath.Join(outDir, fixture.Name)
		reader := seededReader(fixture.Name)
		if err := writePacket(outPath, fixture.Packet, priv, reader); err != nil {
			fatal(err)
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
