package governance

import (
	"regexp"
	"strings"
)

// DefaultBotAllowlist names automated authors exempt from DCO sign-off. Matching is by
// substring against the commit author name or email, so the GitHub `[bot]` identities
// are caught regardless of their noreply email shape.
var DefaultBotAllowlist = []string{"dependabot[bot]", "github-actions[bot]"}

// Commit is the minimal commit shape the DCO checker needs. The CLI populates it from
// `git log`; tests populate it directly.
type Commit struct {
	Hash        string
	AuthorName  string
	AuthorEmail string
	Message     string
}

// DCOViolation is a single failed commit with a human-readable reason.
type DCOViolation struct {
	Hash   string
	Reason string
}

var (
	signoffRE  = regexp.MustCompile(`(?im)^\s*Signed-off-by:\s*(.+?)\s*<([^>]+)>\s*$`)
	coAuthorRE = regexp.MustCompile(`(?im)^\s*Co-authored-by:\s*(.+?)\s*<([^>]+)>\s*$`)
	whitespace = regexp.MustCompile(`\s+`)
)

// normIdentity collapses a name+email into a case-insensitive comparison key so that
// "Yaron Torgeman <YARON@x.com>" and "yaron torgeman <yaron@x.com>" match.
func normIdentity(name, email string) string {
	n := strings.ToLower(whitespace.ReplaceAllString(strings.TrimSpace(name), " "))
	e := strings.ToLower(strings.TrimSpace(email))
	return n + "|" + e
}

func isAllowlisted(c Commit, allowlist []string) bool {
	hay := strings.ToLower(c.AuthorName + " " + c.AuthorEmail)
	for _, b := range allowlist {
		if strings.Contains(hay, strings.ToLower(b)) {
			return true
		}
	}
	return false
}

func signoffSet(message string) map[string]bool {
	set := map[string]bool{}
	for _, m := range signoffRE.FindAllStringSubmatch(message, -1) {
		set[normIdentity(m[1], m[2])] = true
	}
	return set
}

// CheckDCO returns a violation for every non-allowlisted commit that lacks a
// Signed-off-by matching its author, or whose co-authors lack matching sign-offs.
func CheckDCO(commits []Commit, allowlist []string) []DCOViolation {
	var out []DCOViolation
	for _, c := range commits {
		if isAllowlisted(c, allowlist) {
			continue
		}
		signoffs := signoffSet(c.Message)
		if !signoffs[normIdentity(c.AuthorName, c.AuthorEmail)] {
			out = append(out, DCOViolation{c.Hash, "missing Signed-off-by matching author " +
				c.AuthorName + " <" + c.AuthorEmail + ">"})
		}
		for _, m := range coAuthorRE.FindAllStringSubmatch(c.Message, -1) {
			if !signoffs[normIdentity(m[1], m[2])] {
				out = append(out, DCOViolation{c.Hash,
					"co-author " + strings.TrimSpace(m[1]) + " <" + strings.TrimSpace(m[2]) + "> has no matching Signed-off-by"})
			}
		}
	}
	return out
}
