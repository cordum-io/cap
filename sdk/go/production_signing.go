package capsdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const (
	ProductionProfileVersion     = "cap-production-v1"
	ProductionAlgorithm          = "ECDSA-P256-SHA256"
	DefaultProductionMaxLifetime = 5 * time.Minute
	MaximumProductionClockSkew   = time.Minute
	DefaultProductionMaxRawBytes = 1 << 20
)

var productionDomain = []byte("CAP-PRODUCTION-SIGNATURE-V1\x00")

var (
	ErrMissingSignatureMetadata = errors.New("capsdk: missing signature metadata")
	ErrSignatureExpired         = errors.New("capsdk: signature expired")
	ErrAudienceMismatch         = errors.New("capsdk: signature audience mismatch")
	ErrUnknownKeyID             = errors.New("capsdk: unknown key id")
	ErrDuplicateSignatureField  = errors.New("capsdk: duplicate signature field")
	ErrMalformedProductionWire  = errors.New("capsdk: malformed production wire")
	ErrUnsupportedProductionKey = errors.New("capsdk: production signatures require ECDSA P-256")
	ErrProductionTrustRequired  = errors.New("capsdk: production verifier requires audience, tenant, sender, and local keys")
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
	if !validProductionPublicKey(&key.PublicKey) || key.D == nil {
		return nil, ErrUnsupportedProductionKey
	}
	if err := validateMetadataForSigning(packet.GetSignatureMetadata(), time.Now()); err != nil {
		return nil, err
	}
	if err := validateNoProductionUnknowns(packet.ProtoReflect()); err != nil {
		return nil, err
	}
	unsignedPacket := proto.Clone(packet).(*agentv1.BusPacket)
	unsignedPacket.Signature = nil
	unsigned, err := proto.Marshal(unsignedPacket)
	if err != nil {
		return nil, fmt.Errorf("marshal unsigned production packet: %w", err)
	}
	if err := validateUnsignedProductionWire(unsigned); err != nil {
		return nil, err
	}
	digest := productionDigest(unsigned)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign production packet: %w", err)
	}
	raw := appendSignatureField(unsigned, signature)
	if len(raw) > DefaultProductionMaxRawBytes {
		return nil, ErrMalformedProductionWire
	}
	return raw, nil
}

func VerifyProductionPacket(raw []byte, trust ProductionTrustStore) (*agentv1.BusPacket, error) {
	if err := validateProductionTrust(trust); err != nil {
		return nil, err
	}
	unsigned, signature, err := extractSignatureField(raw)
	if err != nil {
		return nil, err
	}
	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(unsigned, packet); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedProductionWire, err)
	}
	if err := validateNoProductionUnknowns(packet.ProtoReflect()); err != nil {
		return nil, err
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

func validateProductionTrust(trust ProductionTrustStore) error {
	if !validProductionAuthority(trust.Audience) || !validProductionAuthority(trust.Tenant) ||
		!validProductionAuthority(trust.Sender) ||
		(trust.ResolveKey == nil && len(trust.PublicKeys) == 0) {
		return ErrProductionTrustRequired
	}
	return nil
}

func validProductionAuthority(value string) bool {
	return value != "" && strings.TrimSpace(value) == value
}

// ProductionSignedBodyDigest returns SHA-256 over the exact unsigned wire
// bytes covered by a CAP-PRODUCTION signature. Replay stores use this digest
// so a freshly generated ECDSA signature over the same body remains an
// idempotent redelivery rather than a false conflict.
func ProductionSignedBodyDigest(raw []byte) ([32]byte, error) {
	unsigned, _, err := extractSignatureField(raw)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(unsigned), nil
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

func validateMetadataShape(metadata *agentv1.SignatureMetadata) error {
	if metadata == nil {
		return ErrMissingSignatureMetadata
	}
	expiresAt := metadata.GetExpiresAt()
	if metadata.GetProfileVersion() != ProductionProfileVersion || metadata.GetAlgorithm() != ProductionAlgorithm ||
		len(metadata.GetMessageId()) != 16 || metadata.GetAudience() == "" || expiresAt == nil || metadata.GetKeyId() == "" {
		return ErrMalformedProductionWire
	}
	if err := expiresAt.CheckValid(); err != nil {
		return ErrMalformedProductionWire
	}
	return nil
}

func validateMetadataForSigning(metadata *agentv1.SignatureMetadata, now time.Time) error {
	if err := validateMetadataShape(metadata); err != nil {
		return err
	}
	expiresAt := metadata.GetExpiresAt().AsTime()
	if !expiresAt.After(now) {
		return ErrSignatureExpired
	}
	if expiresAt.After(now.Add(DefaultProductionMaxLifetime)) {
		return ErrMalformedProductionWire
	}
	return nil
}

func validateMetadata(metadata *agentv1.SignatureMetadata, trust ProductionTrustStore) error {
	if err := validateMetadataShape(metadata); err != nil {
		return err
	}
	maxLifetime := trust.MaxLifetime
	if maxLifetime == 0 {
		maxLifetime = DefaultProductionMaxLifetime
	}
	if maxLifetime < 0 || maxLifetime > DefaultProductionMaxLifetime ||
		trust.ClockSkew < 0 || trust.ClockSkew > MaximumProductionClockSkew ||
		trust.ClockSkew > maxLifetime {
		return ErrMalformedProductionWire
	}
	now := time.Now()
	if trust.Now != nil {
		now = trust.Now()
	}
	if !metadata.GetExpiresAt().AsTime().After(now.Add(-trust.ClockSkew)) {
		return ErrSignatureExpired
	}
	if metadata.GetExpiresAt().AsTime().After(now.Add(maxLifetime + trust.ClockSkew)) {
		return ErrMalformedProductionWire
	}
	if metadata.GetAudience() != trust.Audience {
		return ErrAudienceMismatch
	}
	return nil
}

func resolveProductionKey(packet *agentv1.BusPacket, keyID string, trust ProductionTrustStore) (*ecdsa.PublicKey, error) {
	tenant, sender := packet.GetIdentity().GetTenantId(), packet.GetSenderId()
	if tenant != trust.Tenant || sender != trust.Sender {
		return nil, ErrUnknownKeyID
	}
	if trust.ResolveKey != nil {
		key, err := trust.ResolveKey(tenant, sender, keyID)
		if err != nil || key == nil {
			return nil, ErrUnknownKeyID
		}
		if !validProductionPublicKey(key) {
			return nil, ErrUnsupportedProductionKey
		}
		return key, nil
	}
	key := trust.PublicKeys[keyID]
	if key == nil {
		return nil, ErrUnknownKeyID
	}
	if !validProductionPublicKey(key) {
		return nil, ErrUnsupportedProductionKey
	}
	return key, nil
}

func validProductionPublicKey(key *ecdsa.PublicKey) bool {
	return key != nil && key.Curve == elliptic.P256() && key.X != nil && key.Y != nil &&
		key.Curve.IsOnCurve(key.X, key.Y)
}
