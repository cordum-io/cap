// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrRuntimeCapacity  = errors.New("referencescheduler: lifetime job limit")
	ErrRuntimeTransport = errors.New("referencescheduler: transport unavailable")
	ErrRuntimePublish   = errors.New("referencescheduler: publish failed")
)

func dial(cfg *ResolvedConfig) (*nats.Conn, error) {
	opts := []nats.Option{nats.Name("cap-reference-scheduler"), nats.Timeout(2 * time.Second), nats.MaxReconnects(3), nats.ReconnectWait(time.Second), nats.IgnoreDiscoveredServers()}
	if cfg.TLS != nil {
		opts = append(opts, nats.Secure(cfg.TLS))
	}
	return nats.Connect(cfg.NATSURL, opts...)
}

func (r *Runtime) guarded(ctx context.Context, subject string) nats.MsgHandler {
	return func(m *nats.Msg) {
		r.gate.Lock()
		if r.closing || ctx.Err() != nil {
			r.gate.Unlock()
			return
		}
		r.wg.Add(1)
		r.goroutines.Add(1)
		r.gate.Unlock()
		defer r.wg.Done()
		defer r.goroutines.Add(-1)
		select {
		case r.sem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		defer func() { <-r.sem }()
		if ctx.Err() != nil {
			return
		}
		r.handle(subject, m)
	}
}

func (r *Runtime) handle(subject string, m *nats.Msg) {
	var reply []byte
	var err error
	switch subject {
	case capsdk.WorkerHandshakeChallengeSubject:
		reply, err = r.issuer.HandleChallenge(m.Data)
	case capsdk.WorkerHandshakeAuthenticateSubject:
		reply, err = r.issuer.HandleAuthenticate(m.Data)
	default:
		r.packet(subject, m.Data)
		return
	}
	if err != nil {
		r.drop(subject, err)
		return
	}
	if err = m.Respond(reply); err != nil {
		r.drop(subject, ErrRuntimePublish)
	}
}

func (r *Runtime) publishSigned(subject string, id *agentv1.IdentityBinding, set func(*agentv1.BusPacket)) error {
	if id == nil {
		return ErrRuntimePublish
	}
	message := make([]byte, 16)
	if _, err := rand.Read(message); err != nil {
		return ErrRuntimePublish
	}
	c := r.opts.Config
	p := &agentv1.BusPacket{TraceId: hex.EncodeToString(message), SenderId: c.SchedulerID, ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt: timestamppb.Now(), Identity: clone(id), SignatureMetadata: &agentv1.SignatureMetadata{ProfileVersion: capsdk.ProductionProfileVersion,
			Algorithm: capsdk.ProductionAlgorithm, MessageId: message, Audience: subject, KeyId: c.SigningKeyID, ExpiresAt: timestamppb.New(time.Now().Add(2 * time.Minute))}}
	set(p)
	raw, err := capsdk.SignProductionPacket(p, c.SigningKey)
	if err != nil {
		return ErrRuntimePublish
	}
	if err = r.nc.Publish(subject, raw); err != nil {
		return ErrRuntimePublish
	}
	r.published.Add(1)
	return nil
}

func (r *Runtime) drain() {
	r.gate.Lock()
	r.closing = true
	r.gate.Unlock()
	// Teardown errors cannot retain transport: always close and join, even after disconnect.
	for _, sub := range r.subs {
		_ = sub.Unsubscribe()
	}
	_ = r.nc.FlushTimeout(time.Second)
	r.nc.Close()
	r.wg.Wait()
}

func (r *Runtime) drop(subject string, err error) {
	r.opts.Logger.Warn("refsched: packet dropped", "subject", subject, "reason", err.Error())
	r.dropped.Add(1)
}
