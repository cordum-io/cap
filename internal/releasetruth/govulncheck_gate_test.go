package releasetruth

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	govulncheckTests = "        run: go -C tools/govulncheck test -count=1 ./..."
	govulncheckRun   = "        run: go -C tools/govulncheck run . --repo-root ../.."
)

func TestGovulncheckGateIsEnforcedInCI(t *testing.T) {
	root := repoRoot(t)
	workflow := readGovulncheckGateFile(t, root, ".github/workflows/ci.yml")
	for _, rule := range govulncheckWiringViolations(workflow) {
		t.Errorf("Govulncheck CI rule: %s", rule)
	}
	paths, err := filepath.Glob(filepath.Join(root, ".github/workflows/*.yml"))
	if err != nil {
		t.Fatalf("list workflow files: %v", err)
	}
	for _, path := range paths {
		text := activeWorkflow(readGovulncheckGateFile(t, path))
		if strings.Contains(text, "govulncheck@") {
			t.Errorf("Govulncheck rule 4: %s contains an active govulncheck@ install", filepath.Base(path))
		}
	}
	tool := readGovulncheckGateFile(t, root, "tools/govulncheck/go.mod")
	module := readGovulncheckGateFile(t, root, "go.mod")
	for _, rule := range govulncheckModuleViolations(module, tool) {
		t.Errorf("Govulncheck rule 5: %s", rule)
	}
}

func readGovulncheckGateFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(parts...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func govulncheckWiringViolations(workflow string) []string {
	job := registryJobBlock(activeWorkflow(workflow), "go")
	var rules []string
	addExactLine(&rules, job, govulncheckTests, "rule 2: exactly one pinned gate-tests command in Go SDK job")
	addExactLine(&rules, job, govulncheckRun, "rule 2: exactly one seeded gate command in Go SDK job")
	addOrder(&rules, job, "      - name: Go race (SDK)", govulncheckTests, "rule 2: gate tests after Go race (SDK)")
	addOrder(&rules, job, govulncheckTests, govulncheckRun, "rule 2: gate tests before seeded gate")
	if strings.Contains(job, "continue-on-error") {
		rules = append(rules, "rule 3: Go SDK job must not continue on error")
	}
	for _, step := range strings.Split(job, "\n      - ") {
		if !strings.Contains(step, "go -C tools/govulncheck ") {
			continue
		}
		for _, forbidden := range []string{"||", "if:", "shell:"} {
			if strings.Contains(step, forbidden) {
				rules = append(rules, "rule 3: gate step must not contain "+forbidden)
			}
		}
	}
	for _, forbidden := range []string{"GOFLAGS", "GONOSUMDB", "GOSUMDB", "GOPRIVATE", "GOTOOLCHAIN"} {
		if strings.Contains(job, forbidden) {
			rules = append(rules, "rule 3: Go SDK job must not override "+forbidden)
		}
	}
	return rules
}

func govulncheckModuleViolations(root, tool string) []string {
	var rules []string
	addExactLine(&rules, tool, "tool golang.org/x/vuln/cmd/govulncheck", "pinned scanner tool directive")
	for _, name := range []string{"go", "toolchain"} {
		pattern := regexp.MustCompile(`(?m)^` + name + `[ \t]+([^\s/]+)[ \t]*(?://[^\n]*)?$`)
		rootLines, toolLines := pattern.FindAllStringSubmatch(root, -1), pattern.FindAllStringSubmatch(tool, -1)
		if name == "toolchain" && len(rootLines) == 0 && len(toolLines) == 0 {
			continue
		}
		if len(rootLines) != 1 || len(toolLines) != 1 || rootLines[0][1] != toolLines[0][1] {
			rules = append(rules, "tool module "+name+" directive must equal root directive")
		}
	}
	return rules
}

func TestGovulncheckWiringViolations(t *testing.T) {
	valid := "jobs:\n  go:\n    steps:\n      - name: Go race (SDK)\n        run: go test -race ./sdk/go/...\n" +
		"      - name: Gate tests\n" + govulncheckTests + "\n      - name: Seeded gate\n" + govulncheckRun + "\n  node:\n"
	cases := []struct {
		name, workflow string
		wantFailure    bool
	}{
		{"valid", valid, false},
		{"CRLF and comments", strings.ReplaceAll(valid, "\n", "\r\n") + "# continue-on-error: true\r\n", false},
		{"continue on error", strings.Replace(valid, govulncheckRun, "        continue-on-error: true\n"+govulncheckRun, 1), true},
		{"swallowed failure", strings.Replace(valid, govulncheckRun, govulncheckRun+" || true", 1), true},
		{"deleted tests", strings.Replace(valid, govulncheckTests, "", 1), true},
		{"other job", strings.Replace(valid, "  go:", "  other:", 1), true},
		{"duplicate gate", strings.Replace(valid, govulncheckRun, govulncheckRun+"\n"+govulncheckRun, 1), true},
		{"wrong order", strings.NewReplacer(govulncheckTests, govulncheckRun, govulncheckRun, govulncheckTests).Replace(valid), true},
		{"conditional gate", strings.Replace(valid, govulncheckRun, "        if: false\n"+govulncheckRun, 1), true},
		{"toolchain override", strings.Replace(valid, "    steps:", "    env:\n      GOTOOLCHAIN: local\n    steps:", 1), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rules := govulncheckWiringViolations(tc.workflow); (len(rules) > 0) != tc.wantFailure {
				t.Errorf("violations = %v, want failure %v", rules, tc.wantFailure)
			}
		})
	}
}

func TestGovulncheckModuleViolations(t *testing.T) {
	root := "module example.com/root\n\ngo 1.26.0\n\ntoolchain go1.26.11\n"
	tool := root + "\ntool golang.org/x/vuln/cmd/govulncheck\n"
	cases := []struct {
		name, root, tool string
		wantFailure      bool
	}{
		{"matching", root, tool, false},
		{"reviewed scanner bump", root, tool + "\nrequire golang.org/x/vuln v1.9.0\n", false},
		{"missing tool", root, root, true},
		{"different go floor", root, strings.Replace(tool, "go 1.26.0", "go 1.27.0", 1), true},
		{"older toolchain", root, strings.Replace(tool, "go1.26.11", "go1.26.10", 1), true},
		{"missing toolchain", root, strings.Replace(tool, "toolchain go1.26.11\n", "", 1), true},
		{"unexpected toolchain", strings.Replace(root, "toolchain go1.26.11\n", "", 1), tool, true},
		{"missing go floor", root, strings.Replace(tool, "go 1.26.0\n", "", 1), true},
		{"both toolchains absent", strings.Replace(root, "toolchain go1.26.11\n", "", 1), strings.Replace(tool, "toolchain go1.26.11\n", "", 1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rules := govulncheckModuleViolations(tc.root, tc.tool); (len(rules) > 0) != tc.wantFailure {
				t.Errorf("violations = %v, want failure %v", rules, tc.wantFailure)
			}
		})
	}
}
