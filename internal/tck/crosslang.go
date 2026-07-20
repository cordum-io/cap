package tck

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StableSDKs are the SDKs classified stable in sdk/support-tiers.json. Every
// one of them must appear on both axes of the fixture matrix; the list is the
// single source of the 3x3 completeness expectation.
var StableSDKs = []string{"go", "node", "python"}

// MatrixEnvVar opts a run into building real SDK artifacts. It is deliberately
// explicit: the build takes tens of seconds and needs the Go, Python, and Node
// toolchains, so `go test ./...` on a Go-only machine must not silently fail.
// TestCrossLanguageMatrixIsEnforcedInCI keeps the gate from quietly vanishing.
const MatrixEnvVar = "CAP_TCK_MATRIX"

// signatureLifetime keeps every fixture inside the CAP-PRODUCTION validity
// window. Artifacts are built before any packet is signed, so only produce and
// consume have to finish inside it.
const signatureLifetime = 4 * time.Minute

// NegativeResult records how one consumer answered a deliberately invalid job.
type NegativeResult struct {
	ID       string
	Consumer string
	OK       bool
	Error    string
}

type keyPair struct {
	privatePEM   string
	publicDERB64 string
}

type produceRequest struct {
	SDK           string          `json:"sdk"`
	Corpus        json.RawMessage `json:"corpus"`
	KeyID         string          `json:"keyId"`
	PrivateKeyPEM string          `json:"privateKeyPem"`
	CreatedAtUnix int64           `json:"createdAtUnix"`
	ExpiresAtUnix int64           `json:"expiresAtUnix"`
}

type produceResponse struct {
	SDK      string `json:"sdk"`
	Fixtures []struct {
		Case             string `json:"case"`
		Wire             string `json:"wire"`
		NormalizedDigest string `json:"normalizedDigest"`
		PreimageDigest   string `json:"preimageDigest"`
		KeyID            string `json:"keyId"`
	} `json:"fixtures"`
}

type consumeJob struct {
	ID           string `json:"id"`
	Wire         string `json:"wire"`
	KeyID        string `json:"keyId"`
	PublicKeyDER string `json:"publicKeyDer"`
}

type consumeRequest struct {
	SDK      string       `json:"sdk"`
	Audience string       `json:"audience"`
	TenantID string       `json:"tenantId"`
	SenderID string       `json:"senderId"`
	Jobs     []consumeJob `json:"jobs"`
}

type consumeResponse struct {
	SDK     string `json:"sdk"`
	Results []struct {
		ID               string `json:"id"`
		OK               bool   `json:"ok"`
		NormalizedDigest string `json:"normalizedDigest"`
		PreimageDigest   string `json:"preimageDigest"`
		Error            string `json:"error"`
	} `json:"results"`
}

type corpusFile struct {
	Audience string `json:"audience"`
	TenantID string `json:"tenantId"`
	SenderID string `json:"senderId"`
	Cases    []struct {
		Name string `json:"name"`
	} `json:"cases"`
}

func newKeyPair() (keyPair, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return keyPair{}, err
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return keyPair{}, err
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return keyPair{}, err
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})
	return keyPair{
		privatePEM:   string(encoded),
		publicDERB64: base64.StdEncoding.EncodeToString(public),
	}, nil
}

// repoRoot resolves the repository root from the internal/tck test directory.
func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("repo root %q has no go.mod: %w", root, err)
	}
	return root, nil
}

func loadCorpus(root string) (corpusFile, []byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, "spec", "tck", "matrix-corpus.json"))
	if err != nil {
		return corpusFile{}, nil, err
	}
	var parsed corpusFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return corpusFile{}, nil, fmt.Errorf("parse matrix corpus: %w", err)
	}
	if len(parsed.Cases) == 0 {
		return corpusFile{}, nil, fmt.Errorf("matrix corpus declares no cases")
	}
	return parsed, raw, nil
}
