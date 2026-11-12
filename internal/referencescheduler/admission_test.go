// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	adKeyID  = "k1"
	adClient = "c1"
	adWorker = "worker" // the issuer fixture's worker (trust_test.go), tenant "tenant"
)

// adFixture reuses the issuer fixture of trust_test.go, which runs on its own
// clock. The admission clock is separate and anchored at the wall clock,
// because SignProductionPacket checks expiry against the wall clock.
type adFixture struct {
	tr                   *trFixture
	cfg                  *ResolvedConfig
	clientKey, workerKey *ecdsa.PrivateKey
	token                string // live session token of adWorker
	now                  time.Time
	replay               *adCountingReplay
	a                    *Admitter
}

type adReplayCall struct {
	tenant, audience, sender string
	messageID, digest        []byte
	expires                  time.Time
}

// adCountingReplay counts Admit calls. With stub set it returns (outcome, err)
// unchanged; otherwise it delegates to a real in-memory store.
type adCountingReplay struct {
	calls   int
	last    adReplayCall
	store   capsdk.ReplayStore
	stub    bool
	outcome capsdk.ReplayOutcome
	err     error
}

func (r *adCountingReplay) Admit(tenant, audience, sender string, messageID, digest []byte, expires time.Time) (capsdk.ReplayOutcome, error) {
	r.calls++
	r.last = adReplayCall{tenant, audience, sender, bytes.Clone(messageID), bytes.Clone(digest), expires}
	if r.stub {
		return r.outcome, r.err
	}
	return r.store.Admit(tenant, audience, sender, messageID, digest, expires)
}

func adNew(t *testing.T) *adFixture {
	t.Helper()
	f := &adFixture{tr: trNew(t), clientKey: trKey(t), workerKey: trKey(t), now: time.Now()}
	f.token = trIssueSession(t, f.tr).Token
	// The configured worker tenant differs from the session tenant on purpose:
	// admission must take a worker's tenant from its issuer session.
	f.cfg = &ResolvedConfig{MaxPacketBytes: 64 << 10,
		Clients: map[string]ResolvedClient{adClient: {SenderID: adClient, TenantID: "tenant",
			SigningKeys: map[string]*ecdsa.PublicKey{adKeyID: &f.clientKey.PublicKey}}},
		Workers: map[string]ResolvedWorker{adWorker: {WorkerID: adWorker, TenantID: "config-tenant", AgentID: "agent",
			ProofKeyID: "proof", SigningKeys: map[string]*ecdsa.PublicKey{adKeyID: &f.workerKey.PublicKey}}}}
	f.replay = &adCountingReplay{store: capsdk.NewInMemoryReplayStore()}
	adRebuild(t, f)
	return f
}

func adRebuild(t *testing.T, f *adFixture) {
	t.Helper()
	var err error
	if f.a, err = NewAdmitter(f.cfg, f.tr.i, f.replay, func() time.Time { return f.now }); err != nil {
		t.Fatal(err)
	}
}

// adPacket mirrors sdk/go/production_profile_red_test.go: an unsigned
// production packet with an identity binding and a payload for audience.
func adPacket(t *testing.T, sender, tenant, token, audience, job string) *agentv1.BusPacket {
	t.Helper()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	p := &agentv1.BusPacket{TraceId: "trace-admission", SenderId: sender, ProtocolVersion: capsdk.DefaultProtocolVersion,
		AuthToken: token, Identity: &agentv1.IdentityBinding{TenantId: tenant},
		SignatureMetadata: &agentv1.SignatureMetadata{ProfileVersion: capsdk.ProductionProfileVersion,
			Algorithm: capsdk.ProductionAlgorithm, MessageId: id, Audience: audience,
			ExpiresAt: timestamppb.New(time.Now().Add(2 * time.Minute)), KeyId: adKeyID}}
	switch audience {
	case SubjectResult:
		p.Payload = &agentv1.BusPacket_JobResult{JobResult: &agentv1.JobResult{JobId: job,
			Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED, WorkerId: sender}}
	case SubjectProgress:
		p.Payload = &agentv1.BusPacket_JobProgress{JobProgress: &agentv1.JobProgress{JobId: job, Percent: 50}}
	case SubjectClientCancel:
		p.Payload = &agentv1.BusPacket_JobCancel{JobCancel: &agentv1.JobCancel{JobId: job}}
	default:
		p.Payload = &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{JobId: job, Topic: "topic"}}
	}
	return p
}

