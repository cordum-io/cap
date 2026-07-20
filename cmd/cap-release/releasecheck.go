package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cordum-io/cap/v2/internal/releasetruth"
)

// runReleaseCheck validates a release tag against the manifest, source metadata,
// and changelog. It is the fail-closed gate a release-tag workflow runs; it never
// mutates source and never publishes.
func runReleaseCheck(args []string) int {
	fs := flag.NewFlagSet("release-check", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "release/manifest.json", "path to the release manifest")
	repoRoot := fs.String("repo-root", ".", "repository root")
	tag := fs.String("tag", "", "release tag to validate (e.g. v2.14.0)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *tag == "" {
		fmt.Fprintln(os.Stderr, "release-check: --tag is required")
		return 2
	}
	m, err := releasetruth.LoadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		return 1
	}
	problems := releasetruth.ReleaseCheck(m, *repoRoot, *tag)
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "release-check FAILED for %s: %d problem(s)\n", *tag, len(problems))
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", p.Error())
		}
		return 1
	}
	fmt.Printf("release-check OK: %s matches manifest, source metadata, and changelog\n", *tag)
	return 0
}
