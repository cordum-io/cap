package releasetruth

import (
	"os"
	"path/filepath"
	"testing"
)

// writeReleaseTree writes a temp repo whose source metadata and changelog are all
// synced to the given release version/tag.
func writeReleaseTree(t *testing.T, version, tag string) string {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, content string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("sdk/node/package.json", `{"name":"cap-sdk-node","version":"`+version+`"}`)
	mk("sdk/python/pyproject.toml", "[project]\nname = \"cap-sdk-python\"\nversion = \""+version+"\"\n")
	mk("sdk/python-guard/pyproject.toml", "[project]\nname = \"cordum-guard\"\nversion = \""+version+"\"\n")
	mk("CHANGELOG.md", "# Changelog\n\n## "+tag+" — 2026-06-02\n\n- release\n")
	return root
}

func releaseManifest(version, tag string) *Manifest {
	m := validManifest()
	m.Release.Version = version
	m.Release.Tag = tag
	return m
}

func TestReleaseCheck_PassesWhenSynced(t *testing.T) {
	m := releaseManifest("2.14.0", "v2.14.0")
	root := writeReleaseTree(t, "2.14.0", "v2.14.0")
	if ps := ReleaseCheck(m, root, "v2.14.0"); len(ps) != 0 {
		t.Fatalf("ReleaseCheck(synced) = %v, want none", ps)
	}
}

func TestReleaseCheck_Rules(t *testing.T) {
	cases := []struct {
		name   string
		tag    string
		mutate func(root string)
		field  string
	}{
		{"tag mismatch", "v2.13.0", nil, "release.tag"},
		{"development source not release", "v2.14.0", func(root string) {
			_ = os.WriteFile(filepath.Join(root, "sdk", "node", "package.json"), []byte(`{"version":"2.15.0-dev.0"}`), 0o644)
		}, "release.source.version"},
		{"changelog missing entry", "v2.14.0", func(root string) {
			_ = os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\n## v2.13.0 — x\n"), 0o644)
		}, "release.changelog"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := releaseManifest("2.14.0", "v2.14.0")
			root := writeReleaseTree(t, "2.14.0", "v2.14.0")
			if tc.mutate != nil {
				tc.mutate(root)
			}
			ps := ReleaseCheck(m, root, tc.tag)
			if !hasField(ps, tc.field) {
				t.Fatalf("%s: want problem field %q, got %v", tc.name, tc.field, ps)
			}
		})
	}
}
