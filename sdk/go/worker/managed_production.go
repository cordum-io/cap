package worker

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultManagedProductionLifetime = time.Minute

var errManagedProductionSessionUnavailable = errors.New("managed production: authenticated session unavailable")

// ManagedProductionConfig enables the fail-closed CAP-PRODUCTION boundary for
// ManagedWorker. The actual delivered NATS subject is always the signature
// audience; Trust.Audience must therefore be empty.
type ManagedProductionConfig struct {
	Trust             capsdk.ProductionTrustStore
	Replay            ManagedReplayStore
	ResourceResolvers []string
	Stream            string
	KeyID             string
	MessageLifetime   time.Duration
	ReplayLease       time.Duration
	Enabled           bool
}

func cloneManagedProductionConfig(config ManagedConfig) ManagedConfig {
	if !config.Production.Enabled {
		return config
	}
	config.PrivateKey = cloneECDSAPrivateKey(config.PrivateKey)
	keys := config.Production.Trust.PublicKeys
	config.Production.Trust.PublicKeys = make(map[string]*ecdsa.PublicKey, len(keys))
	for keyID, key := range keys {
		config.Production.Trust.PublicKeys[keyID] = cloneECDSAPublicKey(key)
	}
	config.Production.ResourceResolvers = append([]string(nil), config.Production.ResourceResolvers...)
	return config
}

func validateManagedProductionConfig(config ManagedConfig) error {
	production := config.Production
	if !production.Enabled {
		return nil
	}
	if config.AllowUnsigned || config.WorkerTrustMode != capsdk.WorkerTrustModeEnforce || config.WorkerTrust == nil {
		return errors.New("managed production: authenticated enforce mode required")
	}
	if !validManagedProductionKey(config.PrivateKey) {
		return errors.New("managed production: P-256 private key required")
	}
	if !validManagedProductionID(production.KeyID) {
		return errors.New("managed production: canonical bounded key id required")
	}
	if !validManagedReplayStore(production.Replay) {
		return errors.New("managed production: durable settlement store required")
	}
	if !validManagedNATSName(production.Stream) {
		return errors.New("managed production: canonical JetStream name required")
	}
	if err := capsdk.ValidateResourceResolvers(production.ResourceResolvers); err != nil {
		return fmt.Errorf("managed production: %w", err)
	}
	if err := validateManagedProductionTrust(production.Trust); err != nil {
		return err
	}
	if err := validateManagedProductionAuthority(config); err != nil {
		return err
	}
	return validateManagedProductionLifetime(production)
}

func validateManagedProductionTrust(trust capsdk.ProductionTrustStore) error {
	if trust.Audience != "" || (trust.ResolveKey == nil && len(trust.PublicKeys) == 0) {
		return errors.New("managed production: subject-bound scheduler trust required")
	}
	for keyID, key := range trust.PublicKeys {
		if !validManagedProductionID(keyID) || !validManagedProductionPublicKey(key) {
			return errors.New("managed production: scheduler trust contains an invalid key")
		}
	}
	return nil
}

