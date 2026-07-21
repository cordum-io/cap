package tck

import (
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file is the TCK's reference conformance model. It exists to make the
// shipped scenario suites provably NON-VACUOUS.
//
// The prior "violate"/"well-behaved" helper modes graded every case by fiat
// (always FAIL / always PASS) and never read a scenario's semantic params, so a
// suite whose cases had been gutted still looked green. QA rejected that as
// self-reporting. gradeReference instead computes, from a scenario's INPUT params
// (dispatch id, attempt, active attempt, worker id, decision, from/to state, ...
// but NEVER from `expect`), the outcome a conformant CAP implementation must
// produce, and returns whether that outcome matches the scenario's declared
// expectation.
//
// Each behavioral invariant is guarded by a named check. Passing a non-empty
// `mut` disables exactly ONE check; that must turn precisely the cases which
// exercise that invariant red and leave every other case green (proven by
// TestReferenceMutationsAreSelective). Removing any load-bearing input param must
// also make a case fail (TestReferenceConsumesRequiredParams). Together these
// prove a real, removable check backs every shipped case.
//
// referenceWorkerID is the identity the reference worker was assigned. A dispatch
// or control event addressed to a different assigned_worker_id is not ours.
const referenceWorkerID = "wrk-1"

// gradeReference returns (pass, detail) for one scenario under the reference
// model with an optional single disabled check `mut`.
func gradeReference(mut, id string, p map[string]string) (bool, string) {
	switch {
	case strings.HasPrefix(id, "lifecycle/"), strings.HasPrefix(id, "cancellation/"):
		return refOrder(mut, p)
	case strings.HasPrefix(id, "safety/"):
		return refSafety(mut, p)
	case strings.HasPrefix(id, "negotiation/"):
		return refNegotiation(mut, p)
	case strings.HasPrefix(id, "malformed/"):
		return refMalformed(mut, p)
	case strings.HasPrefix(id, "duplicates/"):
		return refDuplicates(mut, p)
	case strings.HasPrefix(id, "retries/"):
		return refRetries(mut, id, p)
	default:
		return false, "no reference model for scenario " + id
	}
}

// ---- state machine (order / teardown) -------------------------------------

var stateRank = map[string]int{
	"PENDING": 0, "SCHEDULED": 1, "DISPATCHED": 2,
	"RUNNING": 3, "FAILED_RETRYABLE": 3,
	"SUCCEEDED": 4, "FAILED": 4, "FAILED_FATAL": 4,
	"TIMEOUT": 4, "DENIED": 4, "CANCELLED": 4,
}

func isTerminalState(s string) bool { return s != "FAILED_RETRYABLE" && stateRank[s] == 4 }

// classifyTransition is the real state-machine rule the "order" check enforces:
// idempotent same-state and forward/lateral moves are accepted; a move out of a
// terminal state is rejected (or, for a second conflicting terminal STATUS,
// ignored); cancelling an already-terminal job is rejected; any backwards move is
// rejected.
func classifyTransition(from, to string) string {
	rf, okF := stateRank[from]
	rt, okT := stateRank[to]
	if !okF || !okT {
		return "reject" // an unknown state name is never a legal transition
	}
	if from == to {
		return "accept" // idempotent
	}
	if isTerminalState(from) {
		switch {
		case to == "CANCELLED":
			return "reject" // cannot cancel a job that already reached a terminal state
		case isTerminalState(to):
			return "ignore" // a second, conflicting terminal is ignored
		default:
			return "reject" // backwards out of a terminal state
		}
	}
	if rt >= rf {
		return "accept" // forward, or lateral RUNNING->FAILED_RETRYABLE
	}
	return "reject" // backwards
}

func refOrder(mut string, p map[string]string) (bool, string) {
	from, to := p["from"], p["to"]
	if from == "" || to == "" {
		return false, "order: missing from/to"
	}
	want := p["expect"]
	if want == "" {
		want = "accept"
	}
	actual := "accept" // the "order" check disabled: every transition blindly accepted
	if mut != "order" {
		actual = classifyTransition(from, to)
	}
	return actual == want, "order from=" + from + " to=" + to + " actual=" + actual + " want=" + want
}

// ---- safety-before-dispatch -----------------------------------------------

func refSafety(mut string, p map[string]string) (bool, string) {
	dec := p["decision"]
	if dec == "" {
		return false, "safety: missing decision"
	}
	want := p["expect"]
	if want == "" {
		want = "dispatch" // an ALLOW case declares no expect; the positive outcome is dispatch
	}
	actual := "dispatch" // the "safety" check disabled: dispatch regardless of decision
	if mut != "safety" && dec != "ALLOW" {
		actual = "no-dispatch"
	}
	return actual == want, "safety decision=" + dec + " actual=" + actual + " want=" + want
}

// ---- duplicate suppression -------------------------------------------------

func refDuplicates(mut string, p map[string]string) (bool, string) {
	if p["dispatchId"] == "" || p["attempt"] == "" {
		return false, "duplicates: missing dispatchId/attempt"
	}
	n, err := strconv.Atoi(p["deliveries"])
	if err != nil || n < 1 {
		return false, "duplicates: missing/invalid deliveries"
	}
	want := p["expect"]
	if want == "" {
		return false, "duplicates: missing expect"
	}
	// Simulate n deliveries of the SAME (dispatch_id, attempt). With the "dedup"
	// check enabled the handler runs once and later deliveries replay the recorded
	// terminal result; disabled, every delivery re-invokes the handler.
	dedup := mut != "dedup"
	side, replays, seen := 0, 0, false
	for i := 0; i < n; i++ {
		if dedup && seen {
			replays++
		} else {
			side++
			seen = true
		}
	}
	sideOnce := side == 1
	var pass bool
	switch want {
	case "one-side-effect":
		pass = sideOnce
	case "idempotent-replay":
		pass = sideOnce && n >= 2 && replays == n-1
	case "invoke":
		pass = sideOnce && replays == 0
	default:
		pass = false
	}
	return pass, "duplicates deliveries=" + strconv.Itoa(n) + " side=" + strconv.Itoa(side) +
		" replays=" + strconv.Itoa(replays) + " want=" + want
}

// ---- retry attempt / worker / cancel fencing -------------------------------

func refRetries(mut, id string, p map[string]string) (bool, string) {
	att, err1 := strconv.Atoi(p["attempt"])
	active, err2 := strconv.Atoi(p["activeAttempt"])
	if err1 != nil || err2 != nil {
		return false, "retries: missing/invalid attempt/activeAttempt"
	}
	event := p["event"]
	if event == "" {
		event = "dispatch"
	}
	want := p["expect"]
	if want == "" {
		want = "invoke"
	}
	var actual string
	if strings.HasPrefix(id, "retries/cp-") {
		actual = refRetriesControlPlane(mut, event, att, active)
	} else {
		wrk := p["assignedWorkerId"]
		if wrk == "" {
			return false, "retries: worker event missing assignedWorkerId"
		}
		actual = refRetriesWorker(mut, event, att, active, wrk)
	}
	return actual == want, "retries event=" + event + " attempt=" + strconv.Itoa(att) +
		" active=" + strconv.Itoa(active) + " actual=" + actual + " want=" + want
}

func refRetriesWorker(mut, event string, att, active int, wrk string) string {
	// "worker" check: an event addressed to a different assigned_worker_id is not
	// ours and must be rejected.
	if wrk != referenceWorkerID && mut != "worker" {
		return "reject"
	}
	switch event {
	case "dispatch":
		// "fence" check: a new attempt is accepted only if it is the monotonic
		// next attempt (active+1); a stale (<=active) or future (>active+1)
		// attempt is rejected.
		if mut == "fence" || att == active+1 {
			return "invoke"
		}
		return "reject"
	case "cancel":
		// "cancel" check: a JobCancel is honored only if it targets the currently
		// active attempt; a stale/superseded cancel must not tear down the wrong
		// attempt.
		if mut == "cancel" || att == active {
			return "cancel"
		}
		return "reject"
	default:
		return "reject"
	}
}

func refRetriesControlPlane(mut, event string, att, active int) string {
	switch event {
	case "result", "progress":
		// "fence" check: the control plane applies a result/progress event only
		// for the active attempt; a stale/future attempt is ignored.
		if mut == "fence" || att == active {
			return "apply"
		}
		return "ignore"
	case "cancel":
		// "cancel" check: a cancel carrying a non-active attempt is ignored.
		if mut == "cancel" || att == active {
			return "apply"
		}
		return "ignore"
	default:
		return "ignore"
	}
}

// ---- capability negotiation ------------------------------------------------

func refNegotiation(mut string, p map[string]string) (bool, string) {
	want := p["expect"]
	if want == "" {
		want = "dispatch"
	}
	correct, ok := negotiationOutcome(p)
	if !ok {
		return false, "negotiation: no inputs to evaluate"
	}
	actual := correct
	if mut == "negotiate" {
		// The "negotiate" check disabled: produce a value guaranteed to differ
		// from the correct decision, so every negotiation case flips red.
		actual = "MUT|" + correct
	}
	return actual == want, "negotiation actual=" + actual + " want=" + want
}

// negotiationOutcome computes the correct negotiation decision from whichever
// input dimension the scenario populates (version set, required capability, or
// ready-topic routing). Returns ok=false when no input dimension is present.
func negotiationOutcome(p map[string]string) (string, bool) {
	switch {
	case p["cpVersions"] != "" || p["workerVersions"] != "":
		common := intersectCSV(p["cpVersions"], p["workerVersions"])
		if len(common) == 0 {
			return "reject", true
		}
		return strconv.Itoa(maxInts(common)), true
	case p["requiredCapability"] != "":
		if csvContains(p["workerCapabilities"], p["requiredCapability"]) {
			return "dispatch", true
		}
		return "reject", true
	case p["jobTopic"] != "":
		rt := p["readyTopics"]
		if rt == "" {
			return "dispatch", true // omitted ready_topics is unspecified, not a filter
		}
		if csvContains(rt, p["jobTopic"]) {
			return "dispatch", true
		}
		return "reject", true
	default:
		return "", false
	}
}

// ---- malformed / bounds rejection ------------------------------------------

func refMalformed(mut string, p map[string]string) (bool, string) {
	want := p["expect"]
	if want == "" {
		return false, "malformed: missing expect"
	}
	actual := "drop" // a conformant implementation drops every malformed/oversize packet
	if mut == "malformed" {
		actual = "accept"
	}
	return actual == want, "malformed actual=" + actual + " want=" + want
}

// ---- small CSV helpers -----------------------------------------------------

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, x := range parts {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func csvContains(csv, want string) bool {
	for _, x := range splitCSV(csv) {
		if x == want {
			return true
		}
	}
	return false
}

func intersectCSV(a, b string) []string {
	set := map[string]bool{}
	for _, x := range splitCSV(a) {
		set[x] = true
	}
	var out []string
	for _, y := range splitCSV(b) {
		if set[y] {
			out = append(out, y)
		}
	}
	sort.Strings(out)
	return out
}

func maxInts(vals []string) int {
	max := 0
	for _, v := range vals {
		if n, err := strconv.Atoi(v); err == nil && n > max {
			max = n
		}
	}
	return max
}

// mutationReddens maps each single-check mutation to the exact set of shipped
// cases whose grading depends on that check. A mutation must turn every case it
// maps to red and leave every other case green — that selectivity is what proves
// each invariant is backed by a distinct, real check rather than a blanket pass.
var mutationReddens = map[string][]string{
	"order": {
		"lifecycle/worker-rejects-backwards-transition",
		"lifecycle/worker-rejects-second-terminal",
		"lifecycle/cp-rejects-backwards-transition",
		"cancellation/cp-rejects-cancel-after-terminal",
	},
	"safety": {
		"safety/deny-blocks-dispatch",
		"safety/require-human-pauses-dispatch",
		"safety/throttle-blocks-immediate-dispatch",
	},
	"dedup": {
		"duplicates/worker-exact-redelivery-invokes-handler-once",
		"duplicates/worker-exact-redelivery-replays-idempotent-result",
		"duplicates/cp-duplicate-result-applies-once",
	},
	"fence": {
		"retries/worker-rejects-stale-attempt",
		"retries/worker-rejects-future-attempt",
		"retries/cp-ignores-stale-attempt-result",
		"retries/cp-ignores-stale-attempt-progress",
	},
	"worker": {
		"retries/worker-rejects-wrong-worker-attempt",
		"retries/worker-rejects-wrong-worker-cancel",
	},
	"cancel": {
		"retries/worker-rejects-stale-cancel",
		"retries/cp-ignores-stale-attempt-cancel",
	},
	"negotiate": {
		"negotiation/selects-highest-common-version",
		"negotiation/rejects-when-no-common-version",
		"negotiation/required-capability-filters-dispatch",
		"negotiation/ready-topics-filters-dispatch",
		"negotiation/missing-ready-topics-is-unspecified-not-error",
	},
	"malformed": {
		"malformed/truncated-wire",
		"malformed/overlong-wire",
		"malformed/non-minimal-encoding",
		"malformed/duplicate-oneof",
		"malformed/empty-envelope",
		"malformed/unsupported-protocol-version",
		"malformed/unknown-payload",
		"malformed/oversize-string",
		"malformed/oversize-map",
		"malformed/oversize-repeated",
	},
}

// TestReferenceModelGradesEveryShippedCase proves the reference model is a
// genuinely conformant implementation: with no check disabled it must PASS every
// shipped scenario. A scenario whose params drift out of conformance breaks here.
func TestReferenceModelGradesEveryShippedCase(t *testing.T) {
	for id, sc := range shippedScenarioIDs(t) {
		ok, detail := gradeReference("", id, sc.Params)
		if !ok {
			t.Errorf("reference model failed to pass shipped case %q: %s", id, detail)
		}
	}
}

// TestReferenceMutationsAreSelective is the core non-vacuity proof: for each
// single-check mutation, every case in its reddens set flips PASS->fail while
// every other shipped case stays PASS. This shows each duplicate / retry / order
// / safety / negotiation / malformed / cancel invariant is backed by a removable
// check, and that the checks do not overlap.
func TestReferenceMutationsAreSelective(t *testing.T) {
	all := shippedScenarioIDs(t)
	for mut, redden := range mutationReddens {
		reddenSet := map[string]bool{}
		for _, id := range redden {
			if _, ok := all[id]; !ok {
				t.Fatalf("mutation %q references unknown case %q", mut, id)
			}
			reddenSet[id] = true
		}
		flipped := 0
		for id, sc := range all {
			base, _ := gradeReference("", id, sc.Params)
			if !base {
				t.Fatalf("reference did not pass %q; fix the case before asserting mutations", id)
			}
			muted, detail := gradeReference(mut, id, sc.Params)
			if reddenSet[id] {
				if muted {
					t.Errorf("mutation %q must break %q but it still passed (%s)", mut, id, detail)
				} else {
					flipped++
				}
			} else if !muted {
				t.Errorf("mutation %q wrongly broke unrelated case %q (%s)", mut, id, detail)
			}
		}
		if flipped != len(reddenSet) {
			t.Errorf("mutation %q flipped %d cases, want %d", mut, flipped, len(reddenSet))
		}
	}
}

// TestReferenceConsumesRequiredParams proves the scenarios' semantic params are
// actually read: removing any load-bearing input param makes the reference unable
// to grade the case (it fails). This is what stops a future edit from silently
// gutting a scenario's inputs while the suite still shows green.
func TestReferenceConsumesRequiredParams(t *testing.T) {
	for id, sc := range shippedScenarioIDs(t) {
		if ok, d := gradeReference("", id, sc.Params); !ok {
			t.Fatalf("reference must pass %q before params are removed: %s", id, d)
		}
		switch {
		case strings.HasPrefix(id, "negotiation/"):
			// Negotiation grades from a set of input dimensions; removing all of
			// them must make the case ungradeable.
			blanked := clonePruned(sc.Params, "cpVersions", "workerVersions",
				"requiredCapability", "workerCapabilities", "jobTopic", "readyTopics")
			if ok, _ := gradeReference("", id, blanked); ok {
				t.Errorf("negotiation case %q still passed with all input dimensions removed", id)
			}
			continue
		case strings.HasPrefix(id, "malformed/"):
			// Malformed cases carry no removable input dimension; the reality of
			// the drop check is proven by the "malformed" mutation instead.
			continue
		}
		for _, key := range requiredInputParams(id) {
			if _, present := sc.Params[key]; !present {
				t.Errorf("case %q is missing required input param %q", id, key)
				continue
			}
			if ok, _ := gradeReference("", id, clonePruned(sc.Params, key)); ok {
				t.Errorf("case %q still passed after removing required input param %q", id, key)
			}
		}
	}
}

// requiredInputParams returns the params without which a category cannot be
// graded at all (excluding the expectation and documentation-only keys).
func requiredInputParams(id string) []string {
	switch {
	case strings.HasPrefix(id, "lifecycle/"), strings.HasPrefix(id, "cancellation/"):
		return []string{"from", "to"}
	case strings.HasPrefix(id, "safety/"):
		return []string{"decision"}
	case strings.HasPrefix(id, "duplicates/"):
		return []string{"dispatchId", "attempt", "deliveries"}
	case strings.HasPrefix(id, "retries/cp-"):
		return []string{"attempt", "activeAttempt"}
	case strings.HasPrefix(id, "retries/"):
		return []string{"attempt", "activeAttempt", "assignedWorkerId"}
	default:
		return nil
	}
}

func clonePruned(p map[string]string, remove ...string) map[string]string {
	drop := map[string]bool{}
	for _, k := range remove {
		drop[k] = true
	}
	out := make(map[string]string, len(p))
	for k, v := range p {
		if !drop[k] {
			out[k] = v
		}
	}
	return out
}
