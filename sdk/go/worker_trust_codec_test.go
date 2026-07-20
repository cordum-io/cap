package capsdk

import (
	"bytes"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

func TestWorkerTrustPacketCodecRoundTrip(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	request := fixture.buildRequest(t, issuePurpose())
	encoded, err := MarshalWorkerTrustPacket(request)
	if err != nil {
		t.Fatalf("MarshalWorkerTrustPacket: %v", err)
	}
	decoded, err := UnmarshalWorkerTrustPacket(encoded)
	if err != nil {
		t.Fatalf("UnmarshalWorkerTrustPacket: %v", err)
	}
	if !proto.Equal(request, decoded) {
		t.Fatal("codec changed signed packet")
	}
}

func TestUnmarshalWorkerTrustPacketRejectsUnknownAndBounds(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	request := fixture.buildRequest(t, issuePurpose())
	request.GetWorkerHandshakeChallengeRequest().ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	encoded, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UnmarshalWorkerTrustPacket(encoded); err == nil {
		t.Fatal("unknown nested field was accepted")
	}
	for _, data := range [][]byte{nil, bytes.Repeat([]byte{1}, WorkerHandshakeMaxPacketSize+1)} {
		if _, err := UnmarshalWorkerTrustPacket(data); !errors.Is(err, ErrWorkerHandshakePacket) {
			t.Errorf("UnmarshalWorkerTrustPacket(size=%d) error=%v", len(data), err)
		}
	}
}

func TestTrustHandshakeTranscriptRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	packet := fixture.buildRequest(t, issuePurpose())
	packet.GetWorkerHandshakeChallengeRequest().ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	if _, err := TrustHandshakeDigest(packet); !errors.Is(err, ErrWorkerHandshakePacket) {
		t.Fatalf("TrustHandshakeDigest error=%v, want %v", err, ErrWorkerHandshakePacket)
	}
}

func TestBuildWorkerHandshakeAuthenticateRejectsPurposeTokenMismatch(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	tests := []struct {
		purpose agentv1.WorkerHandshakePurpose
		token   string
	}{
		{issuePurpose(), "unexpected"},
		{renewPurpose(), ""},
		{renewPurpose(), " padded-token "},
	}
	for _, test := range tests {
		request := fixture.buildRequest(t, test.purpose)
		verified, err := VerifyWorkerHandshakeChallenge(fixture.config, request, fixture.buildChallenge(t, request), fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		_, err = BuildWorkerHandshakeAuthenticate(fixture.config, verified, validWorkerCapability(fixture.config), test.token, fixture.now)
		if !errors.Is(err, ErrWorkerHandshakePacket) {
			t.Errorf("purpose=%v token=%q error=%v", test.purpose, test.token, err)
		}
	}
}

func TestBuildWorkerHandshakeChallengeRequestRejectsUnsafeCorrelation(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	base := WorkerHandshakeRequestOptions{
		RequestID: "request-1", TraceID: "trace-1", Purpose: issuePurpose(),
		ClientNonce: bytes.Repeat([]byte{1}, WorkerHandshakeNonceSize), CreatedAt: fixture.now,
	}
	for _, unsafe := range []string{" request-1", "trace\nforged", string([]byte{'x', 0, 'y'})} {
		options := base
		options.TraceID = unsafe
		if _, err := BuildWorkerHandshakeChallengeRequest(fixture.config, options); !errors.Is(err, ErrWorkerHandshakePacket) {
			t.Errorf("trace=%q error=%v", unsafe, err)
		}
	}
}

func TestBuildWorkerHandshakeChallengeRequestRejectsInvalidTimestamp(t *testing.T) {
	t.Parallel()
	fixture := newWorkerTrustClientFixture(t)
	options := WorkerHandshakeRequestOptions{
		RequestID: "request-1", TraceID: "trace-1", Purpose: issuePurpose(),
		ClientNonce: bytes.Repeat([]byte{1}, WorkerHandshakeNonceSize), CreatedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if _, err := BuildWorkerHandshakeChallengeRequest(fixture.config, options); !errors.Is(err, ErrWorkerHandshakePacket) {
		t.Fatalf("BuildWorkerHandshakeChallengeRequest error=%v", err)
	}
}
