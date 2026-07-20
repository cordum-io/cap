package codegen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
)

// hashContent returns the sha256 of b with CRLF normalized to LF. The manifest
// therefore pins the canonical (LF) generated content and matches on both a
// Windows checkout (git autocrlf yields a CRLF working tree) and a Linux CI
// checkout (LF), so drift detection is portable rather than platform-dependent.
func hashContent(b []byte) string {
	n := bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	sum := sha256.Sum256(n)
	return hex.EncodeToString(sum[:])
}
