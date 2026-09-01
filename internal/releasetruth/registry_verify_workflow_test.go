package releasetruth

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	checkoutPin = "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683"
	setupGoPin  = "actions/setup-go@d35c59abb061a4a6fb18e82ac0862c26744d6ab5"
	setupPyPin  = "actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065"
	natsPin     = "nats:2.12.6-alpine@sha256:1cfc36e2e5e638243d8c722f72c954cd0ec4b15ee82fadbc718ce12e2b3c1652"
)

func readRegistryWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".github", "workflows", "registry-verify.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Registry Verify workflow: %v", err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func addMissing(violations *[]string, text, value, label string) {
	if !strings.Contains(text, value) {
		*violations = append(*violations, label)
	}
}

func addCount(violations *[]string, text, value string, count int, label string) {
	if strings.Count(text, value) != count {
		*violations = append(*violations, label)
	}
}

func addOrder(violations *[]string, text, first, second, label string) {
	left, right := strings.Index(text, first), strings.Index(text, second)
	if left < 0 || right < 0 || left >= right {
		*violations = append(*violations, label)
	}
}

func registryJobNames(text string) []string {
	start := strings.Index(text, "\njobs:\n")
	if start < 0 {
		return nil
	}
	matcher := regexp.MustCompile(`(?m)^  ([a-z][a-z0-9-]*):\s*$`)
	matches := matcher.FindAllStringSubmatch(text[start:], -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1])
	}
	return names
}

func validateRegistryShape(text string) []string {
	var violations []string
	addMissing(&violations, text, "  schedule:\n", "schedule trigger")
	addMissing(&violations, text, "  workflow_dispatch:\n", "workflow_dispatch trigger")
	if !slices.Equal(registryJobNames(text), []string{"build-proof", "live"}) {
		violations = append(violations, "isolated build-proof and live jobs")
	}
	addCount(&violations, text, "    timeout-minutes: 15", 2, "explicit job timeouts")
	addMissing(&violations, text, "os: [ubuntu-24.04, windows-2025]", "Windows/Linux build matrix")
	addMissing(&violations, text, "fail-fast: false", "non-short-circuit matrix")
	addMissing(&violations, text, "needs: build-proof", "live job depends on build proof")
	addMissing(&violations, text, "needs.build-proof.result == 'success'", "successful matrix dependency")
	return violations
}

func validateRegistryPins(text string) []string {
	var violations []string
	addCount(&violations, text, checkoutPin, 2, "immutable checkout pin")
	addCount(&violations, text, setupGoPin, 2, "immutable setup-go pin")
	addCount(&violations, text, setupPyPin, 2, "immutable setup-python pin")
	addCount(&violations, text, "persist-credentials: false", 2, "checkout credential isolation")
	addCount(&violations, text, "go-version-file: 'go.mod'", 2, "root Go version file")
	addCount(&violations, text, "python-version: '3.11'", 2, "exact Python version")
	addMissing(&violations, text, natsPin, "immutable NATS 2.12.6 image")
	return violations
}

func validateRegistryCommands(text string) []string {
	var violations []string
	unit := "python -m unittest tools.registry_verify.test_go_consumer"
	build := "tools/registry_verify/go_consumer.py --manifest release/manifest.json --example examples/quickstart-echo/main.go --build-only"
	run := "tools/registry_verify/go_consumer.py --manifest release/manifest.json --example examples/quickstart-echo/main.go --run"
	addMissing(&violations, text, unit, "helper unit gate")
	addMissing(&violations, text, build, "matrix build-only gate")
	addMissing(&violations, text, run, "live readonly run gate")
	addOrder(&violations, text, unit, build, "helper unit gate before build proof")
	addOrder(&violations, text, "Registry exact-version proof", "Go released-artifact quickstart", "registry proof before Go run")
	addOrder(&violations, text, "Go released-artifact quickstart", "Python released-artifact quickstart", "Go run before Python lane")
	addMissing(&violations, text, "VERSION=\"$(python", "manifest-derived version")
	addMissing(&violations, text, `"cap-sdk-python==${VERSION}" "nats-py==2.6.0"`, "exact Python artifact input")
	addMissing(&violations, text, `test "$failures" -eq 0`, "ABSENT and UNKNOWN fail closed")
	for _, registry := range []string{"proxy.golang.org", "registry.npmjs.org", "pypi.org/pypi/cap-sdk-python", "pypi.org/pypi/cordum-guard"} {
		addMissing(&violations, text, registry, "all exact-version registry probes")
	}
	return violations
}

