package capsdk

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestVerifyTrustedProductionPacketPreservesImmutableProof(t *testing.T) {
	key := testPrivateSigningKey()
	packet := productionPacketSignedForTest(t, key, "job.production")
	raw, err := SignProductionPacket(packet, key)
	if err != nil {
		t.Fatalf("SignProductionPacket: %v", err)
	}

	verified, err := VerifyTrustedProductionPacket(raw, strictProductionTrust(key, "job.production"))
	if err != nil {
		t.Fatalf("VerifyTrustedProductionPacket: %v", err)
	}
	digest, err := ProductionSignedBodyDigest(raw)
	if err != nil {
		t.Fatalf("ProductionSignedBodyDigest: %v", err)
	}
	if !verified.IsVerified() || verified.Subject() != "job.production" ||
		verified.SessionToken() != packet.GetAuthToken() || verified.Sender() != "worker-1" ||
		verified.Tenant() != "tenant-a" || verified.SignedBodyDigest() != digest ||
		!bytes.Equal(verified.MessageID(), packet.GetSignatureMetadata().GetMessageId()) {
		t.Fatalf("verified proof lost authenticated state: %#v", verified)
	}

	first := verified.Packet()
	first.TraceId = "mutated"
	first.SignatureMetadata.MessageId[0] ^= 0xff
	if got := verified.Packet(); got.GetTraceId() == "mutated" ||
		!bytes.Equal(got.GetSignatureMetadata().GetMessageId(), packet.GetSignatureMetadata().GetMessageId()) {
		t.Fatal("verified packet exposed mutable proof state")
	}
	messageID := verified.MessageID()
	messageID[0] ^= 0xff
	if bytes.Equal(messageID, verified.MessageID()) {
		t.Fatal("verified message ID accessor exposed mutable proof state")
	}
	if verified.SignedBodyDigest() == sha256.Sum256(nil) {
		t.Fatal("verified signed-body digest is empty")
	}
}

func TestZeroVerifiedProductionPacketCarriesNoAuthority(t *testing.T) {
	var verified VerifiedProductionPacket
	if verified.IsVerified() || verified.Packet() != nil || verified.Subject() != "" ||
		verified.SessionToken() != "" || verified.Sender() != "" || verified.Tenant() != "" ||
		verified.MessageID() != nil || verified.SignedBodyDigest() != ([32]byte{}) {
		t.Fatalf("zero verified packet carried authority: %#v", verified)
	}
}
