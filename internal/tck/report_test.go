package tck

import (
	"bytes"
	"strings"
	"testing"
)

func sampleCases() []CaseResult {
	return []CaseResult{
		{ID: "b/two", Profile: "core", Role: RoleWorker, Required: true, Applicable: true, Status: StatusPass, DurationMs: 5},
		{ID: "a/one", Profile: "core", Role: RoleWorker, Required: true, Applicable: true, Status: StatusFail, Detail: "bad transition", DurationMs: 3},
		{ID: "c/three", Profile: "core", Role: RoleControlPlane, Required: true, Applicable: false, Status: StatusNA, DurationMs: 0},
	}
}

func sampleHS() AdapterMessage {
	return AdapterMessage{Type: MsgHandshake, SDK: "go", Role: RoleWorker, Transport: "nats"}
}

func TestNewReportSortsCasesAndCountsSummary(t *testing.T) {
	r := NewReport("go-adapter", sampleHS(), "core", "tck-test", sampleCases())
	if got := []string{r.Cases[0].ID, r.Cases[1].ID, r.Cases[2].ID}; got[0] != "a/one" || got[1] != "b/two" || got[2] != "c/three" {
		t.Fatalf("cases not sorted by id: %v", got)
	}
	if r.Summary.Total != 3 || r.Summary.Pass != 1 || r.Summary.Fail != 1 || r.Summary.NA != 1 {
		t.Fatalf("unexpected summary: %+v", r.Summary)
	}
}

// A required applicable FAIL must make the run non-conformant; a non-applicable
// N/A must not.
func TestNewReportConformanceIgnoresNonApplicableButFailsRequired(t *testing.T) {
	failing := NewReport("go", sampleHS(), "core", "v", sampleCases())
	if failing.Summary.Conformant {
		t.Fatal("expected non-conformant: a required applicable case FAILed")
	}

	allPass := []CaseResult{
		{ID: "a", Profile: "core", Role: RoleWorker, Required: true, Applicable: true, Status: StatusPass},
		{ID: "z", Profile: "core", Role: RoleControlPlane, Required: true, Applicable: false, Status: StatusNA},
	}
	ok := NewReport("go", sampleHS(), "core", "v", allPass)
	if !ok.Summary.Conformant {
		t.Fatalf("expected conformant: only non-applicable N/A remained; summary=%+v", ok.Summary)
	}
}

// An applicable required case the adapter itself reports N/A for must fail: the
// suite cannot pass by declining a case it was asked to run.
func TestNewReportAdapterReportedNAFailsRequired(t *testing.T) {
	cases := []CaseResult{{ID: "a", Profile: "core", Role: RoleWorker, Required: true, Applicable: true, Status: StatusNA}}
	if NewReport("go", sampleHS(), "core", "v", cases).Summary.Conformant {
		t.Fatal("expected non-conformant: adapter reported N/A for an applicable required case")
	}
}

func TestReportJSONIsDeterministic(t *testing.T) {
	a := NewReport("go", sampleHS(), "core", "v", sampleCases())
	b := NewReport("go", sampleHS(), "core", "v", sampleCases())
	ja, err := a.JSON()
	if err != nil {
		t.Fatalf("json a: %v", err)
	}
	jb, err := b.JSON()
	if err != nil {
		t.Fatalf("json b: %v", err)
	}
	if !bytes.Equal(ja, jb) {
		t.Fatalf("JSON not deterministic:\nA=%s\nB=%s", ja, jb)
	}
	if !strings.Contains(string(ja), `"conformant": false`) {
		t.Fatalf("expected conformant:false in output: %s", ja)
	}
}

// Adapter-controlled text must be escaped, never able to inject XML structure.
func TestReportJUnitEscapesDetail(t *testing.T) {
	cases := []CaseResult{
		{ID: "x", Profile: "core", Role: RoleWorker, Required: true, Applicable: true,
			Status: StatusFail, Detail: `</testcase><inject a="b">& boom`},
	}
	r := NewReport("go", sampleHS(), "core", "v", cases)
	out, err := r.JUnit()
	if err != nil {
		t.Fatalf("junit: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "<inject") {
		t.Fatalf("unescaped injection reached XML: %s", s)
	}
	if !strings.Contains(s, "&lt;/testcase&gt;") && !strings.Contains(s, "&lt;inject") {
		t.Fatalf("expected escaped detail in XML: %s", s)
	}
	if !strings.Contains(s, `failures="1"`) {
		t.Fatalf("expected failures=1 in junit: %s", s)
	}
}

// A required, applicable case the adapter reports N/A for must be a JUnit
// <failure>, not <skipped> — the JUnit artifact must not launder the exact
// declined-hardest-case loophole the exit code already catches.
func TestReportJUnitRequiredNAIsFailureNotSkipped(t *testing.T) {
	cases := []CaseResult{
		{ID: "req-na", Profile: "core", Role: RoleWorker, Required: true, Applicable: true, Status: StatusNA},
	}
	out, err := NewReport("go", sampleHS(), "core", "v", cases).JUnit()
	if err != nil {
		t.Fatalf("junit: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `failures="1"`) || strings.Contains(s, `skipped="1"`) {
		t.Fatalf("required applicable N/A must be a failure, not skipped: %s", s)
	}
	if !strings.Contains(s, "<failure") {
		t.Fatalf("expected a <failure> element: %s", s)
	}
}

// An optional (non-required) N/A the adapter legitimately declines stays a skip.
func TestReportJUnitOptionalNAStaysSkipped(t *testing.T) {
	cases := []CaseResult{
		{ID: "opt-na", Profile: "core", Role: RoleWorker, Required: false, Applicable: true, Status: StatusNA},
	}
	out, _ := NewReport("go", sampleHS(), "core", "v", cases).JUnit()
	if !strings.Contains(string(out), `skipped="1"`) {
		t.Fatalf("optional N/A should be skipped: %s", out)
	}
}

func TestReportJUnitMarksNonApplicableSkipped(t *testing.T) {
	r := NewReport("go", sampleHS(), "core", "v", sampleCases())
	out, _ := r.JUnit()
	if !strings.Contains(string(out), "<skipped") {
		t.Fatalf("expected a skipped element for the non-applicable case: %s", out)
	}
}
