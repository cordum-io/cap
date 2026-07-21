package tck

import (
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

// decodeFixture verifies and decodes a producer's fixture wire using the SDK's
// public production-trust API and the producer's own key, returning the decoded
// packet so a test can inspect the structured surface that actually crossed the
// wire (rather than trusting a self-reported flag).
func (e *CrossLangEnv) decodeFixture(f Fixture) (*agentv1.BusPacket, error) {
	parsed, err := x509.ParsePKIXPublicKey(f.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	pub, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ECDSA")
	}
	trust := capsdk.ProductionTrustStore{
		Audience:   e.corpus.Audience,
		Tenant:     e.corpus.TenantID,
		Sender:     e.corpus.SenderID,
		PublicKeys: map[string]*ecdsa.PublicKey{f.KeyID: pub},
	}
	return capsdk.VerifyProductionPacket(f.Wire, trust)
}

// validResourceRef reports whether ref carries the minimum CAP-PRODUCTION
// content-resolution surface: a non-empty resolver id and a 32-byte sha256.
// Every decoded ResourceRef - context_ref, result_ref, and each artifact_ref -
// is held to this same bar so a producer cannot satisfy the matrix by emitting
// empty or resolver-id-less refs padded out to the right count.
func validResourceRef(ref *agentv1.ResourceRef) bool {
	return ref != nil && ref.GetResolverId() != "" && len(ref.GetSha256()) == 32
}

// TestCrossLanguageResourceSurface proves the structured ResourceRef surface
// (context_ref / result_ref / artifact_refs) actually round-trips: for every
// producer, the fixture bytes emitted by its installed artifact are decoded and
// the resource fields are asserted present with a resolver id and a 32-byte
// sha256. A driver that silently dropped resources - or a corpus that carried
// none - fails here, not just in an aggregate digest count.
func TestCrossLanguageResourceSurface(t *testing.T) {
	env := NewCrossLangEnv(t)
	fixtures, _ := env.Run(t)

	byKey := map[string]Fixture{}
	for _, f := range fixtures {
		byKey[f.Producer+"/"+f.Case] = f
	}

	for _, sdk := range StableSDKs {
		reqFx, ok := byKey[sdk+"/job-request"]
		if !ok {
			t.Fatalf("no job-request fixture from producer %s", sdk)
		}
		reqPkt, err := env.decodeFixture(reqFx)
		if err != nil {
			t.Fatalf("%s job-request decode: %v", sdk, err)
		}
		ctxRef := reqPkt.GetJobRequest().GetContextRef()
		if !validResourceRef(ctxRef) {
			t.Errorf("%s job-request context_ref surface missing or malformed: %+v", sdk, ctxRef)
		}

		resFx, ok := byKey[sdk+"/job-result-dispatch"]
		if !ok {
			t.Fatalf("no job-result-dispatch fixture from producer %s", sdk)
		}
		resPkt, err := env.decodeFixture(resFx)
		if err != nil {
			t.Fatalf("%s job-result decode: %v", sdk, err)
		}
		result := resPkt.GetJobResult()
		if rr := result.GetResultRef(); !validResourceRef(rr) {
			t.Errorf("%s job-result result_ref surface missing or malformed: %+v", sdk, rr)
		}
		arts := result.GetArtifactRefs()
		if len(arts) < 2 {
			t.Errorf("%s job-result carries %d artifact_refs, want >=2", sdk, len(arts))
		}
		for i, art := range arts {
			if !validResourceRef(art) {
				t.Errorf("%s job-result artifact_refs[%d] surface missing or malformed: %+v", sdk, i, art)
			}
		}
	}
}

// TestHelperSpewer is re-executed as a child to stress bounded output capture.
// It is inert unless TCK_SPEW selects a stream; then it writes TCK_SPEW_BYTES to
// that stream and exits 0, so the parent can prove the capture is bounded.
func TestHelperSpewer(t *testing.T) {
	stream := os.Getenv("TCK_SPEW")
	if stream == "" {
		return
	}
	n, _ := strconv.Atoi(os.Getenv("TCK_SPEW_BYTES"))
	w := os.Stdout
	if stream == "stderr" {
		w = os.Stderr
	}
	chunk := make([]byte, 32*1024)
	for i := range chunk {
		chunk[i] = 'x'
	}
	for written := 0; written < n; written += len(chunk) {
		_, _ = w.Write(chunk)
	}
	os.Exit(0)
}

// TestRunBoundedProcessCapsChildOutput proves a huge or noisy child cannot make
// the matrix driver invocation exhaust memory: stdout past the cap fails closed
// (a truncated response body must never parse as valid), and stderr past the cap
// is tolerated but truncated. Small caps keep the test fast.
func TestRunBoundedProcessCapsChildOutput(t *testing.T) {
	const capOut, capErr = 64 * 1024, 16 * 1024
	spawn := func(stream string, bytesToWrite int) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperSpewer$")
		cmd.Env = append(os.Environ(), "TCK_SPEW="+stream, "TCK_SPEW_BYTES="+strconv.Itoa(bytesToWrite))
		return cmd
	}

	out, _, err := runBoundedProcess(spawn("stdout", 8*capOut), capOut, capErr)
	if err == nil {
		t.Error("a child exceeding the stdout cap must fail closed, got nil error")
	}
	if len(out) > capOut {
		t.Errorf("captured stdout %d bytes exceeds cap %d", len(out), capOut)
	}

	_, errText, err := runBoundedProcess(spawn("stderr", 8*capErr), capOut, capErr)
	if err != nil {
		t.Errorf("a noisy-stderr child that exits 0 must still succeed, got %v", err)
	}
	if len(errText) > capErr {
		t.Errorf("captured stderr %d bytes exceeds cap %d", len(errText), capErr)
	}
}
