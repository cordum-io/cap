package releasetruth

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadmeSnippets_ArePublicAndExtractable proves each SDK registry README
// carries an extractable consumer quickstart snippet that imports only public,
// installed packages — no repository-relative or internal imports — so a
// consumer can run it from an empty directory against the released artifact.
func TestReadmeSnippets_ArePublicAndExtractable(t *testing.T) {
	_, root := loadGolden(t)
	cases := []struct{ file, id, lang string }{
		{"sdk/go/README.md", "go-echo", "go"},
		{"sdk/node/README.md", "node-echo", "node"},
		{"sdk/python/README.md", "python-echo", "python"},
	}
	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
		if err != nil {
			t.Errorf("read %s: %v", c.file, err)
			continue
		}
		snips, err := ExtractSnippets(string(data))
		if err != nil {
			t.Errorf("%s: extract snippets: %v", c.file, err)
			continue
		}
		var found *Snippet
		for i := range snips {
			if snips[i].ID == c.id {
				found = &snips[i]
			}
		}
		if found == nil {
			t.Errorf("%s: quickstart snippet %q not found (got %d snippets)", c.file, c.id, len(snips))
			continue
		}
		if found.Language != c.lang {
			t.Errorf("%s: snippet %q language = %q, want %q", c.file, c.id, found.Language, c.lang)
		}
		if problems := CheckPublicImports(*found); len(problems) != 0 {
			t.Errorf("%s: snippet %q has non-public imports: %+v", c.file, c.id, problems)
		}
	}
}
