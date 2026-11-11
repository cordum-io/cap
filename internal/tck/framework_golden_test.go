package tck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
)

// The framework golden file is the case set every CAP framework adapter replays
// (docs/adoption/framework-adapters-design.md). This test is its schema; workloads
// are symbolic names an operator registry resolves, never packet-selected code.

type frameworkGolden struct {
	SchemaVersion int          `json:"schemaVersion"`
	Description   string       `json:"description"`
	Cases         []goldenCase `json:"cases"`
}

type goldenCase struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Workload string         `json:"workload"`
	Input    map[string]any `json:"input"`
	Limits   *goldenLimits  `json:"limits,omitempty"`
	Cancel   *goldenCancel  `json:"cancel,omitempty"`
	Expect   goldenExpect   `json:"expect"`
}

// goldenLimits are operator-provisioned harness bounds, never packet fields.
type goldenLimits struct {
	DeadlineMs     int64 `json:"deadlineMs,omitempty"`
	MaxOutputBytes int64 `json:"maxOutputBytes,omitempty"`
}

type goldenCancel struct {
	AfterProgress string `json:"afterProgress"`
}

type goldenExpect struct {
	Status        string           `json:"status"`
	Progress      []string         `json:"progress"`
	Output        map[string]any   `json:"output,omitempty"`
	Artifacts     []goldenArtifact `json:"artifacts,omitempty"`
	ErrorCode     string           `json:"errorCode,omitempty"`
	ErrorString   string           `json:"errorString,omitempty"`
	ForbiddenText []string         `json:"forbiddenText,omitempty"`
}

type goldenArtifact struct {
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Content   string `json:"content"`
}

var (
	goldenIDPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)
	goldenMarkerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	goldenDriveLetter   = regexp.MustCompile(`^[A-Za-z]:`)
)

// goldenStatuses are the terminal JobStatus values a worker-side adapter may
// report (spec/07-state-machine.md transitions out of RUNNING plus CANCELLED).
var goldenStatuses = strings.Fields("JOB_STATUS_SUCCEEDED JOB_STATUS_FAILED " +
	"JOB_STATUS_FAILED_RETRYABLE JOB_STATUS_FAILED_FATAL JOB_STATUS_CANCELLED")

var goldenDeniedKeys = strings.Fields("module import path file url uri command cmd " +
	"shell exec eval script entrypoint endpoint host")

// validateFrameworkGolden strictly decodes raw and enforces every invariant,
// returning the first violation.
func validateFrameworkGolden(raw []byte) (*frameworkGolden, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var g frameworkGolden
	if err := dec.Decode(&g); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if g.SchemaVersion != 1 {
		return nil, fmt.Errorf("schemaVersion = %d, want 1", g.SchemaVersion)
	}
	if strings.TrimSpace(g.Description) == "" || len(g.Cases) == 0 {
		return nil, fmt.Errorf("description and cases are required")
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("decode tree: %w", err)
	}
	if err := scanGoldenTree("$", tree); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for i, c := range g.Cases {
		if seen[c.ID] {
			return nil, fmt.Errorf("cases[%d]: duplicate id %q", i, c.ID)
		}
		seen[c.ID] = true
		if err := validateGoldenCase(c); err != nil {
			return nil, fmt.Errorf("cases[%d] %q: %w", i, c.ID, err)
		}
	}
	return &g, nil
}

// scanGoldenTree walks every key and string value: no denied key name and no
// value that looks like a URL, absolute path or executable file.
func scanGoldenTree(at string, node any) error {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if slices.Contains(goldenDeniedKeys, strings.ToLower(k)) {
				return fmt.Errorf("%s: denied key %q", at, k)
			}
			if err := scanGoldenTree(at+"."+k, child); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range v {
			if err := scanGoldenTree(fmt.Sprintf("%s[%d]", at, i), child); err != nil {
				return err
			}
		}
	case string:
		return checkGoldenString(at, v)
	}
	return nil
}

