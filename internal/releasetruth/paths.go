package releasetruth

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// safeRelPath verifies p is a forward-slash relative repository path that
// cannot escape the repository root. It rejects empty strings, absolute paths,
// backslashes, "." / ".." segments, and empty (double-slash) segments.
func safeRelPath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if strings.Contains(p, `\`) {
		return fmt.Errorf("backslash not allowed; use forward slashes")
	}
	if path.IsAbs(p) {
		return fmt.Errorf("absolute path not allowed")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" {
			return fmt.Errorf("empty path segment (leading, trailing, or double slash)")
		}
		if seg == "." || seg == ".." {
			return fmt.Errorf("relative segment %q not allowed", seg)
		}
	}
	return nil
}

// CheckSpecsOnDisk verifies each spec file resolves to an existing regular file
// under repoRoot without escaping it. It performs local file I/O only, no
// network access.
func CheckSpecsOnDisk(m *Manifest, repoRoot string) []Problem {
	var ps []Problem
	for _, s := range m.Specs {
		if err := safeRelPath(s.File); err != nil {
			ps = append(ps, Problem{"specs.file", "unsafe spec path " + s.File + ": " + err.Error()})
			continue
		}
		full := filepath.Join(repoRoot, filepath.FromSlash(s.File))
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			ps = append(ps, Problem{"specs.file.missing", s.File})
		}
	}
	return ps
}
