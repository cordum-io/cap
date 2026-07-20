package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

func unixSecond(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

func produce(req produceRequest) (produceResponse, error) {
	key, err := parsePrivateKey(req.PrivateKeyPEM)
	if err != nil {
		return produceResponse{}, err
	}
	resp := produceResponse{SDK: "go"}
	for _, c := range req.Corpus.Cases {
		pkt, err := buildPacket(c, req.Corpus, req.KeyID, req.ExpiresAtUnix)
		if err != nil {
			return produceResponse{}, err
		}
		raw, err := capsdk.SignProductionPacket(pkt, key)
		if err != nil {
			return produceResponse{}, fmt.Errorf("case %s: sign: %w", c.Name, err)
		}
		normalized, preimage, err := digests(raw, pkt)
		if err != nil {
			return produceResponse{}, fmt.Errorf("case %s: digests: %w", c.Name, err)
		}
		resp.Fixtures = append(resp.Fixtures, fixtureOut{
			Case:             c.Name,
			Wire:             base64.StdEncoding.EncodeToString(raw),
			NormalizedDigest: normalized,
			PreimageDigest:   preimage,
			KeyID:            req.KeyID,
		})
	}
	return resp, nil
}

func consume(req consumeRequest) (consumeResponse, error) {
	resp := consumeResponse{SDK: "go"}
	for _, job := range req.Jobs {
		resp.Results = append(resp.Results, runJob(req, job))
	}
	return resp, nil
}

func runJob(req consumeRequest, job consumeJob) consumeResult {
	out := consumeResult{ID: job.ID}
	raw, err := base64.StdEncoding.DecodeString(job.Wire)
	if err != nil {
		out.Error = fmt.Sprintf("decode wire: %v", err)
		return out
	}
	pub, err := parsePublicKey(job.PublicKeyDER)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	trust := capsdk.ProductionTrustStore{
		Audience:   req.Audience,
		Tenant:     req.TenantID,
		Sender:     req.SenderID,
		PublicKeys: map[string]*ecdsa.PublicKey{job.KeyID: pub},
	}
	packet, err := capsdk.VerifyProductionPacket(raw, trust)
	if err != nil {
		out.Error = fmt.Sprintf("verify: %v", err)
		return out
	}
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		out.Error = fmt.Sprintf("validate: %v", err)
		return out
	}
	// Digests are recomputed from the received bytes and from an independent
	// re-encode of the decoded message, never copied from the producer.
	normalized, preimage, err := digests(raw, packet)
	if err != nil {
		out.Error = fmt.Sprintf("digests: %v", err)
		return out
	}
	out.OK, out.NormalizedDigest, out.PreimageDigest = true, normalized, preimage
	return out
}

// digests returns the normalized-semantic and unsigned-preimage digests of an
// exact CAP-PRODUCTION wire packet. The preimage digest covers the unsigned
// bytes exactly as received; the normalized digest covers a fresh deterministic
// re-encode of the decoded message, so a producer that emits non-canonical
// bytes diverges from its consumers instead of being blessed by them.
func digests(raw []byte, packet *agentv1.BusPacket) (normalized, preimage string, err error) {
	body, err := capsdk.ProductionSignedBodyDigest(raw)
	if err != nil {
		return "", "", err
	}
	clone := proto.Clone(packet).(*agentv1.BusPacket)
	clone.Signature = nil
	canonical, err := capsdk.MarshalDeterministic(clone)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), hex.EncodeToString(body[:]), nil
}

func parsePrivateKey(pemText string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("private key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ECDSA")
	}
	return key, nil
}

func parsePublicKey(b64 string) (*ecdsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ECDSA")
	}
	return key, nil
}
