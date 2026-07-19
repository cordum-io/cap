package governance

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// reviewWindowDays is the minimum public review window per compatibility class.
var reviewWindowDays = map[string]int{
	"editorial":     0,
	"additive-wire": 14,
	"governance":    14,
	"security":      14,
}

var validRFCStatus = map[string]bool{
	"Draft": true, "Review": true, "Accepted": true,
	"Rejected": true, "Withdrawn": true, "Superseded": true,
}

var fileNumRE = regexp.MustCompile(`(\d{4})-`)

// RFC is a parsed RFC frontmatter record.
type RFC struct {
	Path         string
	Number       int
	Title        string
	Status       string
	Class        string
	Created      string
	ReviewOpens  string
	ReviewCloses string
	Authors      string
	Decision     string
}

// ParseRFC extracts the leading `---` frontmatter block into an RFC.
func ParseRFC(path string, content []byte) (*RFC, error) {
	fm, err := frontmatter(string(content))
	if err != nil {
		return nil, err
	}
	num, err := strconv.Atoi(strings.TrimSpace(fm["rfc"]))
	if err != nil {
		return nil, fmt.Errorf("%s: rfc number not an integer: %q", path, fm["rfc"])
	}
	return &RFC{
		Path: path, Number: num, Title: fm["title"], Status: fm["status"], Class: fm["class"],
		Created: fm["created"], ReviewOpens: fm["review-opens"], ReviewCloses: fm["review-closes"],
		Authors: fm["authors"], Decision: fm["decision"],
	}, nil
}

// frontmatter parses the fenced `---` key:value block at the top of a document.
func frontmatter(s string) (map[string]string, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, fmt.Errorf("missing --- frontmatter")
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return nil, fmt.Errorf("unterminated --- frontmatter")
	}
	out := map[string]string{}
	for _, line := range strings.Split(s[4:4+end], "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out, nil
}

// Validate returns a list of human-readable problems, empty when the RFC is valid as of
// the given reference date. It enforces class review windows and RFC chronology so that
// premature or backdated reviews are caught.
func (r *RFC) Validate(now time.Time) []string {
	var p []string
	if m := fileNumRE.FindStringSubmatch(filepath.Base(r.Path)); m == nil || m[1] != fmt.Sprintf("%04d", r.Number) {
		p = append(p, fmt.Sprintf("rfc number %04d does not match filename %s", r.Number, filepath.Base(r.Path)))
	}
	if !validRFCStatus[r.Status] {
		p = append(p, fmt.Sprintf("invalid status %q", r.Status))
	}
	window, ok := reviewWindowDays[r.Class]
	if !ok {
		p = append(p, fmt.Sprintf("invalid class %q", r.Class))
	}
	created, err := time.Parse(dateLayout, r.Created)
	if err != nil {
		p = append(p, fmt.Sprintf("invalid created date %q", r.Created))
	}
	underReview := r.Status == "Review" || r.Status == "Accepted" || r.Status == "Rejected"
	if !underReview {
		return p
	}
	return append(p, r.validateReview(created, window, now)...)
}

// validateReview checks the dated rules that apply once an RFC is in or past review.
func (r *RFC) validateReview(created time.Time, window int, now time.Time) []string {
	var p []string
	opens, oerr := time.Parse(dateLayout, r.ReviewOpens)
	closes, cerr := time.Parse(dateLayout, r.ReviewCloses)
	if oerr != nil || cerr != nil {
		return append(p, "status requires valid review-opens and review-closes dates")
	}
	if !created.IsZero() && opens.Before(created) {
		p = append(p, "review-opens predates created (backdated review)")
	}
	if closes.Before(opens.AddDate(0, 0, window)) {
		p = append(p, fmt.Sprintf("review window shorter than %d days required for class %q", window, r.Class))
	}
	if r.Status == "Accepted" || r.Status == "Rejected" {
		if r.Decision == "" || r.Decision == "none" {
			p = append(p, "Accepted/Rejected RFC must link a decision record")
		}
		if now.Before(closes) {
			p = append(p, "RFC decided before its review window closed (premature)")
		}
	}
	return p
}
