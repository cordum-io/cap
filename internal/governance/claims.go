package governance

import (
	"regexp"
	"strings"
)

// DraftBanner is the exact banner every draft outreach/partnership asset must carry.
const DraftBanner = "DRAFT — DO NOT SEND OR PUBLISH WITHOUT EXPLICIT HUMAN APPROVAL"

// ClaimHit is one flagged line of an outward-facing document.
type ClaimHit struct {
	File string
	Line int
	Rule string
	Text string
}

type claimRule struct {
	re     *regexp.Regexp
	reason string
}

// negationRE marks a line as an honest disclaimer rather than a positive claim. A
// fabricated claim ("we are a CNCF project") never negates itself; an honest note ("no
// working group exists", "not certified") does. Skipping negated lines avoids flagging
// the project's own truthful disclosures. Trade-off: a claim smuggled into a sentence
// that also contains a negation elsewhere is not caught — acceptable for a lint gate
// whose worse failure mode is blocking honest docs.
var negationRE = regexp.MustCompile(`(?i)\b(no|not|never|without|isn't|aren't|wasn't|doesn't|don't|nor|no longer)\b`)

// claimRules target *positive* fabricated assertions. They are written narrowly so that
// honest, negated disclosure ("does not imply foundation affiliation", "not
// certification") is not flagged.
var claimRules = []claimRule{
	{regexp.MustCompile(`(?i)\bfoundation[- ]ready\b`), "unverified foundation-ready claim"},
	{regexp.MustCompile(`(?i)\bCNCF\s+(member|project|sandbox|incubating|graduated)\b`), "unverified CNCF status claim"},
	{regexp.MustCompile(`(?i)\bcertified\s+(conformant|implementation|by)\b`), "certification claim (self-test is not certification)"},
	{regexp.MustCompile(`(?i)\bofficial\s+partner\b`), "unverified partnership claim"},
	{regexp.MustCompile(`(?i)\bearly[- ]adopters?\s+include\b`), "unverified adopter claim"},
	{regexp.MustCompile(`(?i)\btrusted\s+by\s+\d`), "fabricated adopter/metric claim"},
	{regexp.MustCompile(`(?i)\bworking\s+group\b`), "nonexistent working-group claim"},
}

// LintClaims returns a hit for each line matching a fabricated-claim rule.
func LintClaims(file string, content []byte) []ClaimHit {
	var hits []ClaimHit
	for i, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		if negationRE.MatchString(line) {
			continue // honest disclaimer, not a positive claim
		}
		for _, r := range claimRules {
			if r.re.MatchString(line) {
				hits = append(hits, ClaimHit{File: file, Line: i + 1, Rule: r.reason, Text: strings.TrimSpace(line)})
			}
		}
	}
	return hits
}

// RequireDraftBanner reports whether the DRAFT banner is MISSING (true = missing/flag).
func RequireDraftBanner(content []byte) bool {
	return !strings.Contains(string(content), DraftBanner)
}
