package releasetruth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// GitHub Actions pin contract for the Node 24 runtime migration.
//
// Every `uses:` in .github/workflows must name an approved action at its
// approved 40-hex commit SHA with a trailing `# <version>` comment, and no
// workflow may set ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION. Each pin is the
// newest release in the action's lowest Node 24 major. All were re-proven on
// 2026-11-10 with `git ls-remote` (peeled tag SHA) plus the raw action.yml at
// that SHA showing `runs.using: node24`. For composite actions every nested
// Node action is node24: pypi-publish nests setup-python v6.2.0, and
// rust-toolchain nests no action.

type actionPinEntry struct{ sha, version string }

var actionPinApproved = map[string]actionPinEntry{
	"actions/checkout":            {"fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09", "v5.1.0"},
	"actions/setup-go":            {"924ae3a1cded613372ab5595356fb5720e22ba16", "v6.5.0"},
	"actions/setup-node":          {"a0853c24544627f65ddf259abe73b1d18a591444", "v5.0.0"},
	"actions/setup-python":        {"ece7cb06caefa5fff74198d8649806c4678c61a1", "v6.3.0"},
	"actions/setup-java":          {"b6effb05e454b25005698d916606bdc6ffcbf961", "v5.7.0"},
	"actions/upload-artifact":     {"b7c566a772e6b6bfb58ed0dc250532a479d7789f", "v6.0.0"},
	"actions/download-artifact":   {"37930b1c2abaa49bbe596cd826c3c89aef350131", "v7.0.0"},
	"github/codeql-action":        {"33d7b85da1a04171e211befe2c739303b9a4c066", "v4.39.3"},
	"pypa/gh-action-pypi-publish": {"7245906fb61fa33a13554f9d5fb856fe61758086", "v1.14.3"},
	"dtolnay/rust-toolchain":      {"683b588c487a69d01f6c6ddf40f036ff682cc2c8", "v1"},
}

// actionPinNode24Floor is the first release of each action that runs on
// node24. The release before each floor runs node20, which is why it is the
// floor.
var actionPinNode24Floor = map[string]string{
	"actions/checkout":            "v5.0.0",
	"actions/setup-go":            "v6.2.0",
	"actions/setup-node":          "v5.0.0",
	"actions/setup-python":        "v6.0.0",
	"actions/setup-java":          "v5.0.0",
	"actions/upload-artifact":     "v6.0.0",
	"actions/download-artifact":   "v7.0.0",
	"github/codeql-action":        "v4.30.7",
	"pypa/gh-action-pypi-publish": "v1.14.1",
	"dtolnay/rust-toolchain":      "v1",
}

const actionPinEscapeVar = "ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION"

var (
	// RE2 has no backreferences, so the opening and closing quotes are
	// matched independently. actionlint rejects a mismatched pair anyway.
	actionPinUsesRE  = regexp.MustCompile(`^\s*(?:-\s*)?uses:\s*["']?([^"'\s#]+)["']?\s*(?:#\s*(\S+))?`)
	actionPinSHARE   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	actionPinShortRE = regexp.MustCompile(`^[0-9a-f]{7,39}$`)
	actionPinLocalRE = regexp.MustCompile(`^\./\.github/workflows/[^/]+\.ya?ml$`)
	actionPinVerRE   = regexp.MustCompile(`^v(\d+)(?:\.(\d+)\.(\d+))?$`)
)

