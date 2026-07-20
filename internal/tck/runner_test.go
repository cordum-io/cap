package tck

import (
	"context"
	"testing"
	"time"
)

func fixedClockRunner() *Runner {
	r := NewRunner()
	r.now = func() time.Time { return time.Unix(0, 0) }
	return r
}

func caseByID(rep Report, id string) (CaseResult, bool) {
	for _, c := range rep.Cases {
		if c.ID == id {
			return c, true
		}
	}
	return CaseResult{}, false
}

const runnerSuite = `{"version":1,"scenarios":[
  {"id":"w/pass","role":"worker","profile":"core","required":true,"params":{"status":"PASS"}},
  {"id":"w/fail","role":"worker","profile":"core","required":true,"params":{"status":"FAIL"}},
  {"id":"cp/only","role":"control-plane","profile":"core","required":true}
]}`

func TestRunnerGradesApplicableCasesAndSkipsForeignRole(t *testing.T) {
	suite, err := LoadSuite([]byte(runnerSuite))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	a := helperAdapter("echo-status")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()

	rep := fixedClockRunner().Run(context.Background(), a, suite, "core")

	pass, _ := caseByID(rep, "w/pass")
	if !pass.Applicable || pass.Status != StatusPass {
		t.Fatalf("w/pass: %+v", pass)
	}
	fail, _ := caseByID(rep, "w/fail")
	if !fail.Applicable || fail.Status != StatusFail {
		t.Fatalf("w/fail: %+v", fail)
	}
	cp, _ := caseByID(rep, "cp/only")
	if cp.Applicable || cp.Status != StatusNA {
		t.Fatalf("cp/only should be non-applicable N/A: %+v", cp)
	}
	if rep.Summary.Conformant {
		t.Fatal("run must be non-conformant: a required applicable case FAILed")
	}
	if rep.Summary.Pass != 1 || rep.Summary.Fail != 1 || rep.Summary.NA != 1 {
		t.Fatalf("unexpected summary: %+v", rep.Summary)
	}
}

func TestRunnerConformantWhenAllApplicableRequiredPass(t *testing.T) {
	suite, _ := LoadSuite([]byte(`{"version":1,"scenarios":[
      {"id":"w/pass","role":"worker","profile":"core","required":true,"params":{"status":"PASS"}},
      {"id":"cp/only","role":"control-plane","profile":"core","required":true}]}`))
	a := helperAdapter("echo-status")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	rep := fixedClockRunner().Run(context.Background(), a, suite, "core")
	if !rep.Summary.Conformant {
		t.Fatalf("expected conformant; summary=%+v", rep.Summary)
	}
}

// A hung adapter must surface as ERROR on the case, not hang the runner or pass.
func TestRunnerRecordsErrorOnTimeout(t *testing.T) {
	suite, _ := LoadSuite([]byte(`{"version":1,"scenarios":[
      {"id":"w/x","role":"worker","profile":"core","required":true}]}`))
	a := helperAdapter("hang-run")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	r := fixedClockRunner()
	r.CaseDeadline = 300 * time.Millisecond
	rep := r.Run(context.Background(), a, suite, "core")
	c, _ := caseByID(rep, "w/x")
	if c.Status != StatusError {
		t.Fatalf("expected ERROR on timeout, got %+v", c)
	}
	if rep.Summary.Conformant {
		t.Fatal("a timed-out required case must make the run non-conformant")
	}
}
