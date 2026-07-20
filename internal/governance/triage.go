package governance

import "time"

// NeedsTriageLabel marks an item awaiting first human triage.
const NeedsTriageLabel = "needs-triage"

// TriageItem is a minimal issue/PR shape for the triage-age audit. The CLI populates it
// from `gh` API JSON; tests populate it directly with a fixed clock.
type TriageItem struct {
	Number    int       `json:"number"`
	CreatedAt time.Time `json:"createdAt"`
	Labels    []string  `json:"labels"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
}

func (i TriageItem) hasLabel(l string) bool {
	for _, x := range i.Labels {
		if x == l {
			return true
		}
	}
	return false
}

// OverdueTriage returns items still labeled needs-triage whose age exceeds slaDays as of
// now. The comparison uses whole elapsed hours so the same fixed clock in tests always
// produces the same result, independent of wall-clock time.
func OverdueTriage(items []TriageItem, now time.Time, slaDays int) []TriageItem {
	var over []TriageItem
	limit := time.Duration(slaDays) * 24 * time.Hour
	for _, it := range items {
		if it.hasLabel(NeedsTriageLabel) && now.Sub(it.CreatedAt) > limit {
			over = append(over, it)
		}
	}
	return over
}