func actionPinLines(text string) []string {
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

// actionPinParseLine extracts a `uses:` ref and its trailing comment from
// one line. Lines whose first non-space character is '#' are ignored.
func actionPinParseLine(line string) (ref, comment string, ok bool) {
	if strings.HasPrefix(strings.TrimSpace(line), "#") {
		return "", "", false
	}
	m := actionPinUsesRE.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// actionPinSplit returns the owner/repo key and the revision of an action ref.
// A subpath such as github/codeql-action/init maps to github/codeql-action.
func actionPinSplit(ref string) (key, rev string, ok bool) {
	path, rev, found := strings.Cut(ref, "@")
	parts := strings.Split(path, "/")
	if !found || len(parts) < 2 || parts[0] == "" || parts[1] == "" ||
		strings.HasPrefix(path, ".") || strings.Contains(path, ":") {
		return "", "", false
	}
	return parts[0] + "/" + parts[1], rev, true
}

// actionPinCheck returns the violation reason for one `uses:`, or "".
func actionPinCheck(ref, comment string) string {
	if actionPinLocalRE.MatchString(ref) {
		return ""
	}
	key, rev, ok := actionPinSplit(ref)
	if !ok {
		return "unsupported ref (want owner/repo@<40-hex SHA> or ./.github/workflows/*.yml)"
	}
	if actionPinShortRE.MatchString(rev) {
		return "short SHA (want 40-hex SHA)"
	}
	if !actionPinSHARE.MatchString(rev) {
		return "mutable ref (want 40-hex SHA)"
	}
	want, approved := actionPinApproved[key]
	switch {
	case !approved:
		return "unapproved action (not in the approved pin table)"
	case rev != want.sha:
		return fmt.Sprintf("unapproved SHA (want %s # %s)", want.sha, want.version)
	case comment == "":
		return fmt.Sprintf("missing version comment (want # %s)", want.version)
	case comment != want.version:
		return fmt.Sprintf("wrong version comment %q (want # %s)", comment, want.version)
	}
	return ""
}

// actionPinViolations lists every contract violation in one workflow, in line
// order, as `<file>:<line>: <subject>: <reason>`. It also reports whether the
// file holds at least one exact approved pin.
func actionPinViolations(file, text string) (out []string, hasApproved bool) {
	for i, line := range actionPinLines(text) {
		if strings.Contains(line, actionPinEscapeVar) {
			out = append(out, fmt.Sprintf("%s:%d: %s: Node 20 escape hatch is forbidden", file, i+1, actionPinEscapeVar))
		}
		ref, comment, ok := actionPinParseLine(line)
		if !ok {
			continue
		}
		if key, rev, ok := actionPinSplit(ref); ok && actionPinApproved[key].sha == rev {
			hasApproved = true
		}
		if why := actionPinCheck(ref, comment); why != "" {
			out = append(out, fmt.Sprintf("%s:%d: %s: %s", file, i+1, ref, why))
		}
	}
	return out, hasApproved
}

// actionPinFileReport applies the TRANSITIONAL rule. A file that must
// comply, or that already contains an approved pin, fails on every violation.
// Any other file fails only on the escape variable; its remaining violations
// are returned as pending migration. Slice 5 of the Node 24 migration
// (task-cf23dfc6) deletes this rule, so every workflow is then fully checked.
func actionPinFileReport(file, text string, mustComply bool) (failures, pending []string) {
	all, hasApproved := actionPinViolations(file, text)
	if mustComply || hasApproved {
		return all, nil
	}
	for _, v := range all {
		if strings.Contains(v, ": "+actionPinEscapeVar+": ") {
			failures = append(failures, v)
		} else {
			pending = append(pending, v)
		}
	}
	return failures, pending
}

func TestActionPinsAcrossWorkflows(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(repoRoot(t), ".github", "workflows", "*.y*ml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("glob workflows: %v (found %d)", err, len(files))
	}
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		name := filepath.Base(path)
		failures, pending := actionPinFileReport(name, string(b), false)
		for _, f := range failures {
			t.Error(f)
		}
		if len(pending) > 0 {
			t.Logf("pending: %s: %d unmigrated uses (TRANSITIONAL)", name, len(pending))
		}
	}
}

// actionPinVersionAtLeast compares vN or vN.N.N versions numerically.
func actionPinVersionAtLeast(v, floor string) (bool, error) {
	var nums [2][3]int
	for i, s := range []string{v, floor} {
		m := actionPinVerRE.FindStringSubmatch(s)
		if m == nil {
			return false, fmt.Errorf("version %q is not vN or vN.N.N", s)
		}
		for j, d := range m[1:] {
			nums[i][j], _ = strconv.Atoi(d) // digits only; "" (absent) is 0
		}
	}
	return slices.Compare(nums[0][:], nums[1][:]) >= 0, nil
}

func TestActionPinTableAboveNode24Floor(t *testing.T) {
	if len(actionPinApproved) != len(actionPinNode24Floor) {
		t.Errorf("approved table has %d actions, floor table %d", len(actionPinApproved), len(actionPinNode24Floor))
	}
	for action, pin := range actionPinApproved {
		floor, ok := actionPinNode24Floor[action]
		at, err := actionPinVersionAtLeast(pin.version, floor)
		switch {
		case !actionPinSHARE.MatchString(pin.sha):
			t.Errorf("%s: sha %q is not 40 lowercase hex", action, pin.sha)
		case !ok:
			t.Errorf("%s: no node24 floor", action)
		case err != nil:
			t.Errorf("%s: %v", action, err)
		case !at:
			t.Errorf("%s: approved %s is below node24 floor %s", action, pin.version, floor)
		}
	}
	for _, c := range []struct {
		v, floor string
		want     bool
	}{{"v6.1.0", "v6.2.0", false}, {"v6.2.0", "v6.2.0", true}, {"v6.10.0", "v6.2.0", true},
		{"v5.9.9", "v6.0.0", false}, {"v1.14.3", "v1.14.1", true}, {"v1", "v1", true}} {
		if got, err := actionPinVersionAtLeast(c.v, c.floor); err != nil || got != c.want {
			t.Errorf("actionPinVersionAtLeast(%s, %s) = %v, %v; want %v", c.v, c.floor, got, err, c.want)
		}
	}
	if _, err := actionPinVersionAtLeast("release/v1", "v1"); err == nil {
		t.Error("actionPinVersionAtLeast accepted a non-semver version")
	}
}

const (
	actionPinFixtureHead = "name: fixture\non: push\njobs:\n  j:\n    runs-on: ubuntu-latest\n    steps:\n"
	actionPinOKCheckout  = "      - uses: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09 # v5.1.0"
	actionPinSetupGoV5   = "        uses: actions/setup-go@v5"
	actionPinEscapeLine  = "      ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION: true"
	actionPinMutableWant = "fixture.yml:8: actions/setup-go@v5: mutable ref (want 40-hex SHA)"
	actionPinEscapeWant  = "fixture.yml:9: ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION: Node 20 escape hatch is forbidden"
)

// TestActionPinMutations runs every rule against synthetic workflows; step
// lines start at line 7, after actionPinFixtureHead.
func TestActionPinMutations(t *testing.T) {
	cases := []struct {
		name        string
		lines       []string
		crlf, loose bool // loose: mustComply=false, so the TRANSITIONAL rule decides
		want, pend  []string
	}{
		{name: "mutable tag", lines: []string{"      - uses: actions/setup-go@v5"},
			want: []string{"fixture.yml:7: actions/setup-go@v5: mutable ref (want 40-hex SHA)"}},
		{name: "node20-era SHA", lines: []string{"      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2"},
			want: []string{"fixture.yml:7: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683: unapproved SHA (want fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09 # v5.1.0)"}},
		{name: "short SHA", lines: []string{"      - uses: actions/checkout@fbc6f39 # v5.1.0"},
			want: []string{"fixture.yml:7: actions/checkout@fbc6f39: short SHA (want 40-hex SHA)"}},
		{name: "missing comment", lines: []string{"      - uses: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09"},
			want: []string{"fixture.yml:7: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09: missing version comment (want # v5.1.0)"}},
		{name: "wrong comment", lines: []string{"      - uses: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09 # v5.0.0"},
			want: []string{`fixture.yml:7: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09: wrong version comment "v5.0.0" (want # v5.1.0)`}},
		{name: "escape env var", lines: []string{actionPinOKCheckout, "    env:", actionPinEscapeLine},
			want: []string{actionPinEscapeWant}},
		{name: "unknown action", lines: []string{"      - uses: evil/x@0123456789abcdef0123456789abcdef01234567 # v1.0.0"},
			want: []string{"fixture.yml:7: evil/x@0123456789abcdef0123456789abcdef01234567: unapproved action (not in the approved pin table)"}},
		{name: "quoted mutable refs", lines: []string{`      - uses: "actions/checkout@v4"`, `        uses: 'actions/setup-go@v5'`},
			want: []string{"fixture.yml:7: actions/checkout@v4: mutable ref (want 40-hex SHA)", actionPinMutableWant}},
		{name: "unsupported refs", lines: []string{"      - uses: ./local-action", "      - uses: docker://alpine:3.20"},
			want: []string{"fixture.yml:7: ./local-action: unsupported ref (want owner/repo@<40-hex SHA> or ./.github/workflows/*.yml)",
				"fixture.yml:8: docker://alpine:3.20: unsupported ref (want owner/repo@<40-hex SHA> or ./.github/workflows/*.yml)"}},
		{name: "commented-out uses ignored", lines: []string{actionPinOKCheckout, "      # - uses: actions/checkout@v4", "#uses: evil/x@v1"}},
		{name: "CRLF line endings", crlf: true, lines: []string{actionPinOKCheckout, actionPinSetupGoV5},
			want: []string{actionPinMutableWant}},
		{name: "transitional: approved pin forces full check", loose: true, lines: []string{actionPinOKCheckout, actionPinSetupGoV5},
			want: []string{actionPinMutableWant}},
		{name: "transitional: unmigrated file is pending", loose: true, lines: []string{"      - uses: actions/checkout@v4", actionPinSetupGoV5},
			pend: []string{"fixture.yml:7: actions/checkout@v4: mutable ref (want 40-hex SHA)", actionPinMutableWant}},
		{name: "transitional: escape var fails a pending file", loose: true, lines: []string{"      - uses: actions/checkout@v4", "    env:", actionPinEscapeLine},
			want: []string{actionPinEscapeWant}, pend: []string{"fixture.yml:7: actions/checkout@v4: mutable ref (want 40-hex SHA)"}},
		{name: "fully approved", lines: []string{actionPinOKCheckout,
			"        uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0",
			"        uses: github/codeql-action/init@33d7b85da1a04171e211befe2c739303b9a4c066 # v4.39.3",
			"      - uses: dtolnay/rust-toolchain@683b588c487a69d01f6c6ddf40f036ff682cc2c8 # v1",
			"    uses: ./.github/workflows/onboarding.yml"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := actionPinFixtureHead + strings.Join(c.lines, "\n") + "\n"
			if c.crlf {
				text = strings.ReplaceAll(text, "\n", "\r\n")
			}
			got, pend := actionPinFileReport("fixture.yml", text, !c.loose)
			if !slices.Equal(got, c.want) || !slices.Equal(pend, c.pend) {
				t.Errorf("failures = %q, pending = %q\nwant failures %q, pending %q", got, pend, c.want, c.pend)
			}
		})
	}
}
