package capsdk

import (
	"crypto/ecdsa"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProductionVerifyRejectsUnboundedExpiryByDefault(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	packet.SignatureMetadata.ExpiresAt = futureTimestampAfter(10 * time.Minute)
	if _, err := SignProductionPacket(packet, key); !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("SignProductionPacket error=%v, want bounded-lifetime rejection", err)
	}
	unsigned, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	raw := signProductionWireForTest(t, unsigned, key)

	_, err = VerifyProductionPacket(raw, strictProductionTrust(key, "tenant-a"))
	if !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("VerifyProductionPacket error=%v, want bounded-lifetime rejection", err)
	}
}

func TestProductionVerifyRequiresExpectedAuthority(t *testing.T) {
	key := testPrivateSigningKey()
	raw, err := SignProductionPacket(productionPacketSignedForTest(t, key, "tenant-a"), key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	tests := map[string]ProductionTrustStore{
		"missing audience": {Tenant: "tenant-a", Sender: "worker-1", PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey}},
		"missing tenant":   {Audience: "tenant-a", Sender: "worker-1", PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey}},
		"missing sender":   {Audience: "tenant-a", Tenant: "tenant-a", PublicKeys: map[string]*ecdsa.PublicKey{"k1": &key.PublicKey}},
	}
	for name, trust := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyProductionPacket(raw, trust); !errors.Is(err, ErrProductionTrustRequired) {
				t.Fatalf("VerifyProductionPacket error=%v, want ErrProductionTrustRequired", err)
			}
		})
	}
}

func TestProductionVerifyRejectsWhitespaceAuthority(t *testing.T) {
	key := testPrivateSigningKey()
	raw, err := SignProductionPacket(productionPacketSignedForTest(t, key, "tenant-a"), key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	for name, mutate := range map[string]func(*ProductionTrustStore){
		"blank audience":  func(trust *ProductionTrustStore) { trust.Audience = " \t" },
		"padded audience": func(trust *ProductionTrustStore) { trust.Audience = " tenant-a" },
		"blank tenant":    func(trust *ProductionTrustStore) { trust.Tenant = " \t" },
		"padded tenant":   func(trust *ProductionTrustStore) { trust.Tenant = "tenant-a " },
		"blank sender":    func(trust *ProductionTrustStore) { trust.Sender = " \t" },
		"padded sender":   func(trust *ProductionTrustStore) { trust.Sender = " worker-1" },
	} {
		t.Run(name, func(t *testing.T) {
			trust := strictProductionTrust(key, "tenant-a")
			mutate(&trust)
			if _, err := VerifyProductionPacket(raw, trust); !errors.Is(err, ErrProductionTrustRequired) {
				t.Fatalf("VerifyProductionPacket error=%v, want ErrProductionTrustRequired", err)
			}
		})
	}
}

func TestProductionVerifyRejectsAuthorityMismatch(t *testing.T) {
	key := testPrivateSigningKey()
	raw, err := SignProductionPacket(productionPacketSignedForTest(t, key, "tenant-a"), key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	for name, mutate := range map[string]func(*ProductionTrustStore){
		"tenant": func(trust *ProductionTrustStore) { trust.Tenant = "other-tenant" },
		"sender": func(trust *ProductionTrustStore) { trust.Sender = "other-sender" },
	} {
		t.Run(name, func(t *testing.T) {
			trust := strictProductionTrust(key, "tenant-a")
			mutate(&trust)
			if _, err := VerifyProductionPacket(raw, trust); !errors.Is(err, ErrUnknownKeyID) {
				t.Fatalf("VerifyProductionPacket error=%v, want authority mismatch rejection", err)
			}
		})
	}
}

func TestProductionVerifyRejectsNegativeClockSkew(t *testing.T) {
	key := testPrivateSigningKey()
	raw, err := SignProductionPacket(productionPacketSignedForTest(t, key, "tenant-a"), key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	trust := strictProductionTrust(key, "tenant-a")
	trust.ClockSkew = -time.Second
	_, err = VerifyProductionPacket(raw, trust)
	if !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("VerifyProductionPacket error=%v, want invalid skew rejection", err)
	}
}

func TestProductionVerifyRejectsClockSkewBeyondLifetime(t *testing.T) {
	key := testPrivateSigningKey()
	raw, err := SignProductionPacket(productionPacketSignedForTest(t, key, "tenant-a"), key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	trust := strictProductionTrust(key, "tenant-a")
	trust.MaxLifetime, trust.ClockSkew = 30*time.Second, 45*time.Second
	_, err = VerifyProductionPacket(raw, trust)
	if !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("VerifyProductionPacket error=%v, want excessive-skew rejection", err)
	}
}

func TestProductionVerifyRejectsClockSkewBeyondProfileMaximum(t *testing.T) {
	key := testPrivateSigningKey()
	raw, err := SignProductionPacket(productionPacketSignedForTest(t, key, "tenant-a"), key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	trust := strictProductionTrust(key, "tenant-a")
	trust.ClockSkew = MaximumProductionClockSkew + time.Second
	_, err = VerifyProductionPacket(raw, trust)
	if !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("VerifyProductionPacket error=%v, want absolute skew rejection", err)
	}
}

func TestProductionVerifyRejectsLifetimeBeyondProfileMaximum(t *testing.T) {
	key := testPrivateSigningKey()
	raw, err := SignProductionPacket(productionPacketSignedForTest(t, key, "tenant-a"), key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	trust := strictProductionTrust(key, "tenant-a")
	trust.MaxLifetime = DefaultProductionMaxLifetime + time.Second
	_, err = VerifyProductionPacket(raw, trust)
	if !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("VerifyProductionPacket error=%v, want excessive-lifetime rejection", err)
	}
}

func TestProductionVerifyRejectsNegativeMaxLifetime(t *testing.T) {
	key := testPrivateSigningKey()
	raw, err := SignProductionPacket(productionPacketSignedForTest(t, key, "tenant-a"), key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}
	trust := strictProductionTrust(key, "tenant-a")
	trust.MaxLifetime = -time.Second
	_, err = VerifyProductionPacket(raw, trust)
	if !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("VerifyProductionPacket error=%v, want negative-lifetime rejection", err)
	}
}

func TestProductionVerifyRejectsExactExpiryBoundary(t *testing.T) {
	key := testPrivateSigningKey()
	now := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	packet.SignatureMetadata.ExpiresAt = timestamppb.New(now)
	unsigned, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	raw := signProductionWireForTest(t, unsigned, key)
	trust := strictProductionTrust(key, "tenant-a")
	trust.Now = func() time.Time { return now }
	_, err = VerifyProductionPacket(raw, trust)
	if !errors.Is(err, ErrSignatureExpired) {
		t.Fatalf("VerifyProductionPacket error=%v, want exact-boundary expiry", err)
	}
}

func TestProductionSigningRejectsInvalidExpiryTimestamp(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	packet.SignatureMetadata.ExpiresAt.Nanos = -1
	if _, err := SignProductionPacket(packet, key); !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("SignProductionPacket error=%v, want invalid timestamp rejection", err)
	}
	unsigned, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	raw := signProductionWireForTest(t, unsigned, key)
	_, err = VerifyProductionPacket(raw, strictProductionTrust(key, "tenant-a"))
	if !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("VerifyProductionPacket error=%v, want invalid timestamp rejection", err)
	}
}