func validManagedProductionID(value string) bool {
	if value == "" || len(value) > capsdk.WorkerHandshakeMaxIdentityLength || strings.TrimSpace(value) != value {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func validManagedNATSName(value string) bool {
	if value == "" || len(value) > 255 || strings.TrimSpace(value) != value {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e || strings.ContainsRune(".*>/\\", rune(value[index])) {
			return false
		}
	}
	return true
}

func validManagedProductionPublicKey(key *ecdsa.PublicKey) bool {
	return key != nil && key.Curve == elliptic.P256() && key.X != nil && key.Y != nil &&
		key.Curve.IsOnCurve(key.X, key.Y)
}

func validateManagedProductionAuthority(config ManagedConfig) error {
	trust, authority := config.Production.Trust, config.WorkerTrust
	if trust.Tenant != "" && trust.Tenant != authority.TenantID {
		return errors.New("managed production: trust tenant conflicts with authenticated session")
	}
	if trust.Sender != "" && trust.Sender != authority.ExpectedSchedulerID {
		return errors.New("managed production: trust sender conflicts with authenticated session")
	}
	return nil
}

func validateManagedProductionLifetime(config ManagedProductionConfig) error {
	lifetime := managedProductionLifetime(config)
	lease := managedReplayLease(config)
	maxLifetime := config.Trust.MaxLifetime
	if maxLifetime == 0 {
		maxLifetime = capsdk.DefaultProductionMaxLifetime
	}
	if config.MessageLifetime < 0 || lifetime <= 0 || lifetime > capsdk.DefaultProductionMaxLifetime ||
		config.ReplayLease < 0 || lease <= 0 || lease >= lifetime ||
		maxLifetime < 0 || maxLifetime > capsdk.DefaultProductionMaxLifetime ||
		config.Trust.ClockSkew < 0 || config.Trust.ClockSkew > capsdk.MaximumProductionClockSkew ||
		config.Trust.ClockSkew > maxLifetime {
		return errors.New("managed production: invalid signature lifetime or clock skew")
	}
	return nil
}

func managedReplayLease(config ManagedProductionConfig) time.Duration {
	if config.ReplayLease > 0 {
		return config.ReplayLease
	}
	lease := defaultManagedReplayLease
	lifetime := managedProductionLifetime(config)
	if lease >= lifetime {
		lease = lifetime / 3
	}
	return lease
}

func validManagedProductionKey(key *ecdsa.PrivateKey) bool {
	if key == nil || key.Curve != elliptic.P256() || key.D == nil || key.D.Sign() <= 0 ||
		key.D.Cmp(key.Curve.Params().N) >= 0 || key.X == nil || key.Y == nil || !key.Curve.IsOnCurve(key.X, key.Y) {
		return false
	}
	x, y := key.Curve.ScalarBaseMult(key.D.Bytes())
	return x.Cmp(key.X) == 0 && y.Cmp(key.Y) == 0
}

func managedProductionLifetime(config ManagedProductionConfig) time.Duration {
	if config.MessageLifetime > 0 {
		return config.MessageLifetime
	}
	return defaultManagedProductionLifetime
}

func (w *ManagedWorker) admitManagedProductionPacket(raw []byte, audience string) (*agentv1.BusPacket, error) {
	trust, err := w.managedProductionTrust(audience)
	if err != nil {
		return nil, err
	}
	verified, err := capsdk.VerifyTrustedProductionPacket(raw, trust)
	if err != nil {
		return nil, err
	}
	packet := verified.Packet()
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return nil, err
	}
	if err := w.validateManagedProductionRequest(packet); err != nil {
		return nil, err
	}
	return packet, nil
}

func (w *ManagedWorker) managedProductionTrust(audience string) (capsdk.ProductionTrustStore, error) {
	if audience == "" || strings.TrimSpace(audience) != audience {
		return capsdk.ProductionTrustStore{}, capsdk.ErrAudienceMismatch
	}
	if w.sessionToken() == "" {
		return capsdk.ProductionTrustStore{}, errManagedProductionSessionUnavailable
	}
	trust := w.cfg.Production.Trust
	trust.Audience = audience
	trust.Tenant = w.cfg.WorkerTrust.TenantID
	trust.Sender = w.cfg.WorkerTrust.ExpectedSchedulerID
	return trust, nil
}

func (w *ManagedWorker) validateManagedProductionRequest(packet *agentv1.BusPacket) error {
	request, authoritative := packet.GetJobRequest(), packet.GetIdentity()
	if request == nil || authoritative == nil || authoritative.GetTenantId() == "" ||
		authoritative.GetPrincipalId() == "" || authoritative.GetActorId() == "" || request.GetIdentity() == nil ||
		!proto.Equal(request.GetIdentity(), authoritative) {
		return capsdk.ErrIdentityMismatch
	}
	if request.GetTopic() != packet.GetSignatureMetadata().GetAudience() {
		return capsdk.ErrAudienceMismatch
	}
	if err := capsdk.ValidateIdentityBinding(request, authoritative); err != nil {
		return err
	}
	if request.GetCompensation() != nil {
		if err := capsdk.ValidateCompensationMonotonicity(request, request.GetCompensation()); err != nil {
			return err
		}
	}
	if err := validateManagedProductionResources(
		request, w.cfg.Production.ResourceResolvers, time.Now(),
	); err != nil {
		return err
	}
	dispatch := request.GetDispatch()
	if dispatch == nil || dispatch.GetDispatchId() == "" || dispatch.GetAttempt() == 0 ||
		dispatch.GetAssignedWorkerId() != w.workerID {
		return capsdk.ErrStaleDispatchEvent
	}
	return nil
}

func (w *ManagedWorker) marshalManagedProductionPacket(
	subject string, packet *agentv1.BusPacket,
) ([]byte, error) {
	messageID := make([]byte, 16)
	if _, err := rand.Read(messageID); err != nil {
		return nil, fmt.Errorf("managed production: message id: %w", err)
	}
	packet.SignatureMetadata = &agentv1.SignatureMetadata{
		ProfileVersion: capsdk.ProductionProfileVersion, Algorithm: capsdk.ProductionAlgorithm,
		MessageId: messageID, Audience: subject,
		ExpiresAt: timestamppb.New(time.Now().Add(managedProductionLifetime(w.cfg.Production))),
		KeyId:     w.cfg.Production.KeyID,
	}
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return nil, fmt.Errorf("validate production packet: %w", err)
	}
	return capsdk.SignProductionPacket(packet, w.cfg.PrivateKey)
}
