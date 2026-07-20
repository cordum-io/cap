package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cordum-io/cap/v2/internal/releasetruth"
)

// renderTarget is a documentation file whose protected blocks are generated from
// the manifest, plus the render root that governs how its links resolve.
type renderTarget struct {
	path string
	root releasetruth.RenderRoot
}

// renderTargets is the canonical set of docs whose factual blocks derive from the
// manifest. Prose outside the protected markers is human-authored and untouched.
var renderTargets = []renderTarget{
	{"README.md", releasetruth.RootNormal},
	{"sdk/README.md", releasetruth.RootNormal},
	{"docs/sdk-comparison.md", releasetruth.RootDocs},
	{"docs/reference.md", releasetruth.RootDocs},
	{"spec/00-index.md", releasetruth.RootNormal},
	{"spec/17-versioning-policy.md", releasetruth.RootNormal},
	{"SECURITY.md", releasetruth.RootNormal},
	{"wiki/Home.md", releasetruth.RootWiki},
}

func runRender(args []string) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "release/manifest.json", "path to the release manifest")
	repoRoot := fs.String("repo-root", ".", "repository root")
	write := fs.Bool("write", false, "write rendered blocks back to files")
	check := fs.Bool("check", false, "exit nonzero if any file would change")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *write == *check {
		fmt.Fprintln(os.Stderr, "render: specify exactly one of --write or --check")
		return 2
	}
	m, err := releasetruth.LoadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		return 1
	}
	return renderAll(m, *repoRoot, *write)
}

func renderAll(m *releasetruth.Manifest, repoRoot string, write bool) int {
	drift := 0
	for _, t := range renderTargets {
		full := filepath.Join(repoRoot, filepath.FromSlash(t.path))
		data, err := os.ReadFile(full)
		if err != nil {
			if !write {
				// Fail closed: a canonical target must exist for the drift gate.
				fmt.Fprintf(os.Stderr, "MISSING target %s: %v\n", t.path, err)
				drift++
				continue
			}
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", t.path, err)
			continue
		}
		canonical, drifted, err := releasetruth.CheckDrift(m, string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "render %s: %v\n", t.path, err)
			return 1
		}
		if !drifted {
			continue
		}
		drift++
		if write {
			// Preserve the file's existing newline style so a CRLF-committed doc is
			// not rewritten wholesale into LF (which would bury the real change).
			out := canonical
			if strings.Contains(string(data), "\r\n") {
				out = strings.ReplaceAll(canonical, "\n", "\r\n")
			}
			if err := atomicWrite(full, out); err != nil {
				fmt.Fprintf(os.Stderr, "write %s: %v\n", t.path, err)
				return 1
			}
			fmt.Printf("rendered %s\n", t.path)
			continue
		}
		fmt.Fprintf(os.Stderr, "DRIFT %s:\n%s\n", t.path, simpleDiff(toLF(string(data)), canonical))
	}
	if !write && drift > 0 {
		fmt.Fprintf(os.Stderr, "render --check: %d file(s) out of sync with the manifest\n", drift)
		return 1
	}
	if write {
		fmt.Printf("render --write: %d file(s) updated\n", drift)
	} else {
		fmt.Println("render --check: all files in sync")
	}
	return 0
}

func toLF(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// atomicWrite writes content to a temp file in the same directory then renames it
// over path so a partial write never corrupts the target.
func atomicWrite(path, content string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".cap-release-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// simpleDiff prints the single changed line range between old and new content.
func simpleDiff(oldC, newC string) string {
	o, n := strings.Split(oldC, "\n"), strings.Split(newC, "\n")
	i := 0
	for i < len(o) && i < len(n) && o[i] == n[i] {
		i++
	}
	if i == len(o) && i == len(n) {
		return "(no line differences)"
	}
	jo, jn := len(o)-1, len(n)-1
	for jo >= i && jn >= i && o[jo] == n[jn] {
		jo--
		jn--
	}
	var b strings.Builder
	fmt.Fprintf(&b, "@@ around line %d @@\n", i+1)
	for k := i; k <= jo; k++ {
		fmt.Fprintf(&b, "- %s\n", o[k])
	}
	for k := i; k <= jn; k++ {
		fmt.Fprintf(&b, "+ %s\n", n[k])
	}
	return b.String()
}
