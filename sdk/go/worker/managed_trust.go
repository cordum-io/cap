package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

type managedTrustState struct {
	mode    capsdk.WorkerTrustMode
	config  *capsdk.WorkerTrustConfig
	timeout time.Duration

	mu      sync.RWMutex
	session capsdk.WorkerHandshakeSession
	cancel  context.CancelFunc
	done    chan struct{}
	fatal   chan error
}

const managedRenewFailureFloor = 30 * time.Second

func newManagedTrustState(config ManagedConfig) *managedTrustState {
	state := &managedTrustState{mode: config.WorkerTrustMode}
	if config.WorkerTrustMode == capsdk.WorkerTrustModeOff {
		return state
	}
	state.config = config.WorkerTrust
	state.timeout = managedTrustTimeout(config)
	state.fatal = make(chan error, 1)
	return state
}

func (w *ManagedWorker) performInitialTrust(ctx context.Context) error {
	if w.trust == nil || w.trust.mode == capsdk.WorkerTrustModeOff {
		return nil
	}
	session, err := w.performTrustExchange(ctx, issueHandshakePurpose(), "")
	if err != nil {
		if w.trust.mode == capsdk.WorkerTrustModeWarn && isManagedTrustOperationalError(err) {
			w.logger.Printf("worker: WARN worker trust failed; continuing tokenless: %v", err)
			return nil
		}
		return fmt.Errorf("worker trust: %w", err)
	}
	w.trust.setSession(session)
	return nil
}

func (w *ManagedWorker) performTrustExchange(ctx context.Context, purpose agentv1.WorkerHandshakePurpose, token string) (*capsdk.WorkerHandshakeSession, error) {
	options, err := newWorkerHandshakeOptions(purpose)
	if err != nil {
		return nil, err
	}
	request, err := capsdk.BuildWorkerHandshakeChallengeRequest(w.trust.config, options)
	if err != nil {
		return nil, err
	}
	challenge, err := w.requestTrustPacket(ctx, capsdk.WorkerHandshakeChallengeSubject, request)
	if err != nil {
		return nil, err
	}
	verified, err := capsdk.VerifyWorkerHandshakeChallenge(w.trust.config, request, challenge, time.Now())
	if err != nil {
		return nil, err
	}
	authenticate, err := capsdk.BuildWorkerHandshakeAuthenticate(w.trust.config, verified, w.capability, token, time.Now())
	if err != nil {
		return nil, err
	}
	result, err := w.requestTrustPacket(ctx, capsdk.WorkerHandshakeAuthenticateSubject, authenticate)
	if err != nil {
		return nil, err
	}
	return capsdk.VerifyWorkerHandshakeResult(w.trust.config, verified, authenticate, result, time.Now())
}

func (w *ManagedWorker) requestTrustPacket(ctx context.Context, subject string, packet *agentv1.BusPacket) (*agentv1.BusPacket, error) {
	data, err := capsdk.MarshalWorkerTrustPacket(packet)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, w.trust.timeout)
	defer cancel()
	message, err := w.conn.RequestWithContext(requestCtx, subject, data)
	if err != nil {
		return nil, newManagedTrustOperationalError(fmt.Errorf("request %s: %w", subject, err))
	}
	if message == nil {
		return nil, newManagedTrustOperationalError(fmt.Errorf("request %s: empty response", subject))
	}
	return capsdk.UnmarshalWorkerTrustPacket(message.Data)
}

func newWorkerHandshakeOptions(purpose agentv1.WorkerHandshakePurpose) (capsdk.WorkerHandshakeRequestOptions, error) {
	nonce := make([]byte, capsdk.WorkerHandshakeNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return capsdk.WorkerHandshakeRequestOptions{}, fmt.Errorf("generate handshake nonce: %w", err)
	}
	requestID, err := randomWorkerTrustID()
	if err != nil {
		return capsdk.WorkerHandshakeRequestOptions{}, err
	}
	traceID, err := randomWorkerTrustID()
	if err != nil {
		return capsdk.WorkerHandshakeRequestOptions{}, err
	}
	return capsdk.WorkerHandshakeRequestOptions{
		RequestID: requestID, TraceID: traceID, Purpose: purpose,
		ClientNonce: nonce, CreatedAt: time.Now(),
	}, nil
}

