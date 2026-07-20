package releasetruth

import "testing"

// TestSourceMetadata_NotStampedAsRelease guards the supply-chain reproducibility
// invariant: source is either a development marker or exactly the explicit
// future candidate, and is never mistaken for the already-published version.
func TestSourceMetadata_NotStampedAsRelease(t *testing.T) {
	m, root := loadGolden(t)
	if ps := CheckSourceMetadata(m, root); len(ps) != 0 {
		t.Fatalf("source metadata is inconsistent with manifest state: %v", ps)
	}
}
