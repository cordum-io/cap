package releasetruth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// minimumSecureGo is the lowest Go release CAP may declare. Go 1.27.2 fixes
// the reachable standard-library advisories GO-2026-6599, GO-2026-6600,
// GO-2026-6603, GO-2026-6605, GO-2026-6607, GO-2026-6608, GO-2026-6610,
// GO-2026-6611, GO-2026-6612, GO-2026-6613 and GO-2026-6617. Raise it, never
// lower it.
const minimumSecureGo = "1.27.2"

var (
	threePartVersionRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	goVersionKeyRE     = regexp.MustCompile(`(?:^|[\s{,])go-version:[ \t]*([^,}#]*)`)
	gettingStartedGoRE = regexp.MustCompile("^[ \t]*- Go `([^`]*)` or newer\\.$")
)

func normalizeEOL(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// goDirective returns the version of the single `go X.Y.Z` directive in a
// go.mod. A toolchain directive is rejected so go.mod stays the one source of
// truth that setup-go's go-version-file reads.
func goDirective(gomod string) (string, error) {
	var found []string
	for _, line := range strings.Split(normalizeEOL(gomod), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "toolchain" || strings.HasPrefix(fields[0], "toolchain(") {
			return "", fmt.Errorf("go.mod has a toolchain directive %q; keep the go directive as the only source", strings.TrimSpace(line))
		}
		if fields[0] != "go" {
			continue
		}
		if len(fields) != 2 || !threePartVersionRE.MatchString(fields[1]) {
			return "", fmt.Errorf("go.mod go directive %q is not `go X.Y.Z`", strings.TrimSpace(line))
		}
		found = append(found, fields[1])
	}
	if len(found) != 1 {
		return "", fmt.Errorf("go.mod has %d go directives, want exactly 1", len(found))
	}
	return found[0], nil
}

// explicitGoVersions returns every `go-version:` value in a workflow file,
// quoted or bare. `go-version-file:` is not an explicit version.
func explicitGoVersions(yaml string) []string {
	var out []string
	for _, line := range strings.Split(normalizeEOL(yaml), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for _, m := range goVersionKeyRE.FindAllStringSubmatch(line, -1) {
			v := strings.TrimSpace(m[1])
			if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
				v = v[1 : len(v)-1]
			}
			out = append(out, v)
		}
	}
	return out
}

// gettingStartedGo returns the version in the single "- Go `X` or newer."
// prerequisite bullet.
func gettingStartedGo(md string) (string, error) {
	var found []string
	for _, line := range strings.Split(normalizeEOL(md), "\n") {
		if m := gettingStartedGoRE.FindStringSubmatch(line); m != nil {
			found = append(found, m[1])
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("found %d \"- Go `X` or newer.\" prerequisites, want exactly 1", len(found))
	}
	return found[0], nil
}

// versionAtLeast compares two X.Y.Z versions numerically, so 1.27.10 > 1.27.2.
func versionAtLeast(v, floor string) (bool, error) {
	a, err := parseThreePart(v)
	if err != nil {
		return false, err
	}
	b, err := parseThreePart(floor)
	if err != nil {
		return false, err
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i], nil
		}
	}
	return true, nil
}

func parseThreePart(v string) ([3]int, error) {
	var out [3]int
	if !threePartVersionRE.MatchString(v) {
		return out, fmt.Errorf("version %q is not X.Y.Z", v)
	}
	for i, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("version %q: %w", v, err)
		}
		out[i] = n
	}
	return out, nil
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return normalizeEOL(string(b))
}

// TestToolchainDeclarationsAgree proves every Go toolchain declaration selects
// the root go directive, and that the directive is not below minimumSecureGo.
func TestToolchainDeclarationsAgree(t *testing.T) {
	m, root := loadGolden(t)
	want, err := goDirective(readRepoFile(t, root, "go.mod"))
	if err != nil {
		t.Fatalf("go.mod: %v", err)
	}
	if ok, err := versionAtLeast(want, minimumSecureGo); err != nil {
		t.Fatalf("go.mod: %v", err)
	} else if !ok {
		t.Errorf("go.mod: go directive = %s, want >= security floor %s", want, minimumSecureGo)
	}
	checkManifestGoToolchain(t, m, want)
	checkWorkflowGoVersions(t, root, want)
	got, err := gettingStartedGo(readRepoFile(t, root, "docs/getting-started.md"))
	if err != nil {
		t.Errorf("docs/getting-started.md: %v", err)
	} else if got != want {
		t.Errorf("docs/getting-started.md: Go prerequisite = %s, want %s (go.mod)", got, want)
	}
}

func checkManifestGoToolchain(t *testing.T, m *Manifest, want string) {
	t.Helper()
	var goSDKs []Component
	for _, c := range m.Components {
		if c.Name == "cap-go" {
			goSDKs = append(goSDKs, c)
		}
	}
	if len(goSDKs) != 1 {
		t.Errorf("release/manifest.json: %d cap-go components, want exactly 1", len(goSDKs))
	} else if goSDKs[0].Toolchain != "go"+want {
		t.Errorf("release/manifest.json: cap-go toolchain = %q, want %q (go.mod)", goSDKs[0].Toolchain, "go"+want)
	}
	if got := m.Toolchains.Go.Tested; got != want {
		t.Errorf("release/manifest.json: toolchains.go.tested = %q, want %q (go.mod)", got, want)
	}
	if got := m.Toolchains.Go.Minimum; got != want {
		t.Errorf("release/manifest.json: toolchains.go.minimum = %q, want %q (go.mod)", got, want)
	}
}

