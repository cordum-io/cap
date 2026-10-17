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

func addExactLine(violations *[]string, text, line, label string) {
	count := 0
	for _, candidate := range strings.Split(text, "\n") {
		if candidate == line {
			count++
		}
	}
	if count != 1 {
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

func activeWorkflow(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	active := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if comment := strings.Index(line, " #"); comment >= 0 {
			line = line[:comment]
		}
		active = append(active, line)
	}
	return strings.Join(active, "\n")
}

func registryJobBlock(text, name string) string {
	marker := "\n  " + name + ":\n"
	start := strings.Index(text, marker)
	if start < 0 {
		return ""
	}
	block := text[start+len(marker):]
	next := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:\s*$`).FindStringIndex(block)
	if next != nil {
		block = block[:next[0]]
	}
	return block
}

func validateRegistryShape(text string) []string {
	var violations []string
	build := registryJobBlock(text, "build-proof")
	live := registryJobBlock(text, "live")
	addMissing(&violations, text, "  schedule:\n", "schedule trigger")
	addMissing(&violations, text, "  workflow_dispatch:\n", "workflow_dispatch trigger")
	if !slices.Equal(registryJobNames(text), []string{"build-proof", "live"}) {
		violations = append(violations, "isolated build-proof and live jobs")
	}
	addCount(&violations, text, "    timeout-minutes: 15", 2, "explicit job timeouts")
	addMissing(&violations, build, "os: [ubuntu-24.04, windows-2025]", "Windows/Linux build matrix")
	addMissing(&violations, build, "fail-fast: false", "non-short-circuit matrix")
	addMissing(&violations, live, "needs: build-proof", "live job depends on build proof")
	addExactLine(&violations, live, "    if: ${{ needs.build-proof.result == 'success' }}", "successful matrix dependency")
	if strings.HasPrefix(build, "    if:") || strings.Contains(build, "\n    if:") {
		violations = append(violations, "build proof job condition")
	}
	addMissing(&violations, live, "    services:\n", "live NATS service")
	addMissing(&violations, live, "          - 4222:4222", "live NATS port")
	addMissing(&violations, live, "      CAP_NATS_URL: nats://127.0.0.1:4222", "live NATS URL")
	return violations
}

func validateRegistryPins(text string) []string {
	var violations []string
	addCount(&violations, text, checkoutPin, 2, "immutable checkout pin")
	addCount(&violations, text, setupGoPin, 2, "immutable setup-go pin")
	addCount(&violations, text, setupPyPin, 2, "immutable setup-python pin")
	addCount(&violations, text, "persist-credentials: false", 2, "checkout credential isolation")
	addCount(&violations, text, "go-version-file: 'go.mod'", 2, "root Go version file")
	addCount(&violations, text, "python-version: '3.11.9'", 2, "exact Python version")
	addMissing(&violations, registryJobBlock(text, "live"), natsPin, "immutable NATS 2.12.6 image")
	return violations
}

func validateRegistryCommands(text string) []string {
	var violations []string
	unit := "        run: python -m unittest tools.registry_verify.test_go_consumer tools.registry_verify.test_process_runner"
	command := "python tools/registry_verify/go_consumer.py --manifest release/manifest.json --example examples/quickstart-echo/main.go"
	build, run := "        run: "+command+" --build-only", "        run: "+command+" --run"
	buildJob, liveJob := registryJobBlock(text, "build-proof"), registryJobBlock(text, "live")
	addExactLine(&violations, buildJob, unit, "helper unit gate")
	addExactLine(&violations, buildJob, build, "matrix build-only gate")
	addExactLine(&violations, liveJob, run, "live readonly run gate")
	if strings.Contains(buildJob, "\n        if:") {
		violations = append(violations, "helper unit gate may not be conditional")
	}
	addOrder(&violations, buildJob, unit, build, "helper unit gate before build proof")
	addOrder(&violations, text, "Registry exact-version proof", "Go released-artifact quickstart", "registry proof before Go run")
	addOrder(&violations, text, "Go released-artifact quickstart", "Python released-artifact quickstart", "Go run before Python lane")
	addCount(&violations, text, "VERSION=\"$(python", 2, "manifest-derived version")
	addMissing(&violations, text, `"cap-sdk-python==${VERSION}" "nats-py==2.6.0"`, "exact Python artifact input")
	validateRegistryFailureFence(&violations, liveJob)
	addCount(&violations, text, canonicalRegistryProof, 1, "canonical registry proof")
	return violations
}

func validateRegistryFailureFence(violations *[]string, live string) {
	addCount(violations, live, "          failures=0", 1, "ABSENT and UNKNOWN fail closed")
	addCount(violations, live, `          test "$failures" -eq 0`, 1, "ABSENT and UNKNOWN fail closed")
	addMissing(violations, live, `404|410) echo "${name}: ABSENT (${code})" >&2; return 1 ;;`, "ABSENT and UNKNOWN fail closed")
	addMissing(violations, live, "echo \"${name}: UNKNOWN after bounded retries\" >&2\n            return 1", "ABSENT and UNKNOWN fail closed")
	probes := []string{
		`check "go v${VERSION}" "https://proxy.golang.org/github.com/cordum-io/cap/v2/@v/v${VERSION}.info"`,
		`check "npm cap-sdk-node" "https://registry.npmjs.org/cap-sdk-node/${VERSION}"`,
		`check "pypi cap-sdk-python" "https://pypi.org/pypi/cap-sdk-python/${VERSION}/json"`,
		`check "pypi cordum-guard" "https://pypi.org/pypi/cordum-guard/${VERSION}/json"`,
	}
	for _, probe := range probes {
		addMissing(violations, live, probe+` || failures=$((failures + 1))`, "all exact-version registry probes")
	}
}

func validateRegistryForbidden(text string) []string {
	var violations []string
	for _, forbidden := range []string{"nats:2.10", "@latest", "continue-on-error", "pip install --upgrade pip", "GOSUMDB=off", "GONOSUMDB=*", "GOPROXY=direct", " replace ", "if: false", "if: ${{ false }}"} {
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
	text = activeWorkflow(text)
	violations := validateRegistryShape(text)
	violations = append(violations, validateRegistryPins(text)...)
	violations = append(violations, validateRegistryCommands(text)...)
	validateCanonicalRegistryJobs(&violations, registryJobBlock(text, "build-proof"), registryJobBlock(text, "live"))
	return append(violations, validateRegistryForbidden(text)...)
}

func TestRegistryVerifyWorkflowContract(t *testing.T) {
	if violations := validateRegistryWorkflow(readRegistryWorkflow(t)); len(violations) != 0 {
		t.Fatalf("Registry Verify workflow contract violations:\n- %s", strings.Join(violations, "\n- "))
	}
}

func TestRegistryVerifyWorkflowMutations(t *testing.T) {
	workflow := readRegistryWorkflow(t)
	for _, mutation := range registryWorkflowMutations {
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
	commented := strings.Replace(workflow, "python -m unittest", "# python -m unittest", 1)
	if !containsViolation(validateRegistryWorkflow(commented), "helper unit gate") {
		t.Fatal("commented helper command escaped validator")
	}
	swapped := strings.Replace(workflow, "--build-only", "--SWAP", 1)
	swapped = strings.Replace(swapped, "--run", "--build-only", 1)
	swapped = strings.Replace(swapped, "--SWAP", "--run", 1)
	if !containsViolation(validateRegistryWorkflow(swapped), "matrix build-only gate") {
		t.Fatal("matrix/live command swap escaped validator")
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
