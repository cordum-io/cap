package governance

import (
	"testing"
	"time"
)

func asOf(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

const draftRFC = `---
rfc: 0001
title: CAP governance process
status: Draft
class: governance
created: 2026-07-19
review-opens: none
review-closes: none
authors: @yaront1111
supersedes: none
superseded-by: none
decision: none
---
body`

func TestParseAndValidateDraft(t *testing.T) {
	r, err := ParseRFC("rfcs/0001-governance-process.md", []byte(draftRFC))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Number != 1 || r.Class != "governance" || r.Status != "Draft" {
		t.Fatalf("unexpected parse: %+v", r)
	}
	if p := r.Validate(asOf("2026-07-20")); len(p) != 0 {
		t.Fatalf("valid draft should have no problems, got %v", p)
	}
}

func TestNumberMustMatchFilename(t *testing.T) {
	r, _ := ParseRFC("rfcs/0007-governance-process.md", []byte(draftRFC))
	if p := r.Validate(asOf("2026-07-20")); len(p) == 0 {
		t.Fatal("rfc number 0001 in a 0007-* file must be flagged")
	}
}

func reviewRFC(opens, closes, status, decision string) string {
	return "---\nrfc: 0002\ntitle: t\nstatus: " + status + "\nclass: governance\ncreated: 2026-07-01\n" +
		"review-opens: " + opens + "\nreview-closes: " + closes + "\nauthors: @a\nsupersedes: none\nsuperseded-by: none\ndecision: " + decision + "\n---\n"
}

func TestGovernanceReviewWindowMustBe14Days(t *testing.T) {
	// 10-day window is too short for the governance class.
	r, _ := ParseRFC("rfcs/0002-t.md", []byte(reviewRFC("2026-07-01", "2026-07-11", "Review", "none")))
	if p := r.Validate(asOf("2026-07-12")); len(p) == 0 {
		t.Fatal("a 10-day governance review window must be rejected")
	}
	r2, _ := ParseRFC("rfcs/0002-t.md", []byte(reviewRFC("2026-07-01", "2026-07-15", "Review", "none")))
	if p := r2.Validate(asOf("2026-07-16")); len(p) != 0 {
		t.Fatalf("a 14-day window should be accepted, got %v", p)
	}
}

func TestAcceptedNeedsDecisionLink(t *testing.T) {
	r, _ := ParseRFC("rfcs/0002-t.md", []byte(reviewRFC("2026-07-01", "2026-07-15", "Accepted", "none")))
	if p := r.Validate(asOf("2026-07-20")); len(p) == 0 {
		t.Fatal("Accepted RFC with decision:none must be flagged")
	}
}

func TestCannotAcceptBeforeReviewCloses(t *testing.T) {
	// review closes 2026-07-15 but we evaluate as-of 2026-07-10: premature.
	r, _ := ParseRFC("rfcs/0002-t.md", []byte(reviewRFC("2026-07-01", "2026-07-15", "Accepted", "DECISIONS.md#d-0001")))
	if p := r.Validate(asOf("2026-07-10")); len(p) == 0 {
		t.Fatal("accepting before the review window closes (premature) must be flagged")
	}
}

func TestReviewOpensCannotPredateCreated(t *testing.T) {
	r, _ := ParseRFC("rfcs/0002-t.md", []byte(reviewRFC("2026-06-01", "2026-06-20", "Review", "none")))
	if p := r.Validate(asOf("2026-07-20")); len(p) == 0 {
		t.Fatal("review-opens before created (backdated) must be flagged")
	}
}
