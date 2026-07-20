package releasetruth

import (
	"encoding/json"
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

type sourceVersion struct {
	source  string
	version string
}

type nodePackageLock struct {
	Version  string `json:"version"`
	Packages map[string]struct {
		Version string `json:"version"`
	} `json:"packages"`
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

func readSourceVersions(repoRoot string) ([]sourceVersion, []error) {
	versions := make([]sourceVersion, 0, len(releaseSourceFiles)+2)
	var errs []error
	for _, sf := range releaseSourceFiles {
		version, err := readSourceVersion(repoRoot, sf)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sf.file, err))
			continue
		}
		versions = append(versions, sourceVersion{sf.file, version})
	}
	lockVersions, err := readNodePackageLockVersions(repoRoot)
	if err != nil {
		errs = append(errs, err)
	} else {
		versions = append(versions, lockVersions...)
	}
	return versions, errs
}

func readNodePackageLockVersions(repoRoot string) ([]sourceVersion, error) {
	const name = "sdk/node/package-lock.json"
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(name)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	var lock nodePackageLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", name, err)
	}
	root, ok := lock.Packages[""]
	if strings.TrimSpace(lock.Version) == "" || !ok || strings.TrimSpace(root.Version) == "" {
		return nil, fmt.Errorf("%s: top-level version and packages[\"\"].version are required", name)
	}
	return []sourceVersion{
		{name + "#version", lock.Version},
		{name + `#packages[""].version`, root.Version},
	}, nil
}

// CheckSourceMetadata verifies that source packages are marked as unreleased
// development or exactly stamped to an explicit candidate or snapshot.
// Source must never masquerade as the latest published release.
func CheckSourceMetadata(m *Manifest, repoRoot string) []Problem {
	var ps []Problem
	versions, errs := readSourceVersions(repoRoot)
	for _, err := range errs {
		ps = append(ps, Problem{"development.source.read", err.Error()})
	}
	field, expected := sourceExpectation(m)
	for _, source := range versions {
		if source.version == m.Release.Version {
			ps = append(ps, Problem{"development.source.version",
				fmt.Sprintf("%s declares %q, equal to published release %q", source.source, source.version, m.Release.Version)})
			continue
		}
		if expected != "" {
			if source.version != expected {
				ps = append(ps, Problem{field + ".source.version",
					fmt.Sprintf("%s declares %q, not %s %q", source.source, source.version, field, expected)})
			}
			continue
		}
		if !isDevelopmentVersion(source.version) {
			ps = append(ps, Problem{"development.source.version",
				fmt.Sprintf("%s declares %q without a development/prerelease marker", source.source, source.version)})
		}
	}
	return ps
}

func sourceExpectation(m *Manifest) (string, string) {
	if m.Snapshot != nil {
		return "snapshot", m.Snapshot.Version
	}
	if m.Candidate != nil {
		return "candidate", m.Candidate.Version
	}
	return "development", ""
}

func isDevelopmentVersion(version string) bool {
	return strings.Contains(strings.ToLower(version), "dev") || strings.Contains(version, "-")
}

// ReleaseCheck validates that a release tag is internally consistent with the
// manifest, the source metadata, and the changelog. A future tag requires a
// prepared snapshot; the snapshot's containing SHA is approved out of band
// before tag creation. This function performs local file I/O only.
func ReleaseCheck(m *Manifest, repoRoot, tag string) []Problem {
	ps := Validate(m)
	version, expectedTag, field := m.Release.Version, m.Release.Tag, "release"
	if m.Snapshot != nil {
		version, expectedTag, field = m.Snapshot.Version, m.Snapshot.Tag, "snapshot"
	} else if m.Candidate != nil {
		ps = append(ps, Problem{"snapshot.required", "candidate must be frozen as a snapshot before tag validation"})
	}
	if tag != expectedTag {
		ps = append(ps, Problem{field + ".tag", fmt.Sprintf("tag %q does not equal manifest %s tag %q", tag, field, expectedTag)})
	}
	versions, errs := readSourceVersions(repoRoot)
	for _, err := range errs {
		ps = append(ps, Problem{"release.source.read", err.Error()})
	}
	for _, source := range versions {
		if source.version != version {
			ps = append(ps, Problem{field + ".source.version",
				fmt.Sprintf("%s declares %q, not %s %q (run release prep to sync source with the tag)", source.source, source.version, field, version)})
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
