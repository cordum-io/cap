package tck

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func scenariosDir() string { return filepath.Join("..", "..", "spec", "tck", "scenarios") }

func loadShippedSuite(t *testing.T, name string) *Suite {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(scenariosDir(), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	s, err := LoadSuite(b)
	if err != nil {
		t.Fatalf("shipped suite %s is invalid: %v", name, err)
	}
	return s
}

// shippedSuiteFiles is the full set of scenario suites the repo ships.
var shippedSuiteFiles = []string{
	"core-lifecycle.json",
	"core-cancellation.json",
	"core-negotiation.json",
	"core-malformed.json",
	"core-safety.json",
	"core-duplicates.json",
	"core-retries.json",
}

func shippedScenarioIDs(t *testing.T) map[string]Scenario {
	t.Helper()
	all := map[string]Scenario{}
	for _, name := range shippedSuiteFiles {
		for _, sc := range loadShippedSuite(t, name).Scenarios {
			if _, dup := all[sc.ID]; dup {
				t.Fatalf("scenario id %q appears in more than one shipped suite", sc.ID)
			}
			all[sc.ID] = sc
		}
	}
	return all
}

func TestShippedScenarioSuitesAreValid(t *testing.T) {
	for _, name := range shippedSuiteFiles {
		if s := loadShippedSuite(t, name); len(s.Scenarios) == 0 {
			t.Fatalf("shipped suite %s must be non-empty", name)
		}
	}
}

// Every malformed/bounds scenario must demand a drop with no side effect, and
// every safety-gate scenario must forbid dispatch or record ALLOW first — the
// two properties this step exists to enforce.
func TestMalformedAndSafetyScenariosDeclareExpectations(t *testing.T) {
	ids := shippedScenarioIDs(t)
	var malformed, safety int
	for id, sc := range ids {
		switch {
		case len(id) >= 10 && id[:10] == "malformed/":
			malformed++
			if sc.Params["expect"] != "drop" {
				t.Fatalf("malformed scenario %q must expect drop, got %q", id, sc.Params["expect"])
			}
		case len(id) >= 7 && id[:7] == "safety/":
			safety++
			if sc.Params["decision"] == "" {
				t.Fatalf("safety scenario %q must name a decision", id)
			}
		}
	}
	if malformed < 8 || safety < 4 {
		t.Fatalf("expected >=8 malformed and >=4 safety scenarios, got %d and %d", malformed, safety)
	}
}

// Every terminal status the state machine defines must be reachable by some
// shipped scenario, or conformance would silently skip a terminal outcome.
func TestShippedScenariosReachEveryTerminalStatus(t *testing.T) {
	terminal := []string{"SUCCEEDED", "FAILED", "FAILED_FATAL", "TIMEOUT", "DENIED", "CANCELLED"}
	reached := map[string]bool{}
	for _, sc := range shippedScenarioIDs(t) {
		if to := sc.Params["to"]; to != "" && sc.Params["expect"] == "" {
			reached[to] = true
		}
	}
	for _, term := range terminal {
		if !reached[term] {
			t.Fatalf("no shipped scenario reaches terminal status %s", term)
		}
	}
}

func TestShippedCriticalRejectionScenariosPresent(t *testing.T) {
	ids := shippedScenarioIDs(t)
	for _, want := range []string{
		"lifecycle/worker-rejects-backwards-transition",
		"lifecycle/worker-rejects-second-terminal",
		"lifecycle/cp-rejects-backwards-transition",
		"cancellation/cp-rejects-cancel-after-terminal",
	} {
		sc, ok := ids[want]
		if !ok {
			t.Fatalf("missing required rejection scenario %q", want)
		}
		if sc.Params["expect"] != "reject" && sc.Params["expect"] != "ignore" {
			t.Fatalf("scenario %q must declare a rejection expectation, got %q", want, sc.Params["expect"])
		}
	}
}

type coverageDoc struct {
	Behaviors []struct {
		Behavior string   `json:"behavior"`
		Gated    string   `json:"gated"`
		Reason   string   `json:"reason"`
		Cases    []string `json:"cases"`
	} `json:"behaviors"`
}

// The coverage index must reference only real cases, and every gated behavior
// must be explicit (marked gated, with a reason, and no cases) so a deferred
// behavior can never masquerade as covered.
func TestCoverageIndexIsConsistent(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "spec", "tck", "coverage.json"))
	if err != nil {
		t.Fatalf("read coverage.json: %v", err)
	}
	var cov coverageDoc
	if err := json.Unmarshal(b, &cov); err != nil {
		t.Fatalf("coverage.json invalid: %v", err)
	}
	ids := shippedScenarioIDs(t)
	sawGated := false
	for _, bh := range cov.Behaviors {
		if bh.Gated != "" {
			sawGated = true
			if len(bh.Cases) != 0 || bh.Reason == "" {
				t.Fatalf("gated behavior %q must have a reason and no cases", bh.Behavior)
			}
			continue
		}
		if len(bh.Cases) == 0 {
			t.Fatalf("non-gated behavior %q lists no cases", bh.Behavior)
		}
		for _, id := range bh.Cases {
			if _, ok := ids[id]; !ok {
				t.Fatalf("coverage behavior %q references unknown case %q", bh.Behavior, id)
			}
		}
	}
	if !sawGated {
		t.Fatal("expected coverage.json to explicitly record the gated CAP-PRODUCTION behaviors")
	}
}

