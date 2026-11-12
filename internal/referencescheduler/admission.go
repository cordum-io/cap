// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"errors"
	"maps"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

// Admission sentinels are bare: none wraps packet bytes, identities or keys.
var (
	// ErrAdmitSubject rejects a subject that has no inbound admission role.
	ErrAdmitSubject = errors.New("referencescheduler: subject not admitted")
	// ErrAdmitTooLarge rejects a packet over MaxPacketBytes before any decoding.
	ErrAdmitTooLarge = errors.New("referencescheduler: packet too large")
	// ErrAdmitMalformed rejects bytes that do not decode as a BusPacket.
	ErrAdmitMalformed = errors.New("referencescheduler: malformed packet")
	// ErrAdmitSender rejects an unknown client or a worker without a live session.
	ErrAdmitSender = errors.New("referencescheduler: sender not admitted")
	// ErrAdmitSignature rejects a packet that fails CAP-PRODUCTION verification.
	ErrAdmitSignature = errors.New("referencescheduler: production verification failed")
	// ErrAdmitDuplicate reports an exact redelivery; the caller drops it as a no-op.
	ErrAdmitDuplicate = errors.New("referencescheduler: duplicate packet")
	// ErrAdmitReplay rejects a replay conflict or a failing replay store (fail closed).
	ErrAdmitReplay = errors.New("referencescheduler: replay admission failed")
)

// Admitter runs CAP-PRODUCTION admission for inbound client and worker
// packets. It is transport-free and never logs; its fields are read-only after
// NewAdmitter, so Admit is safe for concurrent use when the replay store and
// the clock are.
type Admitter struct {
	maxBytes int
	clients  map[string]ResolvedClient
	workers  map[string]ResolvedWorker
	issuer   *Issuer
	replay   capsdk.ReplayStore
	now      func() time.Time
}

// NewAdmitter snapshots the client and worker identities of cfg. Any nil
// input returns ErrInvalidConfig.
func NewAdmitter(cfg *ResolvedConfig, issuer *Issuer, replay capsdk.ReplayStore, now func() time.Time) (*Admitter, error) {
	if cfg == nil || issuer == nil || replay == nil || now == nil {
		return nil, ErrInvalidConfig
	}
	a := &Admitter{maxBytes: cfg.MaxPacketBytes, clients: make(map[string]ResolvedClient, len(cfg.Clients)),
		workers: make(map[string]ResolvedWorker, len(cfg.Workers)), issuer: issuer, replay: replay, now: now}
	for id, c := range cfg.Clients {
		c.SigningKeys = maps.Clone(c.SigningKeys)
		a.clients[id] = c
	}
	for id, w := range cfg.Workers {
		w.SigningKeys = maps.Clone(w.SigningKeys)
		a.workers[id] = w
	}
	return a, nil
}

type admitRole uint8

const (
	admitClient admitRole = iota + 1
	admitWorker
)

func admitRoleOf(subject string) (admitRole, bool) {
	switch subject {
	case SubjectSubmit, SubjectClientCancel:
		return admitClient, true
	case SubjectProgress, SubjectResult:
		return admitWorker, true
	}
	return 0, false
}

// Admit admits one inbound packet received on subject. In order it checks the
// subject's role, the size bound, an unverified peek at sender_id and
// auth_token only, the sender against local config (a worker also needs a live
// issuer session, which supplies its tenant), CAP-PRODUCTION verification with
// the subject as audience, and finally fail-closed replay admission. Every
// failure returns an unverified zero value and one bare ErrAdmit* sentinel;
// ErrAdmitDuplicate marks an exact redelivery that the caller drops as a no-op.
func (a *Admitter) Admit(subject string, raw []byte) (capsdk.VerifiedProductionPacket, error) {
	var none capsdk.VerifiedProductionPacket
	role, ok := admitRoleOf(subject)
	if !ok {
		return none, ErrAdmitSubject
	}
	if len(raw) > a.maxBytes {
		return none, ErrAdmitTooLarge
	}
	var peek agentv1.BusPacket
	if proto.Unmarshal(raw, &peek) != nil {
		return none, ErrAdmitMalformed
	}
	sender, token := peek.GetSenderId(), peek.GetAuthToken()
	trust, ok := a.trustFor(role, subject, sender, token)
	if !ok {
		return none, ErrAdmitSender
	}
	v, err := capsdk.VerifyTrustedProductionPacket(raw, trust)
	if err != nil {
		return none, ErrAdmitSignature
	}
	if role == admitWorker && v.SessionToken() != token {
		return none, ErrAdmitSender
	}
	return a.admitReplay(subject, v)
}

// trustFor builds the trust input locally: the audience is the actual subject,
// and tenant and keys come from config or, for a worker, its live session.
func (a *Admitter) trustFor(role admitRole, subject, sender, token string) (capsdk.ProductionTrustStore, bool) {
	trust := capsdk.ProductionTrustStore{Audience: subject, Sender: sender, Now: a.now}
	if role == admitClient {
		c, ok := a.clients[sender]
		trust.Tenant, trust.PublicKeys = c.TenantID, c.SigningKeys
		return trust, ok
	}
	w, ok := a.workers[sender]
	if !ok {
		return trust, false
	}
	s, live := a.issuer.Session(sender, token)
	trust.Tenant, trust.PublicKeys = s.TenantID, w.SigningKeys
	return trust, live
}

// admitReplay consults the replay store only after verification. Any error, a
// conflict included, or an unknown outcome fails closed.
func (a *Admitter) admitReplay(subject string, v capsdk.VerifiedProductionPacket) (capsdk.VerifiedProductionPacket, error) {
	digest := v.SignedBodyDigest()
	expires := v.Packet().GetSignatureMetadata().GetExpiresAt().AsTime()
	out, err := a.replay.Admit(v.Tenant(), subject, v.Sender(), v.MessageID(), digest[:], expires)
	switch {
	case err != nil:
		return capsdk.VerifiedProductionPacket{}, ErrAdmitReplay
	case out == capsdk.ReplayOutcomeDuplicate:
		return capsdk.VerifiedProductionPacket{}, ErrAdmitDuplicate
	case out != capsdk.ReplayOutcomeFirst:
		return capsdk.VerifiedProductionPacket{}, ErrAdmitReplay
	}
	return v, nil
}