func validateRegistryForbidden(text string) []string {
	var violations []string
	for _, forbidden := range []string{"nats:2.10", "@latest", "continue-on-error", "pip install --upgrade pip", "GOSUMDB=off", "GONOSUMDB=*", "GOPROXY=direct", " replace "} {
		if strings.Contains(text, forbidden) {
			violations = append(violations, "forbidden bypass: "+forbidden)
		}
	}
	if regexp.MustCompile(`actions/[a-z-]+@v[0-9]`).MatchString(text) {
		violations = append(violations, "mutable action tag")
	}
	if strings.Contains(text, "github.com/nats-io/nats.go") || strings.Contains(text, "google.golang.org/protobuf") {
		violations = append(violations, "unversioned Go extras")
	}
	if strings.Contains(text, "2.16.1") || strings.Contains(text, "2.17.0") {
		violations = append(violations, "hard-coded release version")
	}
	return violations
}

func validateRegistryWorkflow(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	violations := validateRegistryShape(text)
	violations = append(violations, validateRegistryPins(text)...)
	violations = append(violations, validateRegistryCommands(text)...)
	return append(violations, validateRegistryForbidden(text)...)
}

func TestRegistryVerifyWorkflowContract(t *testing.T) {
	if violations := validateRegistryWorkflow(readRegistryWorkflow(t)); len(violations) != 0 {
		t.Fatalf("Registry Verify workflow contract violations:\n- %s", strings.Join(violations, "\n- "))
	}
}

func TestRegistryVerifyWorkflowMutations(t *testing.T) {
	workflow := readRegistryWorkflow(t)
	mutations := []struct {
		name, old, replacement, want string
	}{
		{"drop schedule", "  schedule:\n", "  disabled_schedule:\n", "schedule trigger"},
		{"drop Windows", "os: [ubuntu-24.04, windows-2025]", "os: [ubuntu-24.04]", "Windows/Linux build matrix"},
		{"mutable checkout", checkoutPin, "actions/checkout@v4", "immutable checkout pin"},
		{"drop checkout fence", "persist-credentials: false", "persist-credentials: true", "checkout credential isolation"},
		{"float broker", natsPin, "nats:2.10", "immutable NATS 2.12.6 image"},
		{"drop unit gate", "python -m unittest tools.registry_verify.test_go_consumer", "echo skipped-unit", "helper unit gate"},
		{"drop build gate", " --build-only", " --run", "matrix build-only gate"},
		{"drop live gate", " --run", " --build-only", "live readonly run gate"},
		{"allow unknown", `test "$failures" -eq 0`, "true", "ABSENT and UNKNOWN fail closed"},
		{"pip upgrade", "Python released-artifact quickstart", "Python released-artifact quickstart\n        run: pip install --upgrade pip", "forbidden bypass"},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := strings.Replace(workflow, mutation.old, mutation.replacement, 1)
			if changed == workflow {
				t.Fatalf("mutation target absent: %q", mutation.old)
			}
			if !containsViolation(validateRegistryWorkflow(changed), mutation.want) {
				t.Fatalf("mutation escaped validator; want violation containing %q", mutation.want)
			}
		})
	}
}

func containsViolation(violations []string, want string) bool {
	for _, violation := range violations {
		if strings.Contains(violation, want) {
			return true
		}
	}
	return false
}
