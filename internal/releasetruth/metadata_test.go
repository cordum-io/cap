package releasetruth

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSourceMetadata_NotStampedAsRelease guards the supply-chain reproducibility
// invariant: while the manifest marks the tree as unreleased development
// (development.released=false), each stable package's SOURCE version must be a
// development/prerelease marker distinct from the published release artifact, so
// changed source is never mistaken for the already-published version.
func TestSourceMetadata_NotStampedAsRelease(t *testing.T) {
	m, root := loadGolden(t)
	if m.Development.Released {
		t.Skip("development.released=true: source==release is expected only at release prep")
	}
	cases := []struct {
		file string
		re   *regexp.Regexp
	}{
		{"sdk/node/package.json", regexp.MustCompile(`"version"\s*:\s*"([^"]+)"`)},
		{"sdk/python/pyproject.toml", regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)},
		{"sdk/python-guard/pyproject.toml", regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)},
	}
	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
		if err != nil {
			t.Errorf("read %s: %v", c.file, err)
			continue
		}
		match := c.re.FindStringSubmatch(string(data))
		if match == nil {
			t.Errorf("%s: no version found", c.file)
			continue
		}
		v := match[1]
		if v == m.Release.Version {
			t.Errorf("%s version %q equals the published release %q; development source must not be stamped as the released artifact",
				c.file, v, m.Release.Version)
		}
		if !strings.Contains(v, "dev") && !strings.Contains(v, "-") {
			t.Errorf("%s version %q is not a development/prerelease marker while development.released=false", c.file, v)
		}
	}
}
