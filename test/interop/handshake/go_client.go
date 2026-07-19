package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
)

type clientConfig struct {
	testCase, natsURL string
	trust             *capsdk.WorkerTrustConfig
}

type clientResult struct {
	Language                string `json:"language"`
	Case                    string `json:"case"`
	Status                  string `json:"status"`
	Issue                   bool   `json:"issue"`
	Renew                   bool   `json:"renew"`
	Rotated                 bool   `json:"rotated"`
	MutationSignatureValid  bool   `json:"mutation_signature_valid"`
	TamperSignatureRejected bool   `json:"tamper_signature_rejected"`
}

func main() {
	result, err := runClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "handshake interop client failed:", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "encode result failed")
		os.Exit(1)
	}
}

func runClient() (clientResult, error) {
	config, err := loadClientConfig()
	if err != nil {
		return clientResult{}, err
	}
	connection, err := nats.Connect(config.natsURL, nats.Name("cap-go-handshake-interop"), nats.Timeout(3*time.Second))
	if err != nil {
		return clientResult{}, fmt.Errorf("connect NATS: %w", err)
	}
	defer connection.Close()
	result := clientResult{Language: "go", Case: config.testCase, Status: "PASS"}
	if config.testCase == "valid" {
		issued, exchangeErr := exchange(connection, config.trust, issuePurpose(), "")
		if exchangeErr != nil {
			return clientResult{}, exchangeErr
		}
		renewed, exchangeErr := exchange(connection, config.trust, renewPurpose(), issued.Token)
		if exchangeErr != nil {
			return clientResult{}, exchangeErr
		}
		result.Issue, result.Renew = true, true
		result.Rotated = issued.Token != "" && renewed.Token != "" && issued.Token != renewed.Token
		if !result.Rotated {
			return clientResult{}, errors.New("session did not rotate")
		}
		return result, nil
	}
	proof, err := exerciseNegative(connection, config)
	if err != nil {
		return clientResult{}, err
	}
	result.MutationSignatureValid = proof.signatureValid
	result.TamperSignatureRejected = proof.tamperRejected
	return result, nil
}

func loadClientConfig() (*clientConfig, error) {
	environment, err := loadEnvironment()
	if err != nil {
		return nil, err
	}
	privateKey, err := loadPrivateKey(environment["CAP_HANDSHAKE_WORKER_PRIVATE_KEY"])
	if err != nil {
		return nil, err
	}
	publicKey, err := loadPublicKey(environment["CAP_HANDSHAKE_SCHEDULER_PUBLIC_KEY"])
	if err != nil {
		return nil, err
	}
	keyID := environment["CAP_HANDSHAKE_SCHEDULER_KEY_ID"]
	trust := &capsdk.WorkerTrustConfig{
		WorkerID: environment["CAP_HANDSHAKE_WORKER_ID"], ExpectedAgentID: environment["CAP_HANDSHAKE_AGENT_ID"],
		TenantID: environment["CAP_HANDSHAKE_TENANT_ID"], Audience: capsdk.WorkerHandshakeAudience,
		ProofKeyID: environment["CAP_HANDSHAKE_PROOF_KEY_ID"], ProofPrivateKey: privateKey,
		ExpectedSchedulerID: environment["CAP_HANDSHAKE_SCHEDULER_ID"],
		SchedulerPublicKeys: map[string]*ecdsa.PublicKey{keyID: publicKey},
		SDKVersion:          environment["CAP_HANDSHAKE_SDK_VERSION"],
	}
	if err := capsdk.ValidateWorkerTrustConfig(trust); err != nil {
		return nil, err
	}
	return &clientConfig{
		testCase: environment["CAP_HANDSHAKE_CASE"],
		natsURL:  environment["CAP_HANDSHAKE_NATS_URL"], trust: trust,
	}, nil
}