// DoD 1 remainder: the CAP-PRODUCTION DispatchIdentity{dispatch_id, attempt,
// assigned_worker_id} model is now merged to main, so duplicate-suppression and
// retry-attempt-fencing must be REAL shipped coverage — not gated placeholders
// with empty cases. Every listed case must resolve to a shipped scenario.
func TestDuplicateAndRetryCoverageIsShippedNotGated(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "spec", "tck", "coverage.json"))
	if err != nil {
		t.Fatalf("read coverage.json: %v", err)
	}
	var cov coverageDoc
	if err := json.Unmarshal(b, &cov); err != nil {
		t.Fatalf("coverage.json invalid: %v", err)
	}
	ids := shippedScenarioIDs(t)
	seen := map[string]bool{
		"exact-redelivery duplicate suppression":                                         false,
		"retry attempt fencing (monotonic attempt, stale/future/wrong-worker rejection)": false,
	}
	for _, bh := range cov.Behaviors {
		if _, tracked := seen[bh.Behavior]; !tracked {
			continue
		}
		seen[bh.Behavior] = true
		if bh.Gated != "" {
			t.Errorf("behavior %q is still gated on %q, but the CAP-PRODUCTION model is merged", bh.Behavior, bh.Gated)
			continue
		}
		if len(bh.Cases) == 0 {
			t.Errorf("behavior %q lists no cases", bh.Behavior)
		}
		for _, id := range bh.Cases {
			if _, ok := ids[id]; !ok {
				t.Errorf("behavior %q references unknown case %q", bh.Behavior, id)
			}
		}
	}
	for behavior, found := range seen {
		if !found {
			t.Errorf("coverage.json no longer records behavior %q", behavior)
		}
	}
}

// NON-VACUITY (DoD 1), duplicate-suppression + retry/cancel fencing, driven
// end-to-end through the real adapter protocol and runner. The reference model
// (refadapter_test.go) must grade the suites conformant; a mutation that disables
// the specific invariant must make the suite non-conformant with real failures.
// Because the reference computes each outcome from the scenario's INPUT params,
// a case whose semantic params were gutted can no longer pass — so this cannot be
// satisfied by an adapter that blindly passes or blindly fails.
func TestDuplicateAndRetrySuitesAreNonVacuous(t *testing.T) {
	for _, suiteName := range []string{"core-duplicates.json", "core-retries.json"} {
		suite := loadShippedSuite(t, suiteName)
		passes := 0
		for _, role := range []Role{RoleWorker, RoleControlPlane} {
			ref := helperAdapterRole("reference", role)
			if _, err := ref.Start(context.Background()); err != nil {
				t.Fatalf("%s: start reference: %v", suiteName, err)
			}
			rep := fixedClockRunner().Run(context.Background(), ref, suite, "cap-production")
			ref.Close()
			if !rep.Summary.Conformant {
				t.Errorf("%s/%s: reference model must be conformant (summary=%+v)", suiteName, role, rep.Summary)
			}
			passes += rep.Summary.Pass
		}
		if passes == 0 {
			t.Errorf("%s: reference graded no cases", suiteName)
		}
	}
	// Each invariant, when disabled, must surface as real failures.
	checks := map[string][]string{
		"core-duplicates.json": {"dedup"},
		"core-retries.json":    {"fence", "worker", "cancel"},
	}
	for suiteName, muts := range checks {
		suite := loadShippedSuite(t, suiteName)
		for _, mut := range muts {
			nonConformant, fails := false, 0
			for _, role := range []Role{RoleWorker, RoleControlPlane} {
				bad := helperAdapterRole("mut:"+mut, role)
				if _, err := bad.Start(context.Background()); err != nil {
					t.Fatalf("%s/%s: start mutation: %v", suiteName, mut, err)
				}
				rep := fixedClockRunner().Run(context.Background(), bad, suite, "cap-production")
				bad.Close()
				if !rep.Summary.Conformant {
					nonConformant = true
				}
				fails += rep.Summary.Fail
			}
			if !nonConformant || fails == 0 {
				t.Errorf("%s: mutation %q produced no failures; the suite is vacuous", suiteName, mut)
			}
		}
	}
}