func randomWorkerTrustID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate handshake id: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func (w *ManagedWorker) startTrustRenewal(ctx context.Context) {
	if w.trust == nil || w.trust.mode == capsdk.WorkerTrustModeOff || w.trust.token(time.Now()) == "" {
		return
	}
	renewCtx, cancel := context.WithCancel(ctx)
	w.trust.mu.Lock()
	w.trust.cancel = cancel
	w.trust.done = make(chan struct{})
	done := w.trust.done
	w.trust.mu.Unlock()
	go func() {
		defer close(done)
		w.renewTrustLoop(renewCtx)
	}()
}

func (w *ManagedWorker) renewTrustLoop(ctx context.Context) {
	for {
		session := w.trust.snapshot()
		if session.Token == "" || w.trust.token(time.Now()) == "" {
			return
		}
		if !waitManagedTrust(ctx, managedRenewDelay(session, time.Now())) {
			return
		}
		currentToken := w.trust.token(time.Now())
		if currentToken == "" {
			return
		}
		if currentToken != session.Token {
			continue
		}
		replacement, err := w.performTrustExchange(ctx, renewHandshakePurpose(), currentToken)
		if err == nil {
			w.trust.setSession(replacement)
			continue
		}
		w.logger.Printf("worker: WARN session renewal failed: %v", err)
		if w.trust.mode == capsdk.WorkerTrustModeEnforce || !isManagedTrustOperationalError(err) {
			w.trust.clearSession()
			w.trust.signalFatal(fmt.Errorf("worker trust: session renewal failed: %w", err))
			return
		}
		if w.trust.token(time.Now()) != "" {
			if !waitManagedTrust(ctx, managedRenewFailureDelay(session, time.Now())) {
				return
			}
			continue
		}
		return
	}
}

func managedRenewFailureDelay(session capsdk.WorkerHandshakeSession, now time.Time) time.Duration {
	remaining := session.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	if remaining < managedRenewFailureFloor {
		return remaining
	}
	return managedRenewFailureFloor
}

func managedRenewDelay(session capsdk.WorkerHandshakeSession, now time.Time) time.Duration {
	remaining := session.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	delay := remaining / 2
	if delay < 10*time.Millisecond {
		return 10 * time.Millisecond
	}
	return delay
}

func waitManagedTrust(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (w *ManagedWorker) stopTrustRenewal() {
	if w.trust == nil {
		return
	}
	w.trust.mu.Lock()
	cancel, done := w.trust.cancel, w.trust.done
	w.trust.cancel, w.trust.done = nil, nil
	w.trust.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (w *ManagedWorker) trustFailure() <-chan error {
	if w.trust == nil {
		return nil
	}
	return w.trust.fatal
}

func (s *managedTrustState) setSession(session *capsdk.WorkerHandshakeSession) {
	if session == nil {
		return
	}
	s.mu.Lock()
	s.session = *session
	s.mu.Unlock()
}

func (s *managedTrustState) snapshot() capsdk.WorkerHandshakeSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session
}

func (s *managedTrustState) token(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session.Token == "" || !now.Before(s.session.ExpiresAt) {
		s.session = capsdk.WorkerHandshakeSession{}
		return ""
	}
	return s.session.Token
}

func (s *managedTrustState) clearSession() {
	s.mu.Lock()
	s.session = capsdk.WorkerHandshakeSession{}
	s.mu.Unlock()
}

func (s *managedTrustState) signalFatal(err error) {
	if err == nil || s.fatal == nil {
		return
	}
	select {
	case s.fatal <- err:
	default:
	}
}

func issueHandshakePurpose() agentv1.WorkerHandshakePurpose {
	return agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_ISSUE
}

func renewHandshakePurpose() agentv1.WorkerHandshakePurpose {
	return agentv1.WorkerHandshakePurpose_WORKER_HANDSHAKE_PURPOSE_RENEW
}