func checkWorkflowGoVersions(t *testing.T, root, want string) {
	t.Helper()
	var files []string
	for _, pattern := range []string{"*.yml", "*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(root, ".github", "workflows", pattern))
		if err != nil {
			t.Fatalf("glob workflows: %v", err)
		}
		files = append(files, matches...)
	}
	sawOnboarding := false
	for _, path := range files {
		rel := ".github/workflows/" + filepath.Base(path)
		versions := explicitGoVersions(readRepoFile(t, root, rel))
		for _, got := range versions {
			if got != want {
				t.Errorf("%s: go-version = %q, want %q (go.mod)", rel, got, want)
			}
		}
		if filepath.Base(path) == "onboarding.yml" {
			sawOnboarding = true
			if len(versions) != 1 {
				t.Errorf("%s: %d explicit go-version values, want exactly 1", rel, len(versions))
			}
		}
	}
	if !sawOnboarding {
		t.Errorf(".github/workflows/onboarding.yml: not found among %d workflow files", len(files))
	}
}

func TestGoDirective(t *testing.T) {
	cases := []struct {
		name, gomod, want string
		wantErr           bool
	}{
		{"plain", "module m\n\ngo 1.27.2\n\nrequire (\n\tgolang.org/x/net v0.60.0 // indirect\n)\n", "1.27.2", false},
		{"crlf and comment", "module m\r\n\r\ngo 1.27.10 // floor\r\n", "1.27.10", false},
		{"missing", "module m\n", "", true},
		{"duplicated", "module m\ngo 1.27.2\ngo 1.27.2\n", "", true},
		{"two part", "module m\ngo 1.27\n", "", true},
		{"prerelease", "module m\ngo 1.27rc1\n", "", true},
		{"toolchain directive", "module m\ngo 1.27.2\ntoolchain go1.27.2\n", "", true},
		{"toolchain block", "module m\ngo 1.27.2\ntoolchain (\n\tgo1.27.2\n)\n", "", true},
	}
	for _, tc := range cases {
		got, err := goDirective(tc.gomod)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("%s: goDirective = %q, %v; want %q, err=%v", tc.name, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestExplicitGoVersions(t *testing.T) {
	cases := []struct {
		name, yaml string
		want       []string
	}{
		{"double quoted", "        with:\n          go-version: \"1.27.2\"\n", []string{"1.27.2"}},
		{"single quoted crlf", "        with:\r\n          go-version: '1.27.2'\r\n", []string{"1.27.2"}},
		{"bare with comment", "      - go-version: 1.25.12 # pinned\n", []string{"1.25.12"}},
		{"flow style", "        with: { go-version: \"1.26.9\", cache: false }\n", []string{"1.26.9"}},
		{"version file only", "          go-version-file: 'go.mod'\n", nil},
		{"comment only", "# go-version: 1.20.1\n", nil},
		{"several", "go-version: 1.27.2\nx:\n  go-version: \"1.27.1\"\n", []string{"1.27.2", "1.27.1"}},
	}
	for _, tc := range cases {
		if got := explicitGoVersions(tc.yaml); strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: explicitGoVersions = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestGettingStartedGo(t *testing.T) {
	cases := []struct {
		name, md, want string
		wantErr        bool
	}{
		{"nested bullet", "- One language toolchain:\n  - Go `1.27.2` or newer.\n  - Node.js `20`.\n", "1.27.2", false},
		{"crlf", "  - Go `1.27.2` or newer.\r\n", "1.27.2", false},
		{"missing", "- Python `3.11` or newer.\n", "", true},
		{"duplicated", "- Go `1.27.2` or newer.\n- Go `1.25.12` or newer.\n", "", true},
	}
	for _, tc := range cases {
		got, err := gettingStartedGo(tc.md)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("%s: gettingStartedGo = %q, %v; want %q, err=%v", tc.name, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		v, floor string
		want     bool
		wantErr  bool
	}{
		{"1.27.2", "1.27.2", true, false},
		{"1.27.10", "1.27.2", true, false},
		{"1.28.0", "1.27.2", true, false},
		{"2.0.0", "1.27.2", true, false},
		{"1.27.1", "1.27.2", false, false},
		{"1.26.99", "1.27.2", false, false},
		{"1.25.12", "1.27.2", false, false},
		{"1.27", "1.27.2", false, true},
		{"1.27.2", "x.y.z", false, true},
	}
	for _, tc := range cases {
		got, err := versionAtLeast(tc.v, tc.floor)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("versionAtLeast(%q, %q) = %v, %v; want %v, err=%v", tc.v, tc.floor, got, err, tc.want, tc.wantErr)
		}
	}
}