func adSign(t *testing.T, key *ecdsa.PrivateKey, p *agentv1.BusPacket) []byte {
	t.Helper()
	raw, err := capsdk.SignProductionPacket(p, key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func adClientRaw(t *testing.T, f *adFixture, audience string) []byte {
	t.Helper()
	return adSign(t, f.clientKey, adPacket(t, adClient, "tenant", "", audience, "job"))
}

func adWorkerRaw(t *testing.T, f *adFixture, token string) []byte {
	t.Helper()
	return adSign(t, f.workerKey, adPacket(t, adWorker, "tenant", token, SubjectResult, "job"))
}

type adCase struct {
	name string
	want error
	run  func(t *testing.T, f *adFixture) (subject string, raw []byte)
}

func adRejections() []adCase {
	return append(append(adSignatureCases(), adSenderCases()...), adShapeCases()...)
}

func adSignatureCases() []adCase {
	return []adCase{
		{"unsigned", ErrAdmitSignature, func(t *testing.T, f *adFixture) (string, []byte) {
			raw, err := proto.Marshal(adPacket(t, adClient, "tenant", "", SubjectSubmit, "job"))
			if err != nil {
				t.Fatal(err)
			}
			return SubjectSubmit, raw
		}},
		{"expired", ErrAdmitSignature, func(t *testing.T, f *adFixture) (string, []byte) {
			p := adPacket(t, adClient, "tenant", "", SubjectSubmit, "job")
			raw := adSign(t, f.clientKey, p)
			f.now = p.GetSignatureMetadata().GetExpiresAt().AsTime()
			return SubjectSubmit, raw
		}},
		{"wrong key", ErrAdmitSignature, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectSubmit, adSign(t, trKey(t), adPacket(t, adClient, "tenant", "", SubjectSubmit, "job"))
		}},
		{"wrong audience", ErrAdmitSignature, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectSubmit, adClientRaw(t, f, SubjectResult)
		}},
		{"tenant mismatch", ErrAdmitSignature, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectResult, adSign(t, f.workerKey, adPacket(t, adWorker, "t2", f.token, SubjectResult, "job"))
		}},
		{"client tenant mismatch", ErrAdmitSignature, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectSubmit, adSign(t, f.clientKey, adPacket(t, adClient, "t2", "", SubjectSubmit, "job"))
		}},
	}
}

func adSenderCases() []adCase {
	return []adCase{
		{"unknown client", ErrAdmitSender, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectSubmit, adSign(t, f.clientKey, adPacket(t, "c9", "tenant", "", SubjectSubmit, "job"))
		}},
		{"worker on a client subject", ErrAdmitSender, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectSubmit, adSign(t, f.workerKey, adPacket(t, adWorker, "tenant", f.token, SubjectSubmit, "job"))
		}},
		{"client on a worker subject", ErrAdmitSender, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectResult, adClientRaw(t, f, SubjectResult)
		}},
		{"unconfigured worker with a live session", ErrAdmitSender, func(t *testing.T, f *adFixture) (string, []byte) {
			delete(f.cfg.Workers, adWorker)
			adRebuild(t, f)
			return SubjectResult, adWorkerRaw(t, f, f.token)
		}},
		{"worker without a session", ErrAdmitSender, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectResult, adWorkerRaw(t, f, "never-issued-token")
		}},
		{"stale token", ErrAdmitSender, func(t *testing.T, f *adFixture) (string, []byte) {
			f.tr.now = f.tr.now.Add(time.Second)
			auth, v := trAuth(t, f.tr, trRenew, f.token)
			if trFinish(t, f.tr, auth, v).Token == f.token {
				t.Fatal("renewal did not rotate the token")
			}
			return SubjectResult, adWorkerRaw(t, f, f.token)
		}},
	}
}

func adShapeCases() []adCase {
	return []adCase{
		{"oversized", ErrAdmitTooLarge, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectSubmit, bytes.Repeat([]byte{0xff}, f.cfg.MaxPacketBytes+1)
		}},
		{"malformed", ErrAdmitMalformed, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectSubmit, []byte{0x0a, 0x05, 'a'}
		}},
		{"unknown subject", ErrAdmitSubject, func(t *testing.T, f *adFixture) (string, []byte) {
			return "sys.job.dlq", adClientRaw(t, f, SubjectSubmit)
		}},
		{"scheduler cancel subject", ErrAdmitSubject, func(t *testing.T, f *adFixture) (string, []byte) {
			return SubjectCancel, adClientRaw(t, f, SubjectCancel)
		}},
	}
}

