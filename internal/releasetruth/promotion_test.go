package releasetruth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromotionCheck_RequiresTagTargetCommit(t *testing.T) {
	m := validManifest()
	root := writeReleaseTree(t, "2.15.0-dev.0", m.Release.Tag)
	wrong := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	if ps := PromotionCheck(m, root, wrong); !hasField(ps, "release.commit.tagTarget") {
		t.Fatalf("PromotionCheck(wrong tag target) = %v, want release.commit.tagTarget", ps)
	}
}

func TestValidate_ReleaseCommitRequiresFullSHA(t *testing.T) {
	m := validManifest()
	m.Release.Commit = "e65a0a8"
	if ps := Validate(m); !hasField(ps, "release.commit") {
		t.Fatalf("Validate(abbreviated release commit) = %v, want release.commit", ps)
	}
}

func TestPromotionCheck_BindsReleaseDateToChangelog(t *testing.T) {
	m := validManifest()
	m.Release.Date = "2026-06-03"
	root := writeReleaseTree(t, "2.15.0-dev.0", m.Release.Tag)

	if ps := PromotionCheck(m, root, m.Release.Commit); !hasField(ps, "release.date.changelog") {
		t.Fatalf("PromotionCheck(stale date) = %v, want release.date.changelog", ps)
	}
}

func TestPromotionCheck_AcceptsBoundPublishedRelease(t *testing.T) {
	m := validManifest()
	root := writeReleaseTree(t, "2.15.0-dev.0", m.Release.Tag)

	if ps := PromotionCheck(m, root, m.Release.Commit); len(ps) != 0 {
		t.Fatalf("PromotionCheck(bound release) = %v, want none", ps)
	}
}

func TestValidate_PublishedReleaseClaimsFailClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
		field  string
	}{
		{
			name: "current security line missing",
			mutate: func(m *Manifest) {
				m.Security.SupportedLines = []string{"2.13.x"}
			},
			field: "security.supportedLines.current",
		},
		{
			name: "stable component not published",
			mutate: func(m *Manifest) {
				m.Components[0].Publication = "unpublished"
			},
			field: "components.published.publication",
		},
		{
			name: "stable evidence embeds stale version",
			mutate: func(m *Manifest) {
				m.Components[0].Evidence = "released at tag v2.13.0"
			},
			field: "components.published.evidence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			tt.mutate(m)
			if ps := Validate(m); !hasField(ps, tt.field) {
				t.Fatalf("Validate() = %v, want %s", ps, tt.field)
			}
		})
	}
}

func setChangelogDate(t *testing.T, root, oldDate, newDate string) {
	t.Helper()
	path := filepath.Join(root, "CHANGELOG.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), oldDate, newDate, 1)
	if updated == string(data) {
		t.Fatalf("CHANGELOG.md has no date %s", oldDate)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}
