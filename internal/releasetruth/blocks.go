package releasetruth

import (
	"fmt"
	"strings"
)

// RenderBlock returns the generated content for a known block id, derived solely
// from the manifest. Every block is a factual projection of the manifest; prose
// is never generated here. Unknown ids fail closed with an error naming the id.
func RenderBlock(m *Manifest, id string) (string, error) {
	switch id {
	case "spec-count":
		return fmt.Sprintf("%d", len(m.Specs)), nil
	case "spec-toc":
		return renderSpecTOC(m), nil
	case "release-status":
		return renderReleaseStatus(m), nil
	case "sdk-table":
		return renderSDKTable(m), nil
	case "transport-table":
		return renderTransportTable(m), nil
	case "version-policy":
		return renderVersionPolicy(m), nil
	case "security-lines":
		return renderSecurityLines(m), nil
	default:
		return "", fmt.Errorf("unknown cap-release block id %q", id)
	}
}

// mdTable renders a GitHub markdown table with no trailing newline.
func mdTable(header []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(header, " | ") + " |\n")
	seps := make([]string, len(header))
	for i := range seps {
		seps[i] = "---"
	}
	b.WriteString("| " + strings.Join(seps, " | ") + " |")
	for _, r := range rows {
		b.WriteString("\n| " + strings.Join(r, " | ") + " |")
	}
	return b.String()
}

// dash returns s, or "-" when s is empty, so table cells are never blank.
func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func renderSpecTOC(m *Manifest) string {
	rows := make([][]string, 0, len(m.Specs))
	for _, s := range m.Specs {
		rows = append(rows, []string{s.ID, fmt.Sprintf("[%s](%s)", s.Title, s.File)})
	}
	return mdTable([]string{"#", "Document"}, rows)
}

func renderReleaseStatus(m *Manifest) string {
	r, w := m.Release, m.Wire
	lines := []string{
		fmt.Sprintf("- **Current verified published release:** %s (tag `%s`, %s, channel %s)", r.Version, r.Tag, r.Date, r.Channel),
		fmt.Sprintf("- **Wire protocol:** %d (compatible range %d–%d)", w.ProtocolVersion, w.CompatMin, w.CompatMax),
		fmt.Sprintf("- **Wire schema:** %s", w.SchemaVersion),
		fmt.Sprintf("- **Specifications:** %d normative documents", len(m.Specs)),
	}
	if m.Candidate != nil {
		lines = append(lines, fmt.Sprintf("- **Release candidate (not published):** %s (tag `%s`, channel %s)",
			m.Candidate.Version, m.Candidate.Tag, m.Candidate.Channel))
	}
	if m.Snapshot != nil {
		lines = append(lines, renderSnapshotStatus(m.Snapshot))
	}
	return strings.Join(lines, "\n")
}

func renderSDKTable(m *Manifest) string {
	rows := make([][]string, 0, len(m.Components))
	for _, c := range m.Components {
		rows = append(rows, []string{
			c.Name, c.Language, c.Kind, c.Tier,
			dash(c.Registry), dash(c.Package), dash(c.Version), dash(c.Toolchain),
		})
	}
	return mdTable(
		[]string{"Component", "Language", "Kind", "Tier", "Registry", "Package", "Version", "Toolchain"},
		rows,
	)
}

func renderTransportTable(m *Manifest) string {
	rows := make([][]string, 0, len(m.Transports))
	for _, t := range m.Transports {
		rows = append(rows, []string{t.Name, t.State, dash(t.Limitations)})
	}
	return mdTable([]string{"Transport", "Status", "Notes"}, rows)
}

func renderVersionPolicy(m *Manifest) string {
	r, w := m.Release, m.Wire
	lines := []string{
		fmt.Sprintf("- **Wire protocol version:** %d. Wire evolution is append-only within the compatible range %d–%d.", w.ProtocolVersion, w.CompatMin, w.CompatMax),
		fmt.Sprintf("- **Current published release:** %s (tag `%s`). SDK and repository releases track implementation and are pinned by tag.", r.Version, r.Tag),
		"- **Source versus release:** development source may carry an in-progress version distinct from the latest published artifact; the release manifest is the authority on what is published.",
	}
	if m.Candidate != nil {
		lines = append(lines, fmt.Sprintf("- **Release candidate (not published):** %s (tag `%s`, channel %s).",
			m.Candidate.Version, m.Candidate.Tag, m.Candidate.Channel))
	}
	if m.Snapshot != nil {
		lines = append(lines, renderSnapshotStatus(m.Snapshot))
	}
	return strings.Join(lines, "\n")
}

func renderSnapshotStatus(snapshot *Snapshot) string {
	return fmt.Sprintf("- **Prepared release snapshot:** %s (tag `%s`, channel %s); publication status is not asserted by this source state.",
		snapshot.Version, snapshot.Tag, snapshot.Channel)
}

func renderSecurityLines(m *Manifest) string {
	rows := make([][]string, 0, len(m.Security.SupportedLines))
	for _, line := range m.Security.SupportedLines {
		rows = append(rows, []string{line, ":white_check_mark:"})
	}
	return mdTable([]string{"Version", "Supported"}, rows)
}
