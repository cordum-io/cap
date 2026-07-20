package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cordum-io/cap/v2/internal/releasetruth"
)

func runPromotionCheck(args []string) int {
	fs := flag.NewFlagSet("promotion-check", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "release/manifest.json", "path to the release manifest")
	repoRoot := fs.String("repo-root", ".", "repository root with fetched release tags")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	m, err := releasetruth.LoadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		return 1
	}
	commit, err := resolveLocalTagCommit(*repoRoot, m.Release.Tag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "promotion-check: %v\n", err)
		return 1
	}
	problems := releasetruth.PromotionCheck(m, *repoRoot, commit)
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "promotion-check FAILED: %d problem(s)\n", len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem.Error())
		}
		return 1
	}
	fmt.Printf("promotion-check OK: %s resolves to %s with bound release metadata\n", m.Release.Tag, commit)
	return 0
}

func resolveLocalTagCommit(repoRoot, tag string) (string, error) {
	ref := "refs/tags/" + tag + "^{commit}"
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve tag %s: %w: %s", tag, err, strings.TrimSpace(string(out)))
	}
	commit := strings.TrimSpace(string(out))
	cmd = exec.Command("git", "-C", repoRoot, "merge-base", "--is-ancestor", commit, "HEAD")
	if out, err = cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("tag %s target %s is not an ancestor of HEAD: %w: %s", tag, commit, err, strings.TrimSpace(string(out)))
	}
	return commit, nil
}
