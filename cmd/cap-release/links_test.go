package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cordum-io/cap/v2/internal/releasetruth"
)

func TestCheckAllLinksSkipsPythonEnvironments(t *testing.T) {
	for _, env := range []string{".venv", "venv", ".tox"} {
		for _, parent := range []string{"", "nested"} {
			t.Run(filepath.Join(parent, env), func(t *testing.T) {
				root := t.TempDir()
				for _, layout := range []string{
					filepath.Join("Lib", "site-packages"),
					filepath.Join("lib", "python3.11", "site-packages"),
				} {
					writeLinkFixture(t, root, filepath.Join(parent, env, layout, "pdoc", "README.md"))
				}
				if got, err := checkAllLinks(root); err != nil || len(got) != 0 {
					t.Fatalf("environment-only check = %v, %v; want no problems, nil", got, err)
				}
				if got := runLinks([]string{"--repo-root", root}); got != 0 {
					t.Fatalf("environment-only exit = %d; want 0", got)
				}

				writeLinkFixture(t, root, filepath.Join("docs", "broken.md"))
				got, err := checkAllLinks(root)
				want := releasetruth.LinkProblem{
					File: "docs/broken.md", Line: 1, Target: "missing.md", Reason: "path not found: docs/missing.md",
				}
				if err != nil || len(got) != 1 || got[0] != want {
					t.Fatalf("docs check = %v, %v; want [%v], nil", got, err, want)
				}
				if got := runLinks([]string{"--repo-root", root}); got != 1 {
					t.Fatalf("broken docs exit = %d; want 1", got)
				}
			})
		}
	}
}

func writeLinkFixture(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[broken](missing.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
