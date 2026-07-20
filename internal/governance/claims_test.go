package governance

import "testing"

func TestLintFlagsFabricatedClaims(t *testing.T) {
	cases := []string{
		"CAP is foundation-ready today.",
		"We are a CNCF Sandbox project.",
		"Certified conformant by the CAP foundation.",
		"Official partner of Acme Corp.",
		"Early adopters include Globex and Initech.",
		"Trusted by 400 companies.",
	}
	for _, c := range cases {
		if hits := LintClaims("launch-kit/x.md", []byte(c)); len(hits) == 0 {
			t.Errorf("expected a claim hit for %q", c)
		}
	}
}

func TestLintAllowsHonestNegatedLanguage(t *testing.T) {
	honest := "Apache-2.0 is a license; it does not imply Apache Software Foundation " +
		"affiliation. Readiness is BLOCKED. A self-report is not certification or endorsement."
	if hits := LintClaims("GOVERNANCE.md", []byte(honest)); len(hits) != 0 {
		t.Fatalf("honest negated language must not be flagged, got %+v", hits)
	}
}

func TestNegatedGovernanceMentionsNotFlagged(t *testing.T) {
	honest := []string{
		"No working group or TSC exists today.",
		"CAP is not a CNCF Sandbox project.",
		"A self-report is not certified by any foundation.",
		"We are not an official partner of anyone.",
	}
	for _, h := range honest {
		if hits := LintClaims("GOVERNANCE.md", []byte(h)); len(hits) != 0 {
			t.Errorf("negated honest line %q flagged: %+v", h, hits)
		}
	}
}

func TestDraftBannerRequired(t *testing.T) {
	if RequireDraftBanner([]byte("Dear prospect, join our program.")) == false {
		t.Fatal("outreach without the DRAFT banner must be flagged (true = missing)")
	}
	ok := "DRAFT — DO NOT SEND OR PUBLISH WITHOUT EXPLICIT HUMAN APPROVAL\n\nDear prospect..."
	if RequireDraftBanner([]byte(ok)) == true {
		t.Fatal("outreach WITH the banner must pass (false = present)")
	}
}
