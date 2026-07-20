package capsdk

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestVerifyWorkerHandshakeChallengeRejectsRequestCreatedAtSkew(t *testing.T) {
	fixture := newWorkerTrustClientFixture(t)
	for _, createdAt := range []time.Time{
		fixture.now.Add(-WorkerHandshakeMaxSkew - time.Second),
		fixture.now.Add(WorkerHandshakeMaxSkew + time.Second),
	} {
		request, err := BuildWorkerHandshakeChallengeRequest(fixture.config, WorkerHandshakeRequestOptions{
			RequestID: "request-skew", TraceID: "trace-skew", Purpose: issueWorkerHandshakePurpose(),
			ClientNonce: bytes.Repeat([]byte{0x6a}, WorkerHandshakeNonceSize), CreatedAt: createdAt,
		})
		if err != nil {
			t.Fatalf("build request at %v: %v", createdAt, err)
		}
		response := fixture.buildChallenge(t, request)
		if _, err := VerifyWorkerHandshakeChallenge(fixture.config, request, response, fixture.now); !errors.Is(err, ErrWorkerHandshakeExpired) {
			t.Fatalf("request created_at=%v error=%v, want ErrWorkerHandshakeExpired", createdAt, err)
		}
	}
}
