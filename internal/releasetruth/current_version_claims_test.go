package releasetruth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// currentVersionClaims lists the hand-written "current release" claims that
// cap-release render does not own. A release promotion must update each of
// them to the manifest's published version, or TestCurrentVersionClaimsMatchManifest
// fails. spec/17's current-release line is a rendered block and is covered by
// `cap-release render --check`, so it is deliberately absent here.
//
// Each pattern has exactly one capture group: the claimed version.
// An empty component compares against the release version itself.
var currentVersionClaims = []struct {
	path      string
	pattern   *regexp.Regexp
	component string
}{
	{"docs/getting-started.md", regexp.MustCompile(`\(current: \*\*v(\S+?)\*\*\)`), ""},
	{"sdk/go/README.md", regexp.MustCompile(`go get github\.com/cordum-io/cap/v2@v(\S+)`), "cap-go"},
	{"sdk/node/README.md", regexp.MustCompile(`npm install cap-sdk-node@(\S+)`), "cap-node"},
	{"sdk/python/README.md", regexp.MustCompile(`pip install cap-sdk-python==(\S+)`), "cap-python"},
}

var strictSemver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// extractCurrentClaim returns the single captured version of re in text. It
// fails on zero matches, on more than one match, and on a capture that is not
// a strict X.Y.Z (a published current release never has a pre-release or dev
// suffix).
func extractCurrentClaim(text string, re *regexp.Regexp) (string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	matches := re.FindAllStringSubmatch(text, -1)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("claim missing: no match for %s", re)
	case 1:
	default:
		return "", fmt.Errorf("claim duplicated: %d matches for %s", len(matches), re)
	}
	got := matches[0][1]
	if !strictSemver.MatchString(got) {
		return "", fmt.Errorf("claim malformed: %q is not X.Y.Z", got)
	}
	return got, nil
}

func TestCurrentVersionClaimsMatchManifest(t *testing.T) {
	m, root := loadGolden(t)
	for _, c := range currentVersionClaims {
		t.Run(c.path, func(t *testing.T) {
			want := m.Release.Version
			if c.component != "" {
				var found bool
				for _, comp := range m.Components {
					if comp.Name != c.component {
						continue
					}
					found = true
					if comp.Publication != "published" {
						t.Fatalf("%s: component %s publication is %q, want \"published\"", c.path, c.component, comp.Publication)
					}
					want = comp.Version
				}
				if !found {
					t.Fatalf("%s: component %s not in manifest", c.path, c.component)
				}
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.path)))
			if err != nil {
				t.Fatalf("read %s: %v", c.path, err)
			}
			got, err := extractCurrentClaim(string(data), c.pattern)
			if err != nil {
				t.Fatalf("%s: %v", c.path, err)
			}
			if got != want {
				t.Errorf("%s claims current version %s, manifest published version is %s", c.path, got, want)
			}
		})
	}
}

func TestExtractCurrentClaim(t *testing.T) {
	re := regexp.MustCompile(`install pkg@(\S+)`)
	tests := []struct {
		name    string
		text    string
		want    string
		wantErr string
	}{
		{"ok", "run install pkg@2.17.0 now", "2.17.0", ""},
		{"wrong but well-formed is not an extraction error", "install pkg@2.16.1", "2.16.1", ""},
		{"crlf", "a\r\ninstall pkg@2.17.0\r\nb\r\n", "2.17.0", ""},
		{"missing", "install pkg", "", "missing"},
		{"duplicate", "install pkg@2.17.0\ninstall pkg@2.17.0", "", "duplicated"},
		{"malformed short", "install pkg@2.17", "", "malformed"},
		{"malformed prerelease", "install pkg@2.17.0-rc.1", "", "malformed"},
		{"malformed dev", "install pkg@2.17.1.dev0", "", "malformed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractCurrentClaim(tc.text, re)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
