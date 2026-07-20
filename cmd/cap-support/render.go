package main

import (
	"fmt"
	"sort"
	"strings"

	sdksupport "github.com/cordum-io/cap/v2/tools/sdk-support"
)

const (
	beginMarker = "<!-- BEGIN GENERATED SDK SUPPORT TABLE -->"
	endMarker   = "<!-- END GENERATED SDK SUPPORT TABLE -->"
)

var tierRank = map[sdksupport.Tier]int{
	sdksupport.TierStable:       0,
	sdksupport.TierCommunity:    1,
	sdksupport.TierExperimental: 2,
}

// renderTable produces the deterministic Markdown support table. Entries are
// ordered by tier then id, and any pending (not-yet-earned) gate is shown with
// its blocking task so a reader never mistakes pending evidence for satisfied.
func renderTable(m *sdksupport.Manifest) string {
	entries := append([]sdksupport.Entry(nil), m.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		if tierRank[entries[i].Tier] != tierRank[entries[j].Tier] {
			return tierRank[entries[i].Tier] < tierRank[entries[j].Tier]
		}
		return entries[i].ID < entries[j].ID
	})
	var b strings.Builder
	b.WriteString("| SDK | Kind | Tier | Owner | Pending evidence |\n")
	b.WriteString("|-----|------|------|-------|------------------|\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", e.ID, e.Kind, e.Tier, e.Owner, pendingCell(e))
	}
	return b.String()
}

func pendingCell(e sdksupport.Entry) string {
	pg := e.PendingGates()
	if len(pg) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(pg))
	for _, g := range pg {
		parts = append(parts, fmt.Sprintf("%s (%s)", g.ID, g.BlockedBy))
	}
	return strings.Join(parts, ", ")
}

// tierMatrix groups SDK ids by tier in sorted order, for a CI job matrix.
func tierMatrix(m *sdksupport.Manifest) map[string][]string {
	out := map[string][]string{
		string(sdksupport.TierStable):       {},
		string(sdksupport.TierCommunity):    {},
		string(sdksupport.TierExperimental): {},
	}
	for _, e := range m.Entries {
		out[string(e.Tier)] = append(out[string(e.Tier)], e.ID)
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// spliceTable replaces the content between the markers in doc with block. It
// fails if either marker is absent, so a hand edit cannot silently drop the
// generated region.
func spliceTable(doc, block string) (string, error) {
	begin := strings.Index(doc, beginMarker)
	end := strings.Index(doc, endMarker)
	if begin < 0 || end < 0 || end < begin {
		return "", fmt.Errorf("markers not found or out of order")
	}
	before := doc[:begin+len(beginMarker)]
	after := doc[end:]
	return before + "\n" + block + after, nil
}

// currentTable extracts the content currently between the markers.
func currentTable(doc string) (string, error) {
	begin := strings.Index(doc, beginMarker)
	end := strings.Index(doc, endMarker)
	if begin < 0 || end < 0 || end < begin {
		return "", fmt.Errorf("markers not found or out of order")
	}
	return strings.TrimLeft(doc[begin+len(beginMarker):end], "\n"), nil
}
