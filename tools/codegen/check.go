package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Check validates the manifest's internal shape and compares it against root in
// both directions: every declared output must exist with the pinned bytes, and
// no generated artifact on disk may be absent from the declaration. An empty
// result means the generated tree exactly matches the declaration.
func Check(m *Manifest, root fs.FS) []Problem {
	var p []Problem
	p = append(p, checkOutputPaths(m)...)
	p = append(p, checkSources(m, root)...)
	p = append(p, checkOutputsOnDisk(m, root)...)
	p = append(p, checkUnmanifested(m, root)...)
	p = append(p, checkGenerators(m)...)
	return p
}

// checkOutputPaths enforces safe, sorted, unique output paths. Sorted order
// keeps review diffs stable; an unsafe path is refused, never normalized away.
func checkOutputPaths(m *Manifest) []Problem {
	var p []Problem
	seen := map[string]bool{}
	names := make([]string, len(m.Outputs))
	for i, o := range m.Outputs {
		names[i] = o.Path
		if isAbsPath(o.Path) {
			p = append(p, cprob(CodeAbsolutePath, o.Path, ""))
		} else if hasDotDot(o.Path) {
			p = append(p, cprob(CodePathTraversal, o.Path, ""))
		}
		if seen[o.Path] {
			p = append(p, cprob(CodeDuplicatePath, o.Path, ""))
		}
		seen[o.Path] = true
	}
	if !sort.StringsAreSorted(names) {
		p = append(p, cprob(CodeUnsortedPaths, "", ""))
	}
	return p
}

// checkSources ensures every declared proto source actually exists.
func checkSources(m *Manifest, root fs.FS) []Problem {
	var p []Problem
	for _, s := range m.Sources {
		if !exists(root, s) {
			p = append(p, cprob(CodeSourceMissing, s, ""))
		}
	}
	return p
}

// checkOutputsOnDisk resolves each declared output and pins it by content.
// Existence is not enough: a present-but-stale file is the false green this
// checker exists to catch, and an empty file is reported distinctly.
func checkOutputsOnDisk(m *Manifest, root fs.FS) []Problem {
	var p []Problem
	for _, o := range m.Outputs {
		if !fs.ValidPath(o.Path) {
			continue // path shape already reported by checkOutputPaths
		}
		data, err := fs.ReadFile(root, o.Path)
		if err != nil {
			p = append(p, cprob(CodeOutputMissing, o.Path, err.Error()))
			continue
		}
		if len(data) == 0 {
			p = append(p, cprob(CodeOutputEmpty, o.Path, ""))
			continue
		}
		if !strings.EqualFold(sha256hex(data), o.SHA256) {
			p = append(p, cprob(CodeOutputStale, o.Path, ""))
		}
	}
	return p
}

// checkUnmanifested walks the tree and flags any file matching a generated glob
// that the manifest does not declare, so stray or stale artifacts cannot hide.
func checkUnmanifested(m *Manifest, root fs.FS) []Problem {
	declared := map[string]bool{}
	for _, o := range m.Outputs {
		declared[o.Path] = true
	}
	var p []Problem
	_ = fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || declared[name] {
			return nil
		}
		if matchesAnyGlob(name, m.GeneratedGlobs) {
			p = append(p, cprob(CodeOutputUnmanifested, name, ""))
		}
		return nil
	})
	return p
}

// checkGenerators enforces that every generator produces something, references
// only declared outputs, and that every declared output is claimed by exactly
// one generator, so no artifact can be checked in that no generator reproduces.
func checkGenerators(m *Manifest) []Problem {
	var p []Problem
	declared := map[string]bool{}
	for _, o := range m.Outputs {
		declared[o.Path] = true
	}
	claimed := map[string]bool{}
	for _, g := range m.Generators {
		if len(g.Outputs) == 0 {
			p = append(p, cprob(CodeGeneratorNoOutputs, "", g.ID))
			continue
		}
		for _, out := range g.Outputs {
			if !declared[out] {
				p = append(p, cprob(CodeUnknownOutputRef, out, g.ID))
			}
			claimed[out] = true
		}
	}
	for _, o := range m.Outputs {
		if !claimed[o.Path] {
			p = append(p, cprob(CodeOutputUnclaimed, o.Path, ""))
		}
	}
	return p
}

func cprob(code, filePath, detail string) Problem {
	return Problem{Code: code, Path: filePath, Detail: detail}
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// isAbsPath reports rooted paths, including Windows volume-qualified ones, so a
// manifest cannot point a generated-file writer outside the repository tree.
func isAbsPath(p string) bool {
	if path.IsAbs(p) {
		return true
	}
	return len(p) >= 2 && p[1] == ':'
}

func hasDotDot(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

func matchesAnyGlob(name string, globs []string) bool {
	for _, g := range globs {
		if ok, _ := path.Match(g, name); ok {
			return true
		}
	}
	return false
}

// exists reports whether name resolves to a file in root.
func exists(root fs.FS, name string) bool {
	if !fs.ValidPath(name) {
		return false
	}
	_, err := fs.Stat(root, name)
	return err == nil
}
