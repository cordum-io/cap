package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	gov "github.com/cordum-io/cap/v2/internal/governance"
)

func runTriage(args []string) (int, error) {
	fs := flag.NewFlagSet("triage", flag.ContinueOnError)
	input := fs.String("input", "", "path to JSON array of {number,createdAt,labels,title,url}")
	slaDays := fs.Int("sla-days", 7, "first-human-triage SLA in days")
	asOfStr := fs.String("as-of", "", "reference date/time RFC3339 (default now)")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if *input == "" {
		return 2, fmt.Errorf("--input is required")
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return 1, err
	}
	var items []gov.TriageItem
	if err := json.Unmarshal(data, &items); err != nil {
		return 1, fmt.Errorf("parse triage input: %w", err)
	}
	now := time.Now()
	if *asOfStr != "" {
		if now, err = time.Parse(time.RFC3339, *asOfStr); err != nil {
			return 2, fmt.Errorf("bad --as-of: %w", err)
		}
	}
	over := gov.OverdueTriage(items, now, *slaDays)
	summary := fmt.Sprintf("triage audit: %d item(s) scanned, %d overdue (SLA %d days)\n", len(items), len(over), *slaDays)
	fmt.Print(summary)
	for _, it := range over {
		fmt.Printf("  - #%d %s (%s)\n", it.Number, it.Title, it.URL)
	}
	writeStepSummary(summary, over)
	if len(over) > 0 {
		return 1, nil
	}
	return 0, nil
}

// writeStepSummary appends a short report to the GitHub Actions job summary when running
// in CI; it is a no-op locally.
func writeStepSummary(header string, over []gov.TriageItem) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "### %s\n", header)
	for _, it := range over {
		fmt.Fprintf(f, "- [#%d](%s) %s\n", it.Number, it.URL, it.Title)
	}
}