func TestProductionSignerRejectsUnknownTopLevelWire(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "tenant-a")
	packet.ProtoReflect().SetUnknown(appendBytesField(nil, 99, []byte("unknown")))
	if _, err := SignProductionPacket(packet, key); !errors.Is(err, ErrMalformedProductionWire) {
		t.Fatalf("SignProductionPacket error=%v, want unknown-field rejection", err)
	}
}

func TestInMemoryReplayStorePurgesExpiredEntries(t *testing.T) {
	store := NewInMemoryReplayStore()
	store.entries["expired"] = replayEntry{digest: []byte("old"), expires: time.Now().Add(-time.Minute)}

	_, err := store.Admit(
		"tenant-a", "audience-a", "sender-a", []byte("0123456789abcdef"),
		[]byte("digest-a"), time.Now().Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if _, exists := store.entries["expired"]; exists {
		t.Fatal("expired replay entry was not purged")
	}
}

func TestInMemoryReplayStoreUsesUnambiguousTupleKey(t *testing.T) {
	store := NewInMemoryReplayStore()
	expires := time.Now().Add(time.Minute)
	messageID := []byte("0123456789abcdef")
	first, err := store.Admit("a\x00b", "c", "d", messageID, []byte("one"), expires)
	if err != nil || first != ReplayOutcomeFirst {
		t.Fatalf("first Admit = (%v, %v), want first, nil", first, err)
	}
	second, err := store.Admit("a", "b", "c\x00d", messageID, []byte("two"), expires)
	if err != nil || second != ReplayOutcomeFirst {
		t.Fatalf("second Admit = (%v, %v), want independent first, nil", second, err)
	}
}
