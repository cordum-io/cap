package capsdk

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const (
	ProductionProfileVersion = "cap-production-v1"
	ProductionAlgorithm      = "ECDSA-P256-SHA256"
)

var productionDomain = []byte("CAP-PRODUCTION-SIGNATURE-V1\x00")

var (
	ErrMissingSignatureMetadata = errors.New("capsdk: missing signature metadata")
	ErrSignatureExpired         = errors.New("capsdk: signature expired")
	ErrAudienceMismatch         = errors.New("capsdk: signature audience mismatch")
	ErrUnknownKeyID             = errors.New("capsdk: unknown key id")
	ErrDuplicateSignatureField  = errors.New("capsdk: duplicate signature field")
	ErrMalformedProductionWire  = errors.New("capsdk: malformed production wire")
)

type ProductionTrustStore struct {
	Audience    string
	Tenant      string
	Sender      string
	PublicKeys  map[string]*ecdsa.PublicKey
	ResolveKey  func(tenant, sender, keyID string) (*ecdsa.PublicKey, error)
	Now         func() time.Time
	MaxLifetime time.Duration
	ClockSkew   time.Duration
}

func SignProductionPacket(packet *agentv1.BusPacket, key *ecdsa.PrivateKey) ([]byte, error) {
	if packet == nil || key == nil {
		return nil, errors.New("capsdk: packet and private key are required")
	}
	if err := validateMetadataShape(packet.GetSignatureMetadata()); err != nil {
		return nil, err
	}
	unsignedPacket := proto.Clone(packet).(*agentv1.BusPacket)
	unsignedPacket.Signature = nil
	unsigned, err := proto.Marshal(unsignedPacket)
	if err != nil {
		return nil, fmt.Errorf("marshal unsigned production packet: %w", err)
	}
	digest := productionDigest(unsigned)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign production packet: %w", err)
	}
	return appendSignatureField(unsigned, signature), nil
}

func VerifyProductionPacket(raw []byte, trust ProductionTrustStore) (*agentv1.BusPacket, error) {
	unsigned, signature, err := extractSignatureField(raw)
	if err != nil {
		return nil, err
	}
	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(unsigned, packet); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedProductionWire, err)
	}
	metadata := packet.GetSignatureMetadata()
	if err := validateMetadata(metadata, trust); err != nil {
		return nil, err
	}
	key, err := resolveProductionKey(packet, metadata.GetKeyId(), trust)
	if err != nil {
		return nil, err
	}
	digest := productionDigest(unsigned)
	if !ecdsa.VerifyASN1(key, digest[:], signature) {
		return nil, ErrInvalidSignature
	}
	packet.Signature = append([]byte(nil), signature...)
	return packet, nil
}

func productionDigest(unsigned []byte) [32]byte {
	input := make([]byte, 0, len(productionDomain)+len(unsigned))
	input = append(input, productionDomain...)
	input = append(input, unsigned...)
	return sha256.Sum256(input)
}

func appendSignatureField(unsigned, signature []byte) []byte {
	result := append([]byte(nil), unsigned...)
	result = protowire.AppendTag(result, 14, protowire.BytesType)
	return protowire.AppendBytes(result, signature)
}

func extractSignatureField(raw []byte) ([]byte, []byte, error) {
	unsigned := make([]byte, 0, len(raw))
	var signature []byte
	for offset := 0; offset < len(raw); {
		fieldStart := offset
		number, wireType, n, canonical := consumeCanonicalTag(raw[offset:])
		if n < 0 || !canonical {
			return nil, nil, ErrMalformedProductionWire
		}
		offset += n
		valueLength := protowire.ConsumeFieldValue(number, wireType, raw[offset:])
		if valueLength < 0 || offset+valueLength > len(raw) {
			return nil, nil, ErrMalformedProductionWire
		}
		fieldEnd := offset + valueLength
		if number != 14 {
			unsigned = append(unsigned, raw[fieldStart:fieldEnd]...)
		} else {
			if signature != nil {
				return nil, nil, ErrDuplicateSignatureField
			}
			if wireType != protowire.BytesType {
				return nil, nil, ErrMalformedProductionWire
			}
			value, consumed := protowire.ConsumeBytes(raw[offset:fieldEnd])
			if consumed != valueLength || len(value) == 0 {
				return nil, nil, ErrMalformedProductionWire
			}
			signature = append([]byte(nil), value...)
		}
		offset = fieldEnd
	}
	if signature == nil {
		return nil, nil, ErrMissingSignature
	}
	return unsigned, signature, nil
}

func consumeCanonicalTag(raw []byte) (protowire.Number, protowire.Type, int, bool) {
	number, wireType, n := protowire.ConsumeTag(raw)
	if n < 0 {
		return 0, 0, n, false
	}
	canonical := protowire.AppendTag(nil, number, wireType)
	return number, wireType, n, len(canonical) == n && string(canonical) == string(raw[:n])
}

func validateMetadataShape(metadata *agentv1.SignatureMetadata) error {
	if metadata == nil {
		return ErrMissingSignatureMetadata
	}
	if metadata.GetProfileVersion() != ProductionProfileVersion || metadata.GetAlgorithm() != ProductionAlgorithm ||
		len(metadata.GetMessageId()) != 16 || metadata.GetAudience() == "" || metadata.GetExpiresAt() == nil || metadata.GetKeyId() == "" {
		return ErrMalformedProductionWire
	}
	return nil
}

func validateMetadata(metadata *agentv1.SignatureMetadata, trust ProductionTrustStore) error {
	if err := validateMetadataShape(metadata); err != nil {
		return err
	}
	now := time.Now()
	if trust.Now != nil {
		now = trust.Now()
	}
	if metadata.GetExpiresAt().AsTime().Before(now.Add(-trust.ClockSkew)) {
		return ErrSignatureExpired
	}
	if trust.MaxLifetime > 0 && metadata.GetExpiresAt().AsTime().After(now.Add(trust.MaxLifetime+trust.ClockSkew)) {
		return ErrMalformedProductionWire
	}
	if trust.Audience != "" && metadata.GetAudience() != trust.Audience {
		return ErrAudienceMismatch
	}
	return nil
}

func resolveProductionKey(packet *agentv1.BusPacket, keyID string, trust ProductionTrustStore) (*ecdsa.PublicKey, error) {
	tenant, sender := packet.GetIdentity().GetTenantId(), packet.GetSenderId()
	if trust.Tenant != "" && tenant != trust.Tenant || trust.Sender != "" && sender != trust.Sender {
		return nil, ErrUnknownKeyID
	}
	if trust.ResolveKey != nil {
		key, err := trust.ResolveKey(tenant, sender, keyID)
		if err != nil || key == nil {
			return nil, ErrUnknownKeyID
		}
		return key, nil
	}
	key := trust.PublicKeys[keyID]
	if key == nil {
		return nil, ErrUnknownKeyID
	}
	return key, nil
}