func loadEnvironment() (map[string]string, error) {
	names := []string{
		"CAP_HANDSHAKE_WORKER_PRIVATE_KEY", "CAP_HANDSHAKE_SCHEDULER_PUBLIC_KEY",
		"CAP_HANDSHAKE_SCHEDULER_KEY_ID", "CAP_HANDSHAKE_WORKER_ID",
		"CAP_HANDSHAKE_AGENT_ID", "CAP_HANDSHAKE_TENANT_ID", "CAP_HANDSHAKE_PROOF_KEY_ID",
		"CAP_HANDSHAKE_SCHEDULER_ID", "CAP_HANDSHAKE_SDK_VERSION",
		"CAP_HANDSHAKE_CASE", "CAP_HANDSHAKE_NATS_URL",
	}
	values := make(map[string]string, len(names))
	for _, name := range names {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			return nil, fmt.Errorf("missing required environment: %s", name)
		}
		values[name] = value
	}
	return values, nil
}

func exchange(connection *nats.Conn, trust *capsdk.WorkerTrustConfig,
	purpose agentv1.WorkerHandshakePurpose, token string) (*capsdk.WorkerHandshakeSession, error) {
	request, err := buildRequest(trust, purpose, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	challenge, err := requestPacket(connection, capsdk.WorkerHandshakeChallengeSubject, request)
	if err != nil {
		return nil, err
	}
	verified, err := capsdk.VerifyWorkerHandshakeChallenge(trust, request, challenge, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	authenticate, err := capsdk.BuildWorkerHandshakeAuthenticate(trust, verified, capability(trust), token, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	result, err := requestPacket(connection, capsdk.WorkerHandshakeAuthenticateSubject, authenticate)
	if err != nil {
		return nil, err
	}
	return capsdk.VerifyWorkerHandshakeResult(trust, verified, authenticate, result, time.Now().UTC())
}

func buildRequest(trust *capsdk.WorkerTrustConfig, purpose agentv1.WorkerHandshakePurpose,
	createdAt time.Time) (*agentv1.BusPacket, error) {
	nonce := make([]byte, capsdk.WorkerHandshakeNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, errors.New("random nonce unavailable")
	}
	requestID, err := randomID()
	if err != nil {
		return nil, err
	}
	traceID, err := randomID()
	if err != nil {
		return nil, err
	}
	return capsdk.BuildWorkerHandshakeChallengeRequest(trust, capsdk.WorkerHandshakeRequestOptions{
		RequestID: requestID, TraceID: traceID, Purpose: purpose,
		ClientNonce: nonce, CreatedAt: createdAt,
	})
}

func capability(trust *capsdk.WorkerTrustConfig) *agentv1.Handshake {
	return &agentv1.Handshake{
		ComponentId: trust.WorkerID, Role: agentv1.ComponentRole_COMPONENT_ROLE_WORKER,
		SupportedVersions: []int32{1}, Capabilities: map[string]bool{"progress": true},
		SdkVersion: trust.SDKVersion, ReadyTopics: []string{"job.interop"}, AgentName: trust.WorkerID,
	}
}

func requestPacket(connection *nats.Conn, subject string, packet *agentv1.BusPacket) (*agentv1.BusPacket, error) {
	data, err := capsdk.MarshalWorkerTrustPacket(packet)
	if err != nil {
		return nil, err
	}
	message, err := connection.Request(subject, data, 3*time.Second)
	if err != nil {
		return nil, err
	}
	return capsdk.UnmarshalWorkerTrustPacket(message.Data)
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", errors.New("random ID unavailable")
	}
	return hex.EncodeToString(value), nil
}

func issuePurpose() agentv1.WorkerHandshakePurpose {
	return agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_ISSUE
}

func renewPurpose() agentv1.WorkerHandshakePurpose {
	return agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW
}

func loadPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("worker private key is invalid")
	}
	privateKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("worker private key is not EC")
	}
	return privateKey, nil
}

func loadPublicKey(path string) (*ecdsa.PublicKey, error) {
	block, err := readPEM(path)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("scheduler public key is invalid")
	}
	publicKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("scheduler public key is not EC")
	}
	return publicKey, nil
}

func readPEM(path string) (*pem.Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("key file is unavailable")
	}
	block, rest := pem.Decode(data)
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, errors.New("key PEM is invalid")
	}
	return block, nil
}
