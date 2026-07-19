package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/cordum-io/cap/v2/internal/tck"
)

// cmdList prints the scenarios in a suite in deterministic order, optionally
// filtered to a profile. It executes nothing.
func cmdList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	suitePath := fs.String("suite", "", "path to the scenario suite JSON (required)")
	profile := fs.String("profile", "", "restrict to a single profile")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *suitePath == "" {
		fmt.Fprintln(stderr, "list: --suite is required")
		return 2
	}
	data, err := os.ReadFile(*suitePath)
	if err != nil {
		fmt.Fprintf(stderr, "list: %v\n", err)
		return 2
	}
	suite, err := tck.LoadSuite(data)
	if err != nil {
		fmt.Fprintf(stderr, "list: %v\n", err)
		return 2
	}
	scenarios := suite.Scenarios
	if *profile != "" {
		scenarios = suite.Select(*profile)
	}
	for _, sc := range scenarios {
		req := "optional"
		if sc.Required {
			req = "required"
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", sc.ID, sc.Role, sc.Profile, req)
	}
	return 0
}
