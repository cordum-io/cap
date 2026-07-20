package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cordum-io/cap/v2/internal/releasetruth"
)

// runLinks checks every tracked markdown file's internal links and anchors under
// the render root implied by its location, reporting file:line for each problem.
func runLinks(args []string) int {
	flags := flag.NewFlagSet("links", flag.ContinueOnError)
	repoRoot := flags.String("repo-root", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	problems, err := checkAllLinks(*repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "links: %v\n", err)
		return 1
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "%s:%d: %s -> %s\n", p.File, p.Line, p.Target, p.Reason)
		}
		fmt.Fprintf(os.Stderr, "link check: %d problem(s)\n", len(problems))
		return 1
	}
	fmt.Println("link check: all internal links and anchors resolve")
	return 0
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true,
}

func checkAllLinks(repoRoot string) ([]releasetruth.LinkProblem, error) {
	var problems []releasetruth.LinkProblem
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		problems = append(problems, releasetruth.CheckLinks(repoRoot, rel, string(data), rootForPath(rel))...)
		return nil
	})
	return problems, err
}

// rootForPath maps a repo-relative markdown path to its GitHub render root.
func rootForPath(rel string) releasetruth.RenderRoot {
	switch {
	case strings.HasPrefix(rel, ".github/ISSUE_TEMPLATE/"):
		return releasetruth.RootIssueTemplate
	case strings.HasPrefix(rel, "wiki/"):
		return releasetruth.RootWiki
	case strings.HasPrefix(rel, "docs/"):
		return releasetruth.RootDocs
	default:
		return releasetruth.RootNormal
	}
}