// A1 (DoD 1): each rejection class gets its own bare sentinel and no proof.
func TestAdmit_Rejections(t *testing.T) {
	for _, tc := range adRejections() {
		t.Run(tc.name, func(t *testing.T) {
			f := adNew(t)
			subject, raw := tc.run(t, f)
			v, err := f.a.Admit(subject, raw)
			if err != tc.want || v.IsVerified() {
				t.Fatalf("Admit = (verified %v, %v), want (unverified, %v)", v.IsVerified(), err, tc.want)
			}
		})
	}
}

// adAdmitted asserts that raw is admitted with the given identity and that
// exactly one replay admission ran, scoped to that identity and subject.
func adAdmitted(t *testing.T, f *adFixture, subject string, raw []byte, sender, tenant string) capsdk.VerifiedProductionPacket {
	t.Helper()
	calls := f.replay.calls
	v, err := f.a.Admit(subject, raw)
	if err != nil || !v.IsVerified() {
		t.Fatalf("Admit(%s) = (verified %v, %v), want admitted", subject, v.IsVerified(), err)
	}
	meta := v.Packet().GetSignatureMetadata()
	if v.Subject() != subject || v.Tenant() != tenant || v.Sender() != sender || !bytes.Equal(v.MessageID(), meta.GetMessageId()) {
		t.Fatalf("admitted identity = (%q, %q, %q), want (%q, %q, %q)", v.Subject(), v.Tenant(), v.Sender(), subject, tenant, sender)
	}
	digest, err := capsdk.ProductionSignedBodyDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := f.replay.last
	if f.replay.calls != calls+1 || got.tenant != tenant || got.audience != subject || got.sender != sender ||
		!bytes.Equal(got.messageID, meta.GetMessageId()) || !bytes.Equal(got.digest, digest[:]) || !got.expires.Equal(meta.GetExpiresAt().AsTime()) {
		t.Fatalf("replay admission = %d calls, last %+v; want one call scoped to the verified packet", f.replay.calls-calls, got)
	}
	return v
}

func adRefused(t *testing.T, f *adFixture, subject string, raw []byte, want error) {
	t.Helper()
	if v, err := f.a.Admit(subject, raw); err != want || v.IsVerified() {
		t.Fatalf("Admit = (verified %v, %v), want (unverified, %v)", v.IsVerified(), err, want)
	}
}

// A2 (DoD 2): valid packets are admitted with the configured or session
// identity; duplicates and conflicts are classified.
func TestAdmit_AdmitsAndDeduplicates(t *testing.T) {
	t.Run("client subjects", func(t *testing.T) {
		f := adNew(t)
		adAdmitted(t, f, SubjectSubmit, adClientRaw(t, f, SubjectSubmit), adClient, "tenant")
		adAdmitted(t, f, SubjectClientCancel, adClientRaw(t, f, SubjectClientCancel), adClient, "tenant")
	})
	t.Run("worker subjects take the session tenant", func(t *testing.T) {
		f := adNew(t)
		v := adAdmitted(t, f, SubjectResult, adWorkerRaw(t, f, f.token), adWorker, "tenant")
		if v.SessionToken() != f.token {
			t.Fatal("verified session token differs from the live token")
		}
		progress := adSign(t, f.workerKey, adPacket(t, adWorker, "tenant", f.token, SubjectProgress, "job"))
		adAdmitted(t, f, SubjectProgress, progress, adWorker, "tenant")
	})
	t.Run("exact and re-signed redelivery are duplicates", func(t *testing.T) {
		f := adNew(t)
		p := adPacket(t, adClient, "tenant", "", SubjectSubmit, "job")
		raw := adSign(t, f.clientKey, p)
		adAdmitted(t, f, SubjectSubmit, raw, adClient, "tenant")
		adRefused(t, f, SubjectSubmit, raw, ErrAdmitDuplicate)
		adRefused(t, f, SubjectSubmit, adSign(t, f.clientKey, p), ErrAdmitDuplicate)
		if f.replay.calls != 3 {
			t.Fatalf("replay calls = %d, want 3", f.replay.calls)
		}
	})
	t.Run("same message id with another body is a replay", func(t *testing.T) {
		f := adNew(t)
		p := adPacket(t, adClient, "tenant", "", SubjectSubmit, "job")
		adAdmitted(t, f, SubjectSubmit, adSign(t, f.clientKey, p), adClient, "tenant")
		p.GetJobRequest().JobId = "job-2"
		adRefused(t, f, SubjectSubmit, adSign(t, f.clientKey, p), ErrAdmitReplay)
	})
	for _, stub := range []struct {
		name    string
		outcome capsdk.ReplayOutcome
		err     error
	}{
		{"failing store", 0, capsdk.ErrReplayStoreUnavailable},
		{"first with an error", capsdk.ReplayOutcomeFirst, capsdk.ErrReplayStoreUnavailable},
		{"duplicate with an error", capsdk.ReplayOutcomeDuplicate, capsdk.ErrReplayConflict},
		{"unknown outcome", 0, nil},
	} {
		t.Run("replay store "+stub.name+" fails closed", func(t *testing.T) {
			f := adNew(t)
			f.replay.stub, f.replay.outcome, f.replay.err = true, stub.outcome, stub.err
			adRefused(t, f, SubjectSubmit, adClientRaw(t, f, SubjectSubmit), ErrAdmitReplay)
			if f.replay.calls != 1 {
				t.Fatalf("replay calls = %d, want 1", f.replay.calls)
			}
		})
	}
}

