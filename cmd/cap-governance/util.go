package main

import "strings"

// normalizeEOL makes stale-check comparisons independent of CRLF/LF differences.
func normalizeEOL(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
