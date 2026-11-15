// Copyright (c) 2026 The cordum Agent Protocol Authors
// SPDX-License-Identifier: Apache-2.0

package referencescheduler

import "errors"

var (
	// ErrInvalidConfig rejects absent clock, empty allowlist or nonpositive limits.
	ErrInvalidConfig = errors.New("referencescheduler: invalid config")
	// ErrUnverified rejects packets without SDK verification proof.
	ErrUnverified = errors.New("referencescheduler: packet not verified")
	// ErrWrongSubject rejects packets verified for another subject.
	ErrWrongSubject = errors.New("referencescheduler: wrong subject")
	// ErrWrongPayload rejects a payload incompatible with the operation.
	ErrWrongPayload = errors.New("referencescheduler: wrong payload")
	// ErrInvalidJob rejects invalid identifiers, missing context or tenant disagreement.
	ErrInvalidJob = errors.New("referencescheduler: invalid job")
	// ErrDenied accompanies DENIED on an allowlist miss.
	ErrDenied = errors.New("referencescheduler: denied")
	// ErrJobNotFound gives no existence oracle across tenants.
	ErrJobNotFound = errors.New("referencescheduler: job not found")
	// ErrJobConflict rejects a different request under an existing job key.
	ErrJobConflict = errors.New("referencescheduler: job conflict")
	// ErrInvalidTransition rejects a disallowed nonterminal transition.
	ErrInvalidTransition = errors.New("referencescheduler: invalid transition")
	// ErrTerminal preserves the first terminal winner.
	ErrTerminal = errors.New("referencescheduler: job is terminal")
	// ErrStaleAttempt rejects a different dispatch ID or attempt.
	ErrStaleAttempt = errors.New("referencescheduler: stale or future attempt")
	// ErrWrongWorker rejects a sender or assigned worker other than the active worker.
	ErrWrongWorker = errors.New("referencescheduler: wrong worker")
	// ErrInputTooLarge accompanies DENIED for an oversized declared input.
	ErrInputTooLarge = errors.New("referencescheduler: input too large")
	// ErrOutputTooLarge accompanies FAILED with RESOURCE_EXHAUSTED for oversized output.
	ErrOutputTooLarge = errors.New("referencescheduler: output too large")
	// ErrTooManyArtifacts rejects an entire append that would exceed MaxArtifacts.
	ErrTooManyArtifacts = errors.New("referencescheduler: too many artifacts")
)
