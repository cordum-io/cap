package capsdk

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

func TestHandshakeFixturesContainNoPrivateKeyMaterial(t *testing.T) {
	root := filepath.Join("..", "..", "spec", "conformance", "fixtures", "handshake")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, readErr := os.ReadFile(filepath.Join(root, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if bytes.Contains(bytes.ToUpper(data), []byte("PRIVATE KEY")) {
			t.Fatalf("fixture %q contains private key material", entry.Name())
		}
	}
}

type handshakeVectorManifest struct {
	SchemaVersion   int                          `json:"schema_version"`
	ProtocolVersion int                          `json:"protocol_version"`
	Audience        string                       `json:"audience"`
	State           map[string]any               `json:"state"`
	Keys            map[string]handshakeKey      `json:"keys"`
	Positive        map[string]handshakePositive `json:"positive"`
	NegativeVectors []handshakeNegative          `json:"negative_vectors"`
}

type handshakeKey struct {
	ID        string `json:"id"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"public_key"`
}

type handshakePositive struct {
	Packet       string `json:"packet"`
	Domain       string `json:"domain"`
	Signer       string `json:"signer"`
	DigestSHA256 string `json:"digest_sha256"`
}

type handshakeNegative struct {
	ID       string         `json:"id"`
	Base     string         `json:"base"`
	Mutation map[string]any `json:"mutation"`
	Expected map[string]any `json:"expected"`
}

func TestWorkerHandshakeVectorManifestIsComplete(t *testing.T) {
	manifest, root := readHandshakeVectorManifest(t)
	if manifest.SchemaVersion != 1 || manifest.ProtocolVersion != 1 {
		t.Fatalf("manifest versions = (%d, %d), want (1, 1)", manifest.SchemaVersion, manifest.ProtocolVersion)
	}
	if manifest.Audience != "cordum-scheduler" {
		t.Fatalf("manifest audience = %q", manifest.Audience)
	}
	assertHandshakeState(t, manifest.State)
	assertHandshakeKeys(t, root, manifest.Keys)
	assertPositiveVectors(t, root, manifest.Positive)
	assertNegativeVectorCoverage(t, manifest.NegativeVectors)
}

func assertHandshakeState(t *testing.T, state map[string]any) {
	t.Helper()
	now, nowOK := state["now_unix_nanos"].(float64)
	consumed, consumedOK := state["challenge_consumed"].(bool)
	session, sessionOK := state["active_session"].(map[string]any)
	if !nowOK || now <= 0 || !consumedOK || consumed || !sessionOK {
		t.Fatalf("invalid initial state: %+v", state)
	}
	for _, field := range []string{"worker_id", "agent_id", "tenant_id", "audience", "proof_key_id"} {
		if value, ok := session[field].(string); !ok || value == "" {
			t.Errorf("active_session.%s is missing", field)
		}
	}
}

func readHandshakeVectorManifest(t *testing.T) (handshakeVectorManifest, string) {
	t.Helper()
	root := filepath.Join("..", "..", "spec", "conformance", "fixtures", "handshake")
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatalf("read handshake vector manifest: %v", err)
	}
	var manifest handshakeVectorManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode handshake vector manifest: %v", err)
	}
	return manifest, root
}

func assertHandshakeKeys(t *testing.T, root string, keys map[string]handshakeKey) {
	t.Helper()
	for _, signer := range []string{"worker", "scheduler"} {
		key, ok := keys[signer]
		if !ok || key.ID == "" || key.Algorithm != "ECDSA_P256_SHA256" {
			t.Errorf("invalid %s key metadata: %+v", signer, key)
			continue
		}
		if data, err := os.ReadFile(filepath.Join(root, key.PublicKey)); err != nil || len(data) == 0 {
			t.Errorf("read %s public key: bytes=%d err=%v", signer, len(data), err)
		}
	}
}

func assertPositiveVectors(t *testing.T, root string, vectors map[string]handshakePositive) {
	t.Helper()
	hexDigest := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, phase := range []string{"challenge_request", "challenge", "authenticate", "result"} {
		vector, ok := vectors[phase]
		if !ok || vector.Domain == "" || !hexDigest.MatchString(vector.DigestSHA256) {
			t.Errorf("invalid positive %s metadata: %+v", phase, vector)
			continue
		}
		if data, err := os.ReadFile(filepath.Join(root, vector.Packet)); err != nil || len(data) == 0 {
			t.Errorf("read %s packet: bytes=%d err=%v", phase, len(data), err)
		}
	}
}

func assertNegativeVectorCoverage(t *testing.T, vectors []handshakeNegative) {
	t.Helper()
	want := requiredHandshakeNegativeIDs()
	got := make([]string, 0, len(vectors))
	for _, vector := range vectors {
		got = append(got, vector.ID)
		if vector.Base == "" || len(vector.Mutation) == 0 || len(vector.Expected) == 0 {
			t.Errorf("incomplete negative vector %q", vector.ID)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if !equalStrings(got, want) {
		t.Fatalf("negative vector IDs = %v, want %v", got, want)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func requiredHandshakeNegativeIDs() []string {
	return []string{
		"agent_tampered", "audience_tampered", "audience_wrong", "capabilities_tampered",
		"challenge_expired", "client_nonce_tampered", "impersonation_worker_id",
		"missing_component_id", "missing_key_id", "missing_sender_id", "missing_signature",
		"missing_trace_id", "missing_worker_id", "protocol_version_tampered", "renew_a_to_b",
		"renew_superseded_token", "replay_concurrent", "replay_exact",
		"response_challenge_id_mismatch", "response_request_id_mismatch", "response_trace_mismatch",
		"server_nonce_tampered", "skew_negative_boundary", "skew_negative_over_1ns",
		"skew_positive_boundary", "skew_positive_over_1ns", "state_store_unavailable",
		"tenant_tampered", "token_agent_mismatch", "token_audience_mismatch",
		"token_proof_key_mismatch", "token_tenant_mismatch", "token_worker_mismatch",
		"trace_tampered", "unknown_envelope_field", "unknown_payload_field",
		"unknown_scheduler_key", "unsupported_version",
	}
}
