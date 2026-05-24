package capsdk

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxAgentNameLen bounds the human-facing agent display label carried on
// Heartbeat.agent_name / Handshake.agent_name. Labels longer than this are
// truncated by SanitizeAgentName. It is a rendering bound, NOT a security
// limit: agent_name is never an authentication authority.
const MaxAgentNameLen = 128

// SanitizeAgentName normalizes a self-reported agent display label for safe,
// bounded transport and rendering. It trims surrounding whitespace, collapses
// internal runs of whitespace (including newlines and tabs) to a single space,
// drops non-space control characters that could break log lines or audit
// summaries, drops invalid/replacement runes, and truncates to
// MaxAgentNameLen runes (preserving valid UTF-8).
//
// The result is a DISPLAY label only. Consumers MUST prefer authenticated
// identity records over this value and MUST NOT treat it as proof of identity.
func SanitizeAgentName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	lastSpace := false
	for _, r := range name {
		switch {
		case r == utf8.RuneError:
			// Drop invalid bytes / replacement characters.
			continue
		case unicode.IsSpace(r):
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		case unicode.IsControl(r):
			// Non-space control characters (e.g. NUL, ESC) are dropped.
			continue
		default:
			b.WriteRune(r)
			lastSpace = false
		}
	}
	out := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(out) > MaxAgentNameLen {
		out = strings.TrimSpace(string([]rune(out)[:MaxAgentNameLen]))
	}
	return out
}
