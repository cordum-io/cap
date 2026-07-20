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
	// DispatchIdentity is the CAP-PRODUCTION attempt-fencing surface; a corpus
	// of bare heartbeats would prove decoding but not production interop.
	for _, want := range []string{"dispatchId", "attempt", "assignedWorkerId", "identity"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("corpus must exercise %q", want)
		}
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