func checkGoldenString(at, s string) error {
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(s, "://"):
		return fmt.Errorf("%s: URL-like value %q", at, s)
	case strings.HasPrefix(s, "/"), strings.HasPrefix(s, `\`), goldenDriveLetter.MatchString(s):
		return fmt.Errorf("%s: path-like value %q", at, s)
	}
	if slices.ContainsFunc([]string{".py", ".sh", ".exe", ".js"}, func(ext string) bool { return strings.HasSuffix(lower, ext) }) {
		return fmt.Errorf("%s: executable-file value %q", at, s)
	}
	return nil
}

func validateGoldenCase(c goldenCase) error {
	switch {
	case !goldenIDPattern.MatchString(c.ID):
		return fmt.Errorf("id must match %s", goldenIDPattern)
	case strings.TrimSpace(c.Title) == "":
		return fmt.Errorf("title is required")
	case !goldenIDPattern.MatchString(c.Workload):
		return fmt.Errorf("workload %q must be a symbolic name matching %s", c.Workload, goldenIDPattern)
	case c.Input == nil:
		return fmt.Errorf("input is required")
	case c.Expect.Progress == nil:
		return fmt.Errorf("expect.progress is required (use [] for none)")
	}
	for _, m := range c.Expect.Progress {
		if !goldenMarkerPattern.MatchString(m) {
			return fmt.Errorf("progress marker %q must match %s", m, goldenMarkerPattern)
		}
	}
	for _, a := range c.Expect.Artifacts {
		if !goldenIDPattern.MatchString(a.Name) || a.MediaType == "" {
			return fmt.Errorf("artifact %q needs a symbolic name and a mediaType", a.Name)
		}
	}
	if c.Limits != nil && (c.Limits.DeadlineMs < 0 || c.Limits.MaxOutputBytes < 0) {
		return fmt.Errorf("limits must be non-negative")
	}
	return validateGoldenOutcome(c)
}

// validateGoldenOutcome ties status, error code, output and cancel together.
func validateGoldenOutcome(c goldenCase) error {
	e := c.Expect
	if _, ok := agentv1.JobStatus_value[e.Status]; !ok || !slices.Contains(goldenStatuses, e.Status) {
		return fmt.Errorf("expect.status %q is not an allowed terminal JobStatus", e.Status)
	}
	_, knownCode := agentv1.ErrorCode_value[e.ErrorCode]
	failed := strings.HasPrefix(e.Status, "JOB_STATUS_FAILED")
	switch {
	case e.ErrorCode != "" && !knownCode:
		return fmt.Errorf("expect.errorCode %q is not an ErrorCode", e.ErrorCode)
	case e.Status == "JOB_STATUS_SUCCEEDED" && (e.Output == nil || e.ErrorCode != ""):
		return fmt.Errorf("success needs output and no errorCode")
	case failed && (e.ErrorCode == "" || e.Output != nil):
		return fmt.Errorf("failure needs errorCode and no output")
	case e.ErrorCode == "ERROR_CODE_UNSPECIFIED" && e.ErrorString == "":
		return fmt.Errorf("ERROR_CODE_UNSPECIFIED needs errorString (spec/13 backward compatibility)")
	case (c.Cancel != nil) != (e.Status == "JOB_STATUS_CANCELLED"):
		return fmt.Errorf("cancel and JOB_STATUS_CANCELLED must appear together")
	}
	if c.Cancel != nil {
		n := len(e.Progress)
		if n == 0 || e.Progress[n-1] != c.Cancel.AfterProgress {
			return fmt.Errorf("cancel.afterProgress must be the last expected progress marker")
		}
	}
	return nil
}

// goldenCoverage names the behaviours the case set must exercise.
var goldenCoverage = map[string]func(goldenCase) bool{
	"success-with-output": func(c goldenCase) bool { return c.Expect.Status == "JOB_STATUS_SUCCEEDED" },
	"ordered-progress":    func(c goldenCase) bool { return len(c.Expect.Progress) >= 2 },
	"artifact":            func(c goldenCase) bool { return len(c.Expect.Artifacts) > 0 },
	"framework-failure":   func(c goldenCase) bool { return c.Expect.ErrorCode == "ERROR_CODE_UNSPECIFIED" },
	"cancellation":        func(c goldenCase) bool { return c.Cancel != nil },
	"deadline":            func(c goldenCase) bool { return c.Expect.ErrorCode == "ERROR_CODE_JOB_TIMEOUT" },
	"output-bound":        func(c goldenCase) bool { return c.Expect.ErrorCode == "ERROR_CODE_JOB_RESOURCE_EXHAUSTED" },
	"rejected-input":      func(c goldenCase) bool { return c.Expect.ErrorCode == "ERROR_CODE_JOB_INVALID_INPUT" },
	"unregistered":        func(c goldenCase) bool { return c.Expect.ErrorCode == "ERROR_CODE_JOB_NOT_FOUND" },
}

func readFrameworkGolden(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "adapters", "framework-golden-v1.json"))
	if err != nil {
		t.Fatalf("read framework golden file: %v", err)
	}
	return raw
}

func uncoveredGolden(cases []goldenCase) []string {
	var missing []string
	for name, covers := range goldenCoverage {
		if !slices.ContainsFunc(cases, covers) {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func TestFrameworkGoldenFileIsValid(t *testing.T) {
	g, err := validateFrameworkGolden(readFrameworkGolden(t))
	if err != nil {
		t.Fatalf("framework golden file is invalid: %v", err)
	}
	if missing := uncoveredGolden(g.Cases); len(missing) > 0 {
		t.Fatalf("framework golden file does not cover %v", missing)
	}
	// Coverage is live: dropping the cases behind one behaviour must surface it.
	for name, covers := range goldenCoverage {
		rest := slices.DeleteFunc(slices.Clone(g.Cases), covers)
		if !slices.Contains(uncoveredGolden(rest), name) {
			t.Errorf("removing %q cases did not report it uncovered", name)
		}
	}
}

// TestFrameworkGoldenRejectsMutations proves each check is live: every
// in-memory mutation of the shipped file must fail with its named reason.
func TestFrameworkGoldenRejectsMutations(t *testing.T) {
	mutations := []struct {
		name, want string
		mutate     func(root map[string]any, c0, c1 map[string]any)
	}{
		{"duplicate id", "duplicate id", func(_, c0, c1 map[string]any) { c1["id"] = c0["id"] }},
		{"missing workload", "workload", func(_, c0, _ map[string]any) { delete(c0, "workload") }},
		{"module key", `denied key "module"`, func(_, c0, _ map[string]any) { c0["input"].(map[string]any)["module"] = "x" }},
		{"nested host key", `denied key "Host"`, func(_, c0, _ map[string]any) { c0["input"].(map[string]any)["cfg"] = map[string]any{"Host": "x"} }},
		{"url value", "URL-like", func(_, c0, _ map[string]any) { c0["input"].(map[string]any)["text"] = "https://example.invalid" }},
		{"absolute path value", "path-like", func(_, c0, _ map[string]any) { c0["title"] = "/etc/passwd" }},
		{"drive letter value", "path-like", func(_, c0, _ map[string]any) { c0["title"] = `C:\tools` }},
		{"script value", "executable-file", func(_, c0, _ map[string]any) { c0["input"].(map[string]any)["text"] = "run.PY" }},
		{"unknown field", "unknown field", func(_, c0, _ map[string]any) { c0["extra"] = true }},
		{"bad status", "not an allowed terminal JobStatus", func(_, c0, _ map[string]any) { c0["expect"].(map[string]any)["status"] = "JOB_STATUS_DENIED" }},
		{"bad error code", "not an ErrorCode", func(_, c0, _ map[string]any) { c0["expect"].(map[string]any)["errorCode"] = "ERROR_CODE_NOPE" }},
		{"dotted workload", "symbolic name", func(_, c0, _ map[string]any) { c0["workload"] = "pkg.mod" }},
		{"success with error code", "success needs output", func(_, c0, _ map[string]any) { c0["expect"].(map[string]any)["errorCode"] = "ERROR_CODE_JOB_TIMEOUT" }},
		{"cancel without cancelled status", "must appear together", func(_, c0, _ map[string]any) { c0["cancel"] = map[string]any{"afterProgress": "upper"} }},
		{"schema version", "schemaVersion", func(root, _, _ map[string]any) { root["schemaVersion"] = 2 }},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			var root map[string]any
			if err := json.Unmarshal(readFrameworkGolden(t), &root); err != nil {
				t.Fatalf("decode: %v", err)
			}
			cases := root["cases"].([]any)
			m.mutate(root, cases[0].(map[string]any), cases[1].(map[string]any))
			raw, err := json.Marshal(root)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			_, err = validateFrameworkGolden(raw)
			if err == nil || !strings.Contains(err.Error(), m.want) {
				t.Fatalf("mutation %q: got error %v, want one containing %q", m.name, err, m.want)
			}
		})
	}
}
