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

// CheckSourceMetadata verifies that source packages are either marked as
// unreleased development or exactly stamped to an explicit future candidate.
// Source must never masquerade as the latest published release.
func CheckSourceMetadata(m *Manifest, repoRoot string) []Problem {
	var ps []Problem
	for _, sf := range releaseSourceFiles {
		v, err := readSourceVersion(repoRoot, sf)
		if err != nil {
			ps = append(ps, Problem{"development.source.read", err.Error()})
			continue
		}
		if v == m.Release.Version {
			ps = append(ps, Problem{"development.source.version",
				fmt.Sprintf("%s version %q equals published release %q", sf.file, v, m.Release.Version)})
			continue
		}
		if m.Candidate != nil {
			if v != m.Candidate.Version {
				ps = append(ps, Problem{"candidate.source.version",
					fmt.Sprintf("%s version %q != candidate %q", sf.file, v, m.Candidate.Version)})
			}
			continue
		}
		if !isDevelopmentVersion(v) {
			ps = append(ps, Problem{"development.source.version",
				fmt.Sprintf("%s version %q is not a development/prerelease marker", sf.file, v)})
		}
	}
	return ps
}

func isDevelopmentVersion(version string) bool {
	return strings.Contains(strings.ToLower(version), "dev") || strings.Contains(version, "-")
}

// ReleaseCheck validates that a release tag is internally consistent with the
// manifest, the source metadata, and the changelog. It is the fail-closed gate
// a release-tag workflow runs: a release fails unless an approved release-prep
// commit already synchronized these. It performs local file I/O only.
func ReleaseCheck(m *Manifest, repoRoot, tag string) []Problem {
	var ps []Problem
	version, expectedTag, field := m.Release.Version, m.Release.Tag, "release"
	if m.Candidate != nil {
		version, expectedTag, field = m.Candidate.Version, m.Candidate.Tag, "candidate"
	}
	if tag != expectedTag {
		ps = append(ps, Problem{field + ".tag", fmt.Sprintf("tag %q does not equal manifest %s tag %q", tag, field, expectedTag)})
	}
	for _, sf := range releaseSourceFiles {
		v, err := readSourceVersion(repoRoot, sf)
		if err != nil {
			ps = append(ps, Problem{"release.source.read", err.Error()})
			continue
		}
		if v != version {
			ps = append(ps, Problem{field + ".source.version",
				fmt.Sprintf("%s version %q != %s %q (run release prep to sync source with the tag)", sf.file, v, field, version)})
		}
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "CHANGELOG.md"))
	if err != nil {
		ps = append(ps, Problem{"release.changelog.read", err.Error()})
	} else if !strings.Contains(string(data), "## "+expectedTag+" ") && !strings.Contains(string(data), "## "+expectedTag+"\n") {
		ps = append(ps, Problem{field + ".changelog", "CHANGELOG.md has no '## " + expectedTag + "' entry"})
	}
	return ps
}
