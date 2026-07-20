package releasetruth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// releaseSourceFile is a stable package's source metadata file and the regexp
// that captures its declared version.
type releaseSourceFile struct {
	file string
	re   *regexp.Regexp
}

var releaseSourceFiles = []releaseSourceFile{
	{"sdk/node/package.json", regexp.MustCompile(`"version"\s*:\s*"([^"]+)"`)},
	{"sdk/python/pyproject.toml", regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)},
	{"sdk/python-guard/pyproject.toml", regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)},
}

// readSourceVersion returns the declared version in a source metadata file.
func readSourceVersion(repoRoot string, sf releaseSourceFile) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(sf.file)))
	if err != nil {
		return "", err
	}
	m := sf.re.FindStringSubmatch(string(data))
	if m == nil {
		return "", fmt.Errorf("no version found in %s", sf.file)
	}
	return m[1], nil
}

// ReleaseCheck validates that a release tag is internally consistent with the
// manifest, the source metadata, and the changelog. It is the fail-closed gate
// a release-tag workflow runs: a release fails unless an approved release-prep
// commit already synchronized these. It performs local file I/O only.
func ReleaseCheck(m *Manifest, repoRoot, tag string) []Problem {
	var ps []Problem
	if tag != m.Release.Tag {
		ps = append(ps, Problem{"release.tag", fmt.Sprintf("tag %q does not equal manifest release tag %q", tag, m.Release.Tag)})
	}
	for _, sf := range releaseSourceFiles {
		v, err := readSourceVersion(repoRoot, sf)
		if err != nil {
			ps = append(ps, Problem{"release.source.read", err.Error()})
			continue
		}
		if v != m.Release.Version {
			ps = append(ps, Problem{"release.source.version",
				fmt.Sprintf("%s version %q != release %q (run release prep to sync source with the tag)", sf.file, v, m.Release.Version)})
		}
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "CHANGELOG.md"))
	if err != nil {
		ps = append(ps, Problem{"release.changelog.read", err.Error()})
	} else if !strings.Contains(string(data), "## "+m.Release.Tag+" ") && !strings.Contains(string(data), "## "+m.Release.Tag+"\n") {
		ps = append(ps, Problem{"release.changelog", "CHANGELOG.md has no '## " + m.Release.Tag + "' entry"})
	}
	return ps
}
