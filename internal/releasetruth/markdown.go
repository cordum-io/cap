package releasetruth

import (
	"fmt"
	"strings"
)

// GitHubSlug converts one heading's text to its GitHub anchor slug: lowercased,
// characters other than [a-z0-9_-] and whitespace dropped, then runs of
// whitespace collapsed to a single hyphen. It does not apply duplicate suffixes.
func GitHubSlug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '\t':
			b.WriteRune(' ')
		default:
			// drop punctuation and other symbols
		}
	}
	return strings.Join(strings.Fields(b.String()), "-")
}

// isFenceLine reports whether a trimmed line opens or closes a code fence.
func isFenceLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// parseATXHeading returns the heading text if line is an ATX heading (1-6 '#'
// followed by a space or end of line), stripping the leading and optional
// closing '#' sequence.
func parseATXHeading(line string) (string, bool) {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return "", false
	}
	if i < len(line) && line[i] != ' ' && line[i] != '\t' {
		return "", false
	}
	text := strings.TrimSpace(line[i:])
	text = strings.TrimSpace(strings.TrimRight(text, "#"))
	return text, true
}

// DocumentAnchors returns the ordered anchor slugs for all ATX headings in the
// document, applying GitHub duplicate-slug suffixes (foo, foo-1, foo-2) and
// ignoring fenced code blocks.
func DocumentAnchors(content string) []string {
	var anchors []string
	counts := map[string]int{}
	inFence := false
	for _, raw := range strings.Split(normalizeNewlines(content), "\n") {
		t := strings.TrimSpace(raw)
		if isFenceLine(t) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		text, ok := parseATXHeading(t)
		if !ok {
			continue
		}
		slug := GitHubSlug(text)
		n := counts[slug]
		counts[slug]++
		if n == 0 {
			anchors = append(anchors, slug)
		} else {
			anchors = append(anchors, fmt.Sprintf("%s-%d", slug, n))
		}
	}
	return anchors
}
