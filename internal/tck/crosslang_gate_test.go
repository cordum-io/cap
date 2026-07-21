package tck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The cross-language matrix only runs when CAP_TCK_MATRIX is set, because it
// needs the Go, Python and Node toolchains. These tests always run: they keep
// an opt-in gate from decaying into an unenforced one, so the matrix cannot be
// removed from CI or quietly downgraded in the docs without a failing test.

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	if err != nil {
		t.Fatalf("read %v: %v", parts, err)
	}
	return string(body)
}

func TestCrossLanguageMatrixIsEnforcedInCI(t *testing.T) {
	workflow := readRepoFile(t, ".github", "workflows", "sdk-gates.yml")
	for _, want := range []string{
		"cross-language-matrix:",
		MatrixEnvVar + ": '1'",
		"-run TestCrossLanguage",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("sdk-gates.yml must contain %q so the matrix gate stays enforced", want)
		}
	}
}

// The corpus is the shared contract between three independently written
// drivers; an empty or single-case corpus would make every edge trivially green.
func TestCrossLanguageCorpusCoversProductionSurface(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	corpus, raw, err := loadCorpus(root)
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	if len(corpus.Cases) < 4 {
		t.Errorf("corpus declares %d cases, want >=4", len(corpus.Cases))
	}
	// DispatchIdentity is the CAP-PRODUCTION attempt-fencing surface, and the
	// structured ResourceRef surface (context_ref / result_ref / artifact_refs,
	// each with an operator-resolver id and a sha256) is the CAP-PRODUCTION
	// content-resolution surface; a corpus of bare heartbeats or one carrying no
	// resource refs would prove decoding but not production interop.
	for _, want := range []string{
		"dispatchId", "attempt", "assignedWorkerId", "identity",
		"contextRef", "resultRef", "artifactRefs", "resolverId", "sha256",
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("corpus must exercise %q", want)
		}
	}
	// Every case name must be distinct: the matrix keys producer output by case
	// name, so a duplicated name would let a count-preserving corpus hide a
	// dropped case.
	seen := map[string]bool{}
	for _, c := range corpus.Cases {
		if seen[c.Name] {
			t.Errorf("corpus case name %q is duplicated", c.Name)
		}
		seen[c.Name] = true
	}
}

func TestMatrixStatusDocumentsEveryEdge(t *testing.T) {
	status := readRepoFile(t, "spec", "tck", "MATRIX-STATUS.md")
	for _, producer := range StableSDKs {
		for _, consumer := range StableSDKs {
			edge := producer + " → " + consumer
			if !strings.Contains(status, edge) {
				t.Errorf("MATRIX-STATUS.md must document edge %q", edge)
			}
		}
	}
	if strings.Contains(status, "⏳ gated") {
		t.Error("MATRIX-STATUS.md still marks an edge gated; all nine edges are delivered")
	}
}
