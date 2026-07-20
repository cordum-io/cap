package main

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"

	gov "github.com/cordum-io/cap/v2/internal/governance"
)

// field/record separators unlikely to appear in commit metadata.
const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
)

func runDCO(args []string) (int, error) {
	fs := flag.NewFlagSet("dco", flag.ContinueOnError)
	rng := fs.String("range", "", "commit range BASE..HEAD")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if *rng == "" {
		return 2, fmt.Errorf("--range BASE..HEAD is required")
	}
	commits, err := gitLog(*rng)
	if err != nil {
		return 1, err
	}
	violations := gov.CheckDCO(commits, gov.DefaultBotAllowlist)
	if len(violations) == 0 {
		fmt.Printf("DCO: %d commit(s) checked, all signed off\n", len(commits))
		return 0, nil
	}
	fmt.Printf("DCO: %d violation(s) across %d commit(s)\n", len(violations), len(commits))
	for _, v := range violations {
		fmt.Printf("  - %s: %s\n", short(v.Hash), v.Reason)
	}
	return 1, nil
}

func gitLog(rng string) ([]gov.Commit, error) {
	format := strings.Join([]string{"%H", "%an", "%ae", "%B"}, fieldSep) + recordSep
	out, err := exec.Command("git", "log", "--no-merges", "--format="+format, rng).Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", rng, err)
	}
	var commits []gov.Commit
	for _, rec := range strings.Split(string(out), recordSep) {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, fieldSep, 4)
		if len(f) < 4 {
			continue
		}
		commits = append(commits, gov.Commit{Hash: f[0], AuthorName: f[1], AuthorEmail: f[2], Message: f[3]})
	}
	return commits, nil
}

func short(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}