// A3 (DoD 3): no rejection before the replay stage reaches the replay store,
// the subject is checked first and the size bound runs before decoding.
func TestAdmit_OrderOfChecks(t *testing.T) {
	for _, tc := range adRejections() {
		t.Run(tc.name, func(t *testing.T) {
			f := adNew(t)
			subject, raw := tc.run(t, f)
			if _, err := f.a.Admit(subject, raw); err != tc.want || f.replay.calls != 0 {
				t.Fatalf("Admit = %v after %d replay calls, want %v after 0", err, f.replay.calls, tc.want)
			}
		})
	}
	t.Run("oversized valid packet", func(t *testing.T) {
		f := adNew(t)
		raw := adClientRaw(t, f, SubjectSubmit)
		adRefused(t, f, "sys.job.dlq", bytes.Repeat([]byte{0xff}, f.cfg.MaxPacketBytes+1), ErrAdmitSubject)
		f.cfg.MaxPacketBytes = len(raw) - 1
		adRebuild(t, f)
		adRefused(t, f, SubjectSubmit, raw, ErrAdmitTooLarge)
		if f.replay.calls != 0 {
			t.Fatalf("replay calls = %d, want 0", f.replay.calls)
		}
		f.cfg.MaxPacketBytes = len(raw)
		adRebuild(t, f)
		adAdmitted(t, f, SubjectSubmit, raw, adClient, "tenant")
	})
}

// A4: every nil input is refused, and later changes to cfg do not reach an
// existing Admitter.
func TestNewAdmitter_RejectsNil(t *testing.T) {
	f := adNew(t)
	now := func() time.Time { return f.now }
	for name, build := range map[string]func() (*Admitter, error){
		"config": func() (*Admitter, error) { return NewAdmitter(nil, f.tr.i, f.replay, now) },
		"issuer": func() (*Admitter, error) { return NewAdmitter(f.cfg, nil, f.replay, now) },
		"replay": func() (*Admitter, error) { return NewAdmitter(f.cfg, f.tr.i, nil, now) },
		"clock":  func() (*Admitter, error) { return NewAdmitter(f.cfg, f.tr.i, f.replay, nil) },
	} {
		if a, err := build(); a != nil || err != ErrInvalidConfig {
			t.Fatalf("NewAdmitter with nil %s = (%v, %v), want (nil, ErrInvalidConfig)", name, a, err)
		}
	}
}

func TestNewAdmitter_SnapshotsIdentities(t *testing.T) {
	f := adNew(t)
	client, worker := adClientRaw(t, f, SubjectSubmit), adWorkerRaw(t, f, f.token)
	f.cfg.Clients[adClient].SigningKeys[adKeyID] = &trKey(t).PublicKey
	f.cfg.Workers[adWorker].SigningKeys[adKeyID] = &trKey(t).PublicKey
	delete(f.cfg.Clients, adClient)
	delete(f.cfg.Workers, adWorker)
	f.cfg.MaxPacketBytes = 1
	adAdmitted(t, f, SubjectSubmit, client, adClient, "tenant")
	adAdmitted(t, f, SubjectResult, worker, adWorker, "tenant")
}
