package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

const expectedHandshakeNegativeVectors = 38

type mutationManifest struct {
	Keys            map[string]manifestKey     `json:"keys"`
	Positive        map[string]positiveFixture `json:"positive"`
	NegativeVectors []negativeVector           `json:"negative_vectors"`
	State           manifestState              `json:"state"`
}

type positiveFixture struct {
	Packet string `json:"packet"`
	Signer string `json:"signer"`
}

type manifestKey struct {
	ID        string `json:"id"`
	PublicKey string `json:"public_key"`
}

type negativeVector struct {
	ID       string           `json:"id"`
	Base     string           `json:"base"`
	Mutation manifestMutation `json:"mutation"`
	Expected manifestExpected `json:"expected"`
}

type manifestExpected struct {
	Accepted       *bool  `json:"accepted"`
	Reason         string `json:"reason"`
	InstallCount   *int   `json:"install_count"`
	MintCount      *int   `json:"mint_count"`
	AcceptedCount  *int   `json:"accepted_count"`
	RejectedCount  *int   `json:"rejected_count"`
	ChallengeCount *int   `json:"challenge_count"`
}

type manifestMutation struct {
	Kind  string          `json:"kind"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

type manifestState struct {
	NowUnixNanos int64 `json:"now_unix_nanos"`
}

func TestManifestVectorsUseSDKValidationOrDeclareServerAuthority(t *testing.T) {
	directory := filepath.Join("..", "..", "..", "spec", "conformance", "fixtures", "handshake")
	manifest := loadMutationManifest(t, filepath.Join(directory, "manifest.json"))
	if len(manifest.NegativeVectors) != expectedHandshakeNegativeVectors {
		t.Fatalf("negative vector count=%d want=%d", len(manifest.NegativeVectors), expectedHandshakeNegativeVectors)
	}
	keys := loadManifestPublicKeys(t, directory, manifest)
	assertPositiveFixturesValidate(t, directory, manifest, keys)
	assertVectorContractsComplete(t, manifest)
	seen := make(map[string]struct{}, len(manifest.NegativeVectors))
	for _, vector := range manifest.NegativeVectors {
		vector := vector
		t.Run(vector.ID, func(t *testing.T) {
			if _, exists := seen[vector.ID]; exists {
				t.Fatalf("duplicate vector ID %q", vector.ID)
			}
			seen[vector.ID] = struct{}{}
			assertVectorEvidence(t, directory, manifest, vector, keys)
		})
	}
}

func loadMutationManifest(t *testing.T, path string) mutationManifest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mutation manifest: %v", err)
	}
	var manifest mutationManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatalf("decode mutation manifest: %v", err)
	}
	return manifest
}

func loadBasePacket(t *testing.T, directory string, manifest mutationManifest, base string) *agentv1.BusPacket {
	t.Helper()
	fixture, ok := manifest.Positive[base]
	if !ok || fixture.Packet == "" {
		t.Fatalf("base %q has no positive fixture", base)
	}
	data, err := os.ReadFile(filepath.Join(directory, filepath.Base(fixture.Packet)))
	if err != nil {
		t.Fatalf("read base fixture: %v", err)
	}
	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(data, packet); err != nil {
		t.Fatalf("decode base fixture: %v", err)
	}
	return packet
}

func (m manifestMutation) stringValue() (string, error) {
	var value string
	if err := json.Unmarshal(m.Value, &value); err != nil {
		return "", fmt.Errorf("decode string mutation value: %w", err)
	}
	return value, nil
}

func (m manifestMutation) intValue() (int64, error) {
	var value int64
	if err := json.Unmarshal(m.Value, &value); err != nil {
		return 0, fmt.Errorf("decode integer mutation value: %w", err)
	}
	return value, nil
}

func (m manifestMutation) boolValue() (bool, error) {
	var value bool
	if err := json.Unmarshal(m.Value, &value); err != nil {
		return false, fmt.Errorf("decode boolean mutation value: %w", err)
	}
	return value, nil
}

func TestServerRequiredMutationsAreNotLocallySimulated(t *testing.T) {
	kinds := []string{"repeat", "concurrent_repeat", "token_claim", "renew_token_subject", "renew_token_state", "state_store"}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			err := applyManifestPacketMutation(&agentv1.BusPacket{}, manifestMutation{Kind: kind}, 0)
			if err == nil {
				t.Fatalf("server-required mutation %q was accepted as local evidence", kind)
			}
		})
	}
}
