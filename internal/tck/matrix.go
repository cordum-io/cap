package tck

import (
	"fmt"
	"sort"
)

// Fixture is one producer SDK's output for one payload case: the signed wire
// bytes plus the producer's own claimed digests, key, and signature. Every
// other stable SDK must independently decode it to the same digests and verify
// the signature — that bidirectional agreement is what the matrix proves.
type Fixture struct {
	Producer         string
	Case             string
	Wire             []byte
	NormalizedDigest string // semantic canonical form, encoding-independent
	PreimageDigest   string // exact unsigned-for-signature bytes
	KeyID            string
	PublicKey        []byte // DER (PKIX)
	Signature        []byte
}

// Consumer decodes and validates a producer's fixture using only one SDK's
// public API. A real cross-language matrix wraps each stable SDK as a Consumer.
type Consumer interface {
	Name() string
	// Inspect decodes wire and returns the consumer's independently recomputed
	// normalized-semantic and unsigned-preimage digests.
	Inspect(wire []byte) (normalizedDigest, preimageDigest string, err error)
	// VerifySignature verifies the producer's signature over wire with publicKey.
	VerifySignature(wire, publicKey, signature []byte) error
}

// EdgeResult is the outcome of one producer->consumer pair over all its cases.
type EdgeResult struct {
	Producer string
	Consumer string
	Cases    int
	Failures []string
}

// OK reports whether every case on this edge passed.
func (e EdgeResult) OK() bool { return len(e.Failures) == 0 }

// RunMatrix runs every producer's fixtures through every consumer and returns
// one edge per (producer, consumer) pair in deterministic order.
func RunMatrix(fixtures []Fixture, consumers []Consumer) []EdgeResult {
	byProducer := map[string][]Fixture{}
	for _, f := range fixtures {
		byProducer[f.Producer] = append(byProducer[f.Producer], f)
	}
	var edges []EdgeResult
	for _, c := range consumers {
		for _, p := range sortedProducers(byProducer) {
			edges = append(edges, runEdge(p, byProducer[p], c))
		}
	}
	return edges
}

func runEdge(producer string, fixtures []Fixture, c Consumer) EdgeResult {
	e := EdgeResult{Producer: producer, Consumer: c.Name(), Cases: len(fixtures)}
	for _, f := range fixtures {
		nd, pd, err := c.Inspect(f.Wire)
		if err != nil {
			e.Failures = append(e.Failures, fmt.Sprintf("%s: decode: %v", f.Case, err))
			continue
		}
		if nd != f.NormalizedDigest {
			e.Failures = append(e.Failures, fmt.Sprintf("%s: normalized digest mismatch", f.Case))
		}
		if pd != f.PreimageDigest {
			e.Failures = append(e.Failures, fmt.Sprintf("%s: preimage digest mismatch", f.Case))
		}
		if err := c.VerifySignature(f.Wire, f.PublicKey, f.Signature); err != nil {
			e.Failures = append(e.Failures, fmt.Sprintf("%s: signature verify: %v", f.Case, err))
		}
	}
	return e
}

// MissingEdges computes the (producer, consumer) pairs absent from edges given
// the full expected sets, so matrix completeness is computed, never assumed. A
// gated cross-language edge shows up here rather than being silently omitted.
func MissingEdges(edges []EdgeResult, producers, consumers []string) []string {
	have := map[string]bool{}
	for _, e := range edges {
		have[e.Producer+"->"+e.Consumer] = true
	}
	var missing []string
	for _, p := range producers {
		for _, c := range consumers {
			if key := p + "->" + c; !have[key] {
				missing = append(missing, key)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

func sortedProducers(m map[string][]Fixture) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
