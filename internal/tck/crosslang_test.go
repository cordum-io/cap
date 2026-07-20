package tck

import (
	"strings"
	"testing"
)

// The cross-language matrix is the DoD-2 gate: every stable SDK must PRODUCE
// fixtures that every other stable SDK independently DECODES, VALIDATES, and
// VERIFIES. These tests drive real per-SDK CLI drivers built from installed
// artifacts (local Go module proxy, Python wheel venv, npm-packed tarball), so
// a green run is evidence about shipped packages rather than repo source.

func TestCrossLanguageMatrixCoversAllNineEdges(t *testing.T) {
	env := NewCrossLangEnv(t)
	fixtures, consumers := env.Run(t)

	edges := RunMatrix(fixtures, consumers)
	missing := MissingEdges(edges, StableSDKs, StableSDKs)
	if len(missing) != 0 {
		t.Fatalf("expected zero missing producer->consumer edges, got %d: %v", len(missing), missing)
	}
	if len(edges) != 9 {
		t.Fatalf("expected 9 edges for 3 stable SDKs, got %d", len(edges))
	}
	totalCases := 0
	for _, e := range edges {
		if !e.OK() {
			t.Errorf("edge %s->%s failed: %v", e.Producer, e.Consumer, e.Failures)
		}
		if e.Cases < 3 {
			t.Errorf("edge %s->%s carried only %d cases, want >=3", e.Producer, e.Consumer, e.Cases)
		}
		totalCases += e.Cases
		// Emit each edge so a passing run leaves auditable evidence of which
		// edges actually executed, rather than only an aggregate count.
		t.Logf("edge %-6s -> %-6s ok=%v cases=%d", e.Producer, e.Consumer, e.OK(), e.Cases)
	}
	t.Logf("matrix complete: %d edges, %d producer->consumer case verifications, 0 missing",
		len(edges), totalCases)
}

// Every producer must be represented; a driver that silently emits nothing
// would otherwise leave its outgoing edges vacuously green.
func TestCrossLanguageEveryStableSDKProduces(t *testing.T) {
	env := NewCrossLangEnv(t)
	fixtures, _ := env.Run(t)

	counts := map[string]int{}
	for _, f := range fixtures {
		counts[f.Producer]++
	}
	for _, sdk := range StableSDKs {
		if counts[sdk] < 3 {
			t.Errorf("producer %q emitted %d fixtures, want >=3", sdk, counts[sdk])
		}
	}
}

// Bit flips and wrong keys must be rejected by every consumer, in every
// language — otherwise the matrix proves decoding, not verification.
func TestCrossLanguageConsumersRejectTamperedAndWrongKey(t *testing.T) {
	env := NewCrossLangEnv(t)
	_, _ = env.Run(t)

	negatives := env.NegativeResults()
	if len(negatives) == 0 {
		t.Fatal("expected negative verification jobs, got none")
	}
	seen := map[string]int{}
	for _, n := range negatives {
		if n.OK {
			t.Errorf("negative job %q was accepted by %s, want rejection", n.ID, n.Consumer)
		}
		if strings.TrimSpace(n.Error) == "" {
			t.Errorf("negative job %q rejected by %s without an error message", n.ID, n.Consumer)
		}
		seen[n.Consumer]++
	}
	for _, sdk := range StableSDKs {
		if seen[sdk] == 0 {
			t.Errorf("consumer %q ran no negative jobs", sdk)
		}
	}
}
