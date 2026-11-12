// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import (
	"errors"
	"maps"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
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
// packets. It is transport-free, never logs and holds no lock of its own.
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

// Admit is not implemented yet (RED stub).
func (a *Admitter) Admit(subject string, raw []byte) (capsdk.VerifiedProductionPacket, error) {
	return capsdk.VerifiedProductionPacket{}, ErrAdmitSubject
}
