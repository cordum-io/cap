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

func shippedScenarioIDs(t *testing.T) map[string]Scenario {
	t.Helper()
	all := map[string]Scenario{}
	for _, name := range []string{"core-lifecycle.json", "core-cancellation.json"} {
		for _, sc := range loadShippedSuite(t, name).Scenarios {
			all[sc.ID] = sc
		}
	}
	return all
}

func TestShippedScenarioSuitesAreValid(t *testing.T) {
	life := loadShippedSuite(t, "core-lifecycle.json")
	canc := loadShippedSuite(t, "core-cancellation.json")
	if len(life.Scenarios) == 0 || len(canc.Scenarios) == 0 {
		t.Fatalf("shipped suites must be non-empty: lifecycle=%d cancellation=%d", len(life.Scenarios), len(canc.Scenarios))
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
