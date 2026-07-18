package runtime

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
)

func (a *Agent) performHandshake(ctx context.Context) (bool, error) {
	return a.performTrustHandshake(ctx, agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_ISSUE, "")
}

func (a *Agent) performRenew(ctx context.Context) (bool, error) {
	token, _ := a.SessionToken()
	if token == "" {
		return false, errors.New("cap-runtime: renew requires a current unexpired session")
	}
	return a.performTrustHandshake(ctx, agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW, token)
}

func (a *Agent) performTrustHandshake(
	ctx context.Context,
	purpose agentv1.WorkerHandshakePurpose,
	currentToken string,
) (bool, error) {
	if err := a.validateTrustStartup(); err != nil {
		return false, err
	}
	mode := a.activeHandshakeMode()
	trust := a.workerTrustConfig()
	if mode == HandshakeModeOff {
		return false, nil
	}
	requester, ok := a.NATS.(NATSRequester)
	if !ok {
		return a.handleTrustFailure(mode, errors.New("NATS connection does not support request/reply"))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 0; attempt < a.handshakeRetries(); attempt++ {
		if err := ctx.Err(); err != nil {
			lastErr = err
			break
		}
		session, err := a.exchangeWorkerTrust(ctx, requester, purpose, currentToken)
		if err == nil {
			a.setSession(session.Token, session.ExpiresAt)
			a.Logger.Info("cap-runtime: authenticated worker session established",
				"worker_id", trust.WorkerID, "session_exp", session.ExpiresAt)
			return true, nil
		}
		lastErr = err
		var rejected *capsdk.WorkerHandshakeRejectionError
		if errors.As(err, &rejected) {
			lastErr = &HandshakeRejectedError{Reason: rejected.Reason, WorkerID: trust.WorkerID}
			break
		}
		if attempt+1 < a.handshakeRetries() && !sleepCtx(ctx, handshakeBackoff(attempt)) {
			lastErr = ctx.Err()
			break
		}
	}
	if purpose == agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW && mode == HandshakeModeEnforce {
		a.clearSession()
		a.unsubscribeAll()
	}
	return a.handleTrustFailure(mode, lastErr)
}

func (a *Agent) exchangeWorkerTrust(
	ctx context.Context,
	requester NATSRequester,
	purpose agentv1.WorkerHandshakePurpose,
	currentToken string,
) (*capsdk.WorkerHandshakeSession, error) {
	trust := a.workerTrustConfig()
	now := time.Now().UTC()
	options, err := newHandshakeRequestOptions(purpose, now)
	if err != nil {
		return nil, err
	}
	request, err := capsdk.BuildWorkerHandshakeChallengeRequest(trust, options)
	if err != nil {
		return nil, err
	}
	challenge, err := a.requestTrustPacket(ctx, requester, capsdk.WorkerHandshakeChallengeSubject, request)
	if err != nil {
		return nil, err
	}
	verified, err := capsdk.VerifyWorkerHandshakeChallenge(trust, request, challenge, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	authenticate, err := capsdk.BuildWorkerHandshakeAuthenticate(
		trust, verified, a.capabilityHandshake(), currentToken, time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	result, err := a.requestTrustPacket(ctx, requester, capsdk.WorkerHandshakeAuthenticateSubject, authenticate)
	if err != nil {
		return nil, err
	}
	return capsdk.VerifyWorkerHandshakeResult(
		trust, verified, authenticate, result, time.Now().UTC(),
	)
}

func (a *Agent) requestTrustPacket(
	ctx context.Context,
	requester NATSRequester,
	subject string,
	packet *agentv1.BusPacket,
) (*agentv1.BusPacket, error) {
	data, err := capsdk.MarshalWorkerTrustPacket(packet)
	if err != nil {
		return nil, err
	}
	message, err := a.requestWithContext(ctx, requester, subject, data)
	if err != nil {
		return nil, fmt.Errorf("cap-runtime: %s request failed: %w", subject, err)
	}
	if message == nil {
		return nil, fmt.Errorf("cap-runtime: %s returned a nil response", subject)
	}
	return capsdk.UnmarshalWorkerTrustPacket(message.Data)
}

func (a *Agent) requestWithContext(
	ctx context.Context,
	requester NATSRequester,
	subject string,
	data []byte,
) (*nats.Msg, error) {
	requestCtx, cancel := context.WithTimeout(ctx, a.handshakeTimeout())
	defer cancel()
	if contextual, ok := requester.(natsContextRequester); ok {
		return contextual.RequestWithContext(requestCtx, subject, data)
	}
	return requester.Request(subject, data, a.handshakeTimeout())
}

func (a *Agent) handleTrustFailure(mode HandshakeMode, err error) (bool, error) {
	if err == nil {
		err = errors.New("authenticated worker handshake failed")
	}
	if mode == HandshakeModeEnforce {
		return false, fmt.Errorf("cap-runtime: authenticated worker handshake failed: %w", err)
	}
	a.Logger.Warn("cap-runtime: authenticated worker handshake failed; continuing tokenless in warn mode",
		"worker_id", a.workerTrustConfig().WorkerID, "error", err)
	return false, nil
}

func newHandshakeRequestOptions(
	purpose agentv1.WorkerHandshakePurpose,
	now time.Time,
) (capsdk.WorkerHandshakeRequestOptions, error) {
	requestID, err := randomHex(16)
	if err != nil {
		return capsdk.WorkerHandshakeRequestOptions{}, err
	}
	traceID, err := randomHex(16)
	if err != nil {
		return capsdk.WorkerHandshakeRequestOptions{}, err
	}
	nonce := make([]byte, capsdk.WorkerHandshakeNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return capsdk.WorkerHandshakeRequestOptions{}, err
	}
	return capsdk.WorkerHandshakeRequestOptions{
		RequestID: requestID, TraceID: traceID, Purpose: purpose, ClientNonce: nonce, CreatedAt: now,
	}, nil
}
