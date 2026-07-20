package tck

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// jobKind separates the fixtures that must verify from the ones that must not.
type jobKind int

const (
	jobPositive jobKind = iota
	jobTampered
	jobWrongKey
)

// buildJobs turns every produced fixture into one positive verification job
// plus two negatives: a single flipped wire byte and a wrong public key. Both
// negatives are sent to every consumer, so rejection is proven per language
// rather than assumed from the Go reference.
func (e *CrossLangEnv) buildJobs(fixtures []Fixture) ([]consumeJob, map[string]jobKind) {
	var jobs []consumeJob
	kinds := map[string]jobKind{}
	for _, f := range fixtures {
		publicKey := base64.StdEncoding.EncodeToString(f.PublicKey)
		add := func(suffix string, wire []byte, key string, kind jobKind) {
			id := f.Producer + "/" + f.Case + "/" + suffix
			jobs = append(jobs, consumeJob{
				ID: id, Wire: base64.StdEncoding.EncodeToString(wire),
				KeyID: f.KeyID, PublicKeyDER: key,
			})
			kinds[id] = kind
		}
		add("ok", f.Wire, publicKey, jobPositive)
		add("tampered", flipMiddleByte(f.Wire), publicKey, jobTampered)
		add("wrong-key", f.Wire, e.wrongKey.publicDERB64, jobWrongKey)
	}
	return jobs, kinds
}

// flipMiddleByte corrupts one byte inside the signed body so the signature
// stops covering the packet, without changing its length.
func flipMiddleByte(wire []byte) []byte {
	tampered := append([]byte(nil), wire...)
	if len(tampered) > 0 {
		tampered[len(tampered)/2] ^= 0xFF
	}
	return tampered
}

func (e *CrossLangEnv) consume(sdk string, jobs []consumeJob) (consumeResponse, error) {
	request := consumeRequest{
		SDK: sdk, Audience: e.corpus.Audience,
		TenantID: e.corpus.TenantID, SenderID: e.corpus.SenderID, Jobs: jobs,
	}
	var response consumeResponse
	if err := e.invoke(sdk, "consume", request, &response); err != nil {
		return consumeResponse{}, err
	}
	if len(response.Results) != len(jobs) {
		return consumeResponse{}, fmt.Errorf("consumer %s returned %d results for %d jobs",
			sdk, len(response.Results), len(jobs))
	}
	return response, nil
}

// collect indexes one consumer's verdicts by the exact bytes they were computed
// over, so RunMatrix can query them without respawning a process while a
// tampered wire or swapped key still misses the cache and fails.
func (e *CrossLangEnv) collect(sdk string, response consumeResponse, jobs []consumeJob,
	kinds map[string]jobKind) (Consumer, []NegativeResult, error) {
	byID := make(map[string]consumeJob, len(jobs))
	for _, job := range jobs {
		byID[job.ID] = job
	}
	consumer := &cachedConsumer{
		name: sdk, digests: map[string]digestPair{}, verified: map[string]error{},
	}
	var negatives []NegativeResult
	for _, result := range response.Results {
		job, known := byID[result.ID]
		if !known {
			return nil, nil, fmt.Errorf("consumer %s returned unknown job %q", sdk, result.ID)
		}
		wire, err := base64.StdEncoding.DecodeString(job.Wire)
		if err != nil {
			return nil, nil, err
		}
		publicKey, err := base64.StdEncoding.DecodeString(job.PublicKeyDER)
		if err != nil {
			return nil, nil, err
		}
		var verdict error
		if !result.OK {
			verdict = errors.New(result.Error)
		}
		consumer.verified[wireKey(wire, publicKey)] = verdict
		if kinds[result.ID] != jobPositive {
			negatives = append(negatives, NegativeResult{
				ID: result.ID, Consumer: sdk, OK: result.OK, Error: result.Error,
			})
			continue
		}
		consumer.digests[hashWire(wire)] = digestPair{
			normalized: result.NormalizedDigest,
			preimage:   result.PreimageDigest,
			err:        verdict,
		}
	}
	return consumer, negatives, nil
}

type digestPair struct {
	normalized string
	preimage   string
	err        error
}

// cachedConsumer answers Consumer queries from verdicts an out-of-process SDK
// driver already returned. Every lookup key is derived from the caller's exact
// bytes, so an unrecorded wire or key is an error rather than a silent pass.
type cachedConsumer struct {
	name     string
	digests  map[string]digestPair
	verified map[string]error
}

func (c *cachedConsumer) Name() string { return c.name }

func (c *cachedConsumer) Inspect(wire []byte) (string, string, error) {
	entry, ok := c.digests[hashWire(wire)]
	if !ok {
		return "", "", fmt.Errorf("%s: no recorded verdict for this wire", c.name)
	}
	if entry.err != nil {
		return "", "", entry.err
	}
	return entry.normalized, entry.preimage, nil
}

func (c *cachedConsumer) VerifySignature(wire, publicKey, _ []byte) error {
	err, ok := c.verified[wireKey(wire, publicKey)]
	if !ok {
		return fmt.Errorf("%s: no recorded verification for this wire and key", c.name)
	}
	return err
}

func hashWire(wire []byte) string {
	sum := sha256.Sum256(wire)
	return hex.EncodeToString(sum[:])
}
