package releasetruth

import (
	"fmt"
	"strings"
)

// Protected-block markers. Generated content lives strictly between a begin
// marker (carrying the block id) and the next end marker; everything outside
// the markers is human-authored and never rewritten by the renderer.
const (
	blockBeginPrefix = "<!-- cap-release:begin:"
	blockMarkerClose = " -->"
	blockEndMarker   = "<!-- cap-release:end -->"
)

// Block is one protected region discovered in a markdown document.
type Block struct {
	ID        string // identifier parsed from the begin marker
	BeginLine int    // 1-based line number of the begin marker
	EndLine   int    // 1-based line number of the end marker
	Inner     string // current content between the markers, LF-joined, no wrapping newlines
}

// normalizeNewlines converts CRLF and lone CR to LF so rendering is newline-style
// stable regardless of how a file was checked out.
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// parseBeginID returns the block id if trimmed is a begin marker, and ok=false
// otherwise.
func parseBeginID(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, blockBeginPrefix) || !strings.HasSuffix(trimmed, blockMarkerClose) {
		return "", false
	}
	id := trimmed[len(blockBeginPrefix) : len(trimmed)-len(blockMarkerClose)]
	return strings.TrimSpace(id), true
}

// FindBlocks returns every protected block in document order. It errors on a
// begin without a matching end, an end without a begin, a nested begin, or a
// duplicate block id.
func FindBlocks(content string) ([]Block, error) {
	lines := strings.Split(normalizeNewlines(content), "\n")
	var blocks []Block
	seen := map[string]bool{}
	open := false
	var cur Block
	var inner []string
	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if id, ok := parseBeginID(trimmed); ok {
			if open {
				return nil, fmt.Errorf("nested cap-release begin marker at line %d (block %q still open)", i+1, cur.ID)
			}
			if id == "" {
				return nil, fmt.Errorf("empty cap-release block id at line %d", i+1)
			}
			if seen[id] {
				return nil, fmt.Errorf("duplicate cap-release block id %q at line %d", id, i+1)
			}
			seen[id] = true
			open, cur, inner = true, Block{ID: id, BeginLine: i + 1}, nil
			continue
		}
		if trimmed == blockEndMarker {
			if !open {
				return nil, fmt.Errorf("cap-release end marker without a begin at line %d", i+1)
			}
			cur.EndLine = i + 1
			cur.Inner = strings.Join(inner, "\n")
			blocks = append(blocks, cur)
			open = false
			continue
		}
		if open {
			inner = append(inner, raw)
		}
	}
	if open {
		return nil, fmt.Errorf("unclosed cap-release block %q opened at line %d", cur.ID, cur.BeginLine)
	}
	return blocks, nil
}

// Render rewrites the inner content of every protected block in content with
// RenderBlock output for its id, preserving all text outside blocks and
// emitting LF newlines. It is idempotent and fails closed on an unknown id.
func Render(m *Manifest, content string) (string, error) {
	normalized := normalizeNewlines(content)
	blocks, err := FindBlocks(normalized)
	if err != nil {
		return "", err
	}
	beginAt := make(map[int]Block, len(blocks))
	for _, b := range blocks {
		beginAt[b.BeginLine] = b
	}
	lines := strings.Split(normalized, "\n")
	var out []string
	for i := 0; i < len(lines); {
		if b, ok := beginAt[i+1]; ok {
			rendered, err := RenderBlock(m, b.ID)
			if err != nil {
				return "", err
			}
			out = append(out, lines[i]) // begin marker
			if rendered != "" {
				out = append(out, strings.Split(rendered, "\n")...)
			}
			out = append(out, lines[b.EndLine-1]) // end marker
			i = b.EndLine
			continue
		}
		out = append(out, lines[i])
		i++
	}
	return strings.Join(out, "\n"), nil
}
