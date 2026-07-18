package releasetruth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeRelPath_Accepts(t *testing.T) {
	for _, p := range []string{"spec/01-overview.md", "README.md", "docs/a/b.md"} {
		if err := safeRelPath(p); err != nil {
			t.Errorf("safeRelPath(%q) = %v, want nil", p, err)
		}
	}
}

func TestSafeRelPath_Rejects(t *testing.T) {
	for _, p := range []string{"", "/abs/path", "../escape", "a/../b", "a//b", `..\win`, `dir\file`} {
		if err := safeRelPath(p); err == nil {
			t.Errorf("safeRelPath(%q) = nil, want error", p)
		}
	}
}

func TestCheckSpecsOnDisk_FlagsMissingOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "spec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spec", "01-overview.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := validManifest()
	m.Specs = []Spec{
		{ID: "01", File: "spec/01-overview.md", Title: "Overview"},
		{ID: "99", File: "spec/99-missing.md", Title: "Missing"},
	}
	ps := CheckSpecsOnDisk(m, root)
	if !hasField(ps, "specs.file.missing") {
		t.Fatalf("expected specs.file.missing for absent spec, got %v", ps)
	}
	for _, p := range ps {
		if p.Field == "specs.file.missing" && p.Msg == "spec/01-overview.md" {
			t.Errorf("present spec wrongly reported missing: %v", p)
		}
	}
}

func TestCheckSpecsOnDisk_RejectsTraversal(t *testing.T) {
	m := validManifest()
	m.Specs = []Spec{{ID: "x", File: "../outside.md", Title: "Escape"}}
	ps := CheckSpecsOnDisk(m, t.TempDir())
	if !hasField(ps, "specs.file") {
		t.Fatalf("expected specs.file for traversal, got %v", ps)
	}
}
