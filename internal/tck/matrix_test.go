package tck

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func genKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	return k
}

func normalizedDigest(pkt *agentv1.BusPacket) (string, error) {
	clone := proto.Clone(pkt).(*agentv1.BusPacket)
	clone.Signature = nil
	b, err := capsdk.MarshalDeterministic(clone)
	if err != nil {
		return "", err
	}
	return sha256hex(b), nil
}

// goProduceFixtures builds signed fixtures for several payloads using only the
// Go SDK's public signing API — the reference producer edge of the matrix.
func goProduceFixtures(t *testing.T, key *ecdsa.PrivateKey) []Fixture {
	t.Helper()
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal pubkey: %v", err)
	}
	cases := map[string]*agentv1.BusPacket{
		"job-request": {TraceId: "t1", SenderId: "s1", Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{JobId: "j1"}}},
		"job-result":  {TraceId: "t2", SenderId: "s2", Payload: &agentv1.BusPacket_JobResult{JobResult: &agentv1.JobResult{JobId: "j1", Status: agentv1.JobStatus_JOB_STATUS_SUCCEEDED}}},
		"heartbeat":   {TraceId: "t3", SenderId: "s3", Payload: &agentv1.BusPacket_Heartbeat{Heartbeat: &agentv1.Heartbeat{}}},
	}
	var out []Fixture
	for name, pkt := range cases {
		preimage, err := capsdk.MarshalUnsignedForSignature(pkt)
		if err != nil {
			t.Fatalf("%s preimage: %v", name, err)
		}
		nd, err := normalizedDigest(pkt)
		if err != nil {
			t.Fatalf("%s normalized: %v", name, err)
		}
		if err := capsdk.SignPacket(pkt, key); err != nil {
			t.Fatalf("%s sign: %v", name, err)
		}
		wire, err := capsdk.MarshalDeterministic(pkt)
		if err != nil {
			t.Fatalf("%s wire: %v", name, err)
		}
		out = append(out, Fixture{
			Producer: "go", Case: name, Wire: wire,
			NormalizedDigest: nd, PreimageDigest: sha256hex(preimage),
			KeyID: "k1", PublicKey: pubDER, Signature: pkt.GetSignature(),
		})
	}
	return out
}

// goConsumer decodes and verifies fixtures via the Go SDK public API.
type goConsumer struct{}

func (goConsumer) Name() string { return "go" }

func (goConsumer) Inspect(wire []byte) (string, string, error) {
	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(wire, &pkt); err != nil {
		return "", "", err
	}
	preimage, err := capsdk.MarshalUnsignedForSignature(&pkt)
	if err != nil {
		return "", "", err
	}
	nd, err := normalizedDigest(&pkt)
	if err != nil {
		return "", "", err
	}
	return nd, sha256hex(preimage), nil
}

func (goConsumer) VerifySignature(wire, publicKey, _ []byte) error {
	var pkt agentv1.BusPacket
	if err := proto.Unmarshal(wire, &pkt); err != nil {
		return err
	}
	pub, err := x509.ParsePKIXPublicKey(publicKey)
	if err != nil {
		return err
	}
	ecpub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not ECDSA")
	}
	return capsdk.VerifyPacketSignature(&pkt, ecpub)
}

func TestMatrixGoSelfEdgeVerifies(t *testing.T) {
	fx := goProduceFixtures(t, genKey(t))
	edges := RunMatrix(fx, []Consumer{goConsumer{}})
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if !edges[0].OK() {
		t.Fatalf("go->go edge failed: %v", edges[0].Failures)
	}
	if edges[0].Cases < 3 {
		t.Fatalf("expected >=3 cases, got %d", edges[0].Cases)
	}
}

func TestMatrixRejectsTamperedWire(t *testing.T) {
	fx := goProduceFixtures(t, genKey(t))
	fx[0].Wire[len(fx[0].Wire)/2] ^= 0xFF // flip a byte
	edges := RunMatrix(fx, []Consumer{goConsumer{}})
	if edges[0].OK() {
		t.Fatal("a tampered wire must fail the edge")
	}
}

func TestMatrixRejectsWrongKey(t *testing.T) {
	fx := goProduceFixtures(t, genKey(t))
	wrong := genKey(t)
	wrongDER, _ := x509.MarshalPKIXPublicKey(&wrong.PublicKey)
	for i := range fx {
		fx[i].PublicKey = wrongDER
	}
	edges := RunMatrix(fx, []Consumer{goConsumer{}})
	if edges[0].OK() {
		t.Fatal("verification under the wrong key must fail")
	}
}

func TestMatrixDetectsDigestMismatch(t *testing.T) {
	fx := goProduceFixtures(t, genKey(t))
	fx[0].NormalizedDigest = "deadbeef"
	edges := RunMatrix(fx, []Consumer{goConsumer{}})
	if edges[0].OK() {
		t.Fatal("a declared-digest mismatch must fail the edge")
	}
}

// Completeness is computed, not listed: an engine driven with only the Go
// in-process reference must report the other eight edges missing. The delivered
// three-language matrix asserts the complement of this in
// TestCrossLanguageMatrixCoversAllNineEdges, where MissingEdges must be empty.
func TestMatrixCompletenessIsComputed(t *testing.T) {
	edges := RunMatrix(goProduceFixtures(t, genKey(t)), []Consumer{goConsumer{}})
	missing := MissingEdges(edges, StableSDKs, StableSDKs)
	if len(missing) != len(StableSDKs)*len(StableSDKs)-1 {
		t.Fatalf("expected %d missing edges for a go-only engine run, got %d: %v",
			len(StableSDKs)*len(StableSDKs)-1, len(missing), missing)
	}
	for _, m := range missing {
		if m == "go->go" {
			t.Fatal("go->go should be present, not missing")
		}
	}
}
