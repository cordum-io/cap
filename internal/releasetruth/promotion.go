package releasetruth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PromotionCheck binds the latest verified published release metadata to the
// already-resolved immutable tag target and its dated changelog entry.
func PromotionCheck(m *Manifest, repoRoot, resolvedTagCommit string) []Problem {
	ps := Validate(m)
	resolved := strings.TrimSpace(resolvedTagCommit)
	if resolved != m.Release.Commit {
		ps = append(ps, Problem{
			"release.commit.tagTarget",
			fmt.Sprintf("release tag %s resolves to %q, not manifest commit %q", m.Release.Tag, resolved, m.Release.Commit),
		})
	}
	date, err := readChangelogReleaseDate(repoRoot, m.Release.Tag)
	if err != nil {
		ps = append(ps, Problem{"release.date.changelog", err.Error()})
	} else if date != m.Release.Date {
		ps = append(ps, Problem{
			"release.date.changelog",
			fmt.Sprintf("release date %q does not match changelog date %q", m.Release.Date, date),
		})
	}
	return ps
}

func readChangelogReleaseDate(repoRoot, tag string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "CHANGELOG.md"))
	if err != nil {
		return "", fmt.Errorf("read CHANGELOG.md: %w", err)
	}
	pattern := `(?m)^##[ \t]+` + regexp.QuoteMeta(tag) + `[ \t]+—[ \t]+(\d{4}-\d{2}-\d{2})[ \t\r]*$`
	matches := regexp.MustCompile(pattern).FindAllStringSubmatch(string(data), -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("CHANGELOG.md must contain exactly one dated '## %s — YYYY-MM-DD' heading; found %d", tag, len(matches))
	}
	return matches[0][1], nil
}
