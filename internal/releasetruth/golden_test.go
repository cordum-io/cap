package releasetruth

import (
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot walks up from this test file (<root>/internal/releasetruth/*) to the
// module root so golden checks do not depend on the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate caller for repo root")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func loadGolden(t *testing.T) (*Manifest, string) {
	t.Helper()
	root := repoRoot(t)
	m, err := LoadFile(filepath.Join(root, "release", "manifest.json"))
	if err != nil {
		t.Fatalf("load golden manifest: %v", err)
	}
	return m, root
}

func TestGoldenManifest_IsValidAndOnDisk(t *testing.T) {
	m, root := loadGolden(t)
	if ps := Validate(m); len(ps) != 0 {
		t.Fatalf("golden manifest is invalid: %v", ps)
	}
	if ps := CheckSpecsOnDisk(m, root); len(ps) != 0 {
		t.Fatalf("golden manifest specs missing on disk: %v", ps)
	}
	if m.Release.Version != "2.16.1" {
		t.Errorf("golden release version = %q, want 2.16.1", m.Release.Version)
	}
	if m.Wire.ProtocolVersion != 1 {
		t.Errorf("golden wire protocol = %d, want 1", m.Wire.ProtocolVersion)
	}
}

// TestGoldenManifest_GuardTracksReleaseVersion pins the released cordum-guard
// artifact to the release train. The PyPI cordum-guard package publishes on the
// same version line as the SDKs (latest 2.16.1); source pyproject carries an
// in-development version that must not masquerade as the published artifact.
func TestGoldenManifest_GuardTracksReleaseVersion(t *testing.T) {
	m, _ := loadGolden(t)
	var guard *Component
	for i := range m.Components {
		if m.Components[i].Name == "cordum-guard" {
			guard = &m.Components[i]
		}
	}
	if guard == nil {
		t.Fatal("cordum-guard component not found in manifest")
	}
	if guard.Version != m.Release.Version {
		t.Errorf("cordum-guard published version = %q, want release version %q", guard.Version, m.Release.Version)
	}
}

// TestGoldenManifest_SpecCountMatchesDisk enforces that the manifest lists
// exactly the normative spec files on disk (spec/*.md excluding the index), so
// the count cannot silently drift when specs are added or removed.
func TestGoldenManifest_SpecCountMatchesDisk(t *testing.T) {
	m, root := loadGolden(t)
	matches, err := filepath.Glob(filepath.Join(root, "spec", "*.md"))
	if err != nil {
		t.Fatalf("glob specs: %v", err)
	}
	n := 0
	for _, p := range matches {
		if filepath.Base(p) == "00-index.md" {
			continue
		}
		n++
	}
	if len(m.Specs) != n {
		t.Fatalf("manifest lists %d specs, disk has %d normative specs", len(m.Specs), n)
	}
}