// The shipped lifecycle suite must run through the real runner: a worker
// adapter grades the worker cases and the control-plane cases become
// non-applicable N/A rather than false passes.
func TestShippedLifecycleSuiteRunsThroughRunner(t *testing.T) {
	suite := loadShippedSuite(t, "core-lifecycle.json")
	a := helperAdapter("well-behaved")
	if _, err := a.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer a.Close()
	rep := fixedClockRunner().Run(context.Background(), a, suite, "core")
	if rep.Summary.Pass == 0 {
		t.Fatalf("expected some worker cases to run; summary=%+v", rep.Summary)
	}
	if rep.Summary.NA == 0 {
		t.Fatalf("expected control-plane cases to be N/A for a worker adapter; summary=%+v", rep.Summary)
	}
	if !rep.Summary.Conformant {
		t.Fatalf("a well-behaved worker adapter should be conformant; summary=%+v", rep.Summary)
	}
}

// runReferenceModeOverAllSuites drives one reference-adapter mode across every
// shipped suite in both roles and returns each applicable case's verdict. Role
// mismatches (N/A) are skipped, so the map holds exactly the graded verdicts.
func runReferenceModeOverAllSuites(t *testing.T, mode string) map[string]Status {
	t.Helper()
	verdict := map[string]Status{}
	for _, role := range []Role{RoleWorker, RoleControlPlane} {
		a := helperAdapterRole(mode, role)
		if _, err := a.Start(context.Background()); err != nil {
			t.Fatalf("start %s adapter as %s: %v", mode, role, err)
		}
		for _, suiteName := range shippedSuiteFiles {
			suite := loadShippedSuite(t, suiteName)
			for _, profile := range suite.Profiles() {
				rep := fixedClockRunner().Run(context.Background(), a, suite, profile)
				for _, c := range rep.Cases {
					if c.Status != StatusNA {
						verdict[c.ID] = c.Status
					}
				}
			}
		}
		a.Close()
	}
	return verdict
}

// NON-VACUITY, EVERY SHIPPED SUITE (DoD 1). The already-delivered lifecycle /
// cancellation / negotiation / malformed / safety scenarios had no non-vacuity
// proof, so a suite that graded nothing would look as green as one that graded
// everything. This drives the reference model and every single-check mutation
// end-to-end through the real adapter protocol and runner, and asserts:
//   - the reference PASSes every shipped case (nothing is ungraded or N/A-laundered);
//   - each mutation breaks exactly the cases that depend on its check and leaves
//     every other case green (selectivity — proving the checks are real and distinct).
func TestEveryShippedSuiteIsNonVacuous(t *testing.T) {
	all := shippedScenarioIDs(t)

	ref := runReferenceModeOverAllSuites(t, "reference")
	graded := 0
	for id := range all {
		st, ok := ref[id]
		if !ok {
			t.Errorf("reference adapter never graded shipped case %q", id)
			continue
		}
		graded++
		if st != StatusPass {
			t.Errorf("reference adapter did not PASS %q end-to-end: got %s", id, st)
		}
	}
	if graded != len(all) {
		t.Errorf("reference graded %d cases, want %d", graded, len(all))
	}

	for mut, redden := range mutationReddens {
		reddenSet := map[string]bool{}
		for _, id := range redden {
			reddenSet[id] = true
		}
		verdict := runReferenceModeOverAllSuites(t, "mut:"+mut)
		broke := 0
		for id := range all {
			st, ok := verdict[id]
			if !ok {
				t.Errorf("mutation %q never graded %q", mut, id)
				continue
			}
			switch {
			case reddenSet[id]:
				if st == StatusPass {
					t.Errorf("mutation %q must break %q end-to-end but it PASSED", mut, id)
				} else {
					broke++
				}
			case st != StatusPass:
				t.Errorf("mutation %q wrongly broke unrelated case %q: %s", mut, id, st)
			}
		}
		if broke != len(reddenSet) {
			t.Errorf("mutation %q broke %d cases end-to-end, want %d", mut, broke, len(reddenSet))
		}
	}
	t.Logf("non-vacuity proven: reference PASSes %d cases; %d mutations each break their invariant selectively",
		graded, len(mutationReddens))
}
