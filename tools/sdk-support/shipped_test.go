package sdksupport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadShipped(t *testing.T) *Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "sdk", "support-tiers.json"))
	if err != nil {
		t.Fatalf("read shipped manifest: %v", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse shipped manifest: %v", err)
	}
	return &m
}

// The shipped manifest must verify against the real repository tree: every
// declared gate, owner path, and doc must resolve. This is the live evidence
// anchor for CI.
func TestShippedManifestVerifiesAgainstTree(t *testing.T) {
	m := loadShipped(t)
	root := os.DirFS(filepath.Join("..", ".."))
	if problems := Verify(m, root); len(problems) != 0 {
		t.Fatalf("shipped support-tiers.json does not verify: %+v", problems)
	}
}

func entryByID(m *Manifest, id string) (Entry, bool) {
	for _, e := range m.Entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Go must be fully earned; Python and Node are stable but must honestly surface
// pending gates rather than presenting unearned evidence as satisfied.
func TestShippedTiersReflectHonestMaturity(t *testing.T) {
	m := loadShipped(t)

	goEntry, ok := entryByID(m, "go")
	if !ok || goEntry.Tier != TierStable || !goEntry.FullyEarned() {
		t.Fatalf("go must be fully-earned stable, got %+v", goEntry)
	}
	for _, id := range []string{"python", "node"} {
		e, ok := entryByID(m, id)
		if !ok || e.Tier != TierStable {
			t.Fatalf("%s must be stable, got %+v", id, e)
		}
		pg := e.PendingGates()
		if len(pg) == 0 {
			t.Fatalf("%s must declare its pending gates honestly at this baseline", id)
		}
		for _, g := range pg {
			if g.BlockedBy == "" {
				t.Fatalf("%s pending gate %q must name a blocking task", id, g.ID)
			}
		}
	}

	guard, ok := entryByID(m, "python-guard")
	if !ok || guard.Kind != KindExtension {
		t.Fatalf("python-guard must be an extension (never a wire SDK), got %+v", guard)
	}
	cpp, ok := entryByID(m, "cpp")
	if !ok || cpp.Tier != TierCommunity {
		t.Fatalf("cpp must be community, got %+v", cpp)
	}
	for _, id := range []string{"java", "dotnet", "rust", "php", "ruby"} {
		e, ok := entryByID(m, id)
		if !ok || e.Tier != TierExperimental {
			t.Fatalf("%s must be experimental, got %+v", id, e)
		}
	}
}

// A broken gate path must fail verification: the manifest is only as good as
// the evidence it can resolve.
func TestShippedManifestFailsWhenEvidenceGoesMissing(t *testing.T) {
	m := loadShipped(t)
	for i := range m.Entries {
		if m.Entries[i].ID == "go" {
			m.Entries[i].Gates[0].Path = ".github/workflows/does-not-exist.yml"
		}
	}
	root := os.DirFS(filepath.Join("..", ".."))
	if problems := Verify(m, root); !hasCode(problems, CodeGatePathMissing) {
		t.Fatalf("expected gate-path-missing when evidence is removed, got %+v", problems)
	}
}

// Renaming or deleting an owner path must fail verification: ownership is
// resolved evidence, not a free-text claim.
func TestShippedManifestFailsWhenOwnerPathMissing(t *testing.T) {
	m := loadShipped(t)
	for i := range m.Entries {
		if m.Entries[i].ID == "cpp" {
			m.Entries[i].OwnerPath = "sdk/renamed-away/"
		}
	}
	root := os.DirFS(filepath.Join("..", ".."))
	if problems := Verify(m, root); !hasCode(problems, CodeOwnerPathMissing) {
		t.Fatalf("expected owner-path-missing, got %+v", problems)
	}
}
