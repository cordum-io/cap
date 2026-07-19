package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cordum-io/cap/v2/internal/tck"
)

// stringList collects a repeatable string flag.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// cmdMatrix runs a suite against several adapters and prints a per-adapter
// conformance row. It exits non-zero if any adapter is non-conformant.
//
// This is a conformance matrix over adapters, not the cross-language fixture
// (producer->consumer byte) matrix, which a later step adds.
func cmdMatrix(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("matrix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	suitePath := fs.String("suite", "", "path to the scenario suite JSON (required)")
	profile := fs.String("profile", "core", "conformance profile to run")
	deadline := fs.Duration("deadline", 30*time.Second, "per-case deadline")
	timeout := fs.Duration("timeout", 5*time.Minute, "per-adapter run deadline")
	var adapters stringList
	fs.Var(&adapters, "adapter", "adapter as 'label=argv0|arg1|...' (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *suitePath == "" || len(adapters) == 0 {
		fmt.Fprintln(stderr, "matrix: --suite and at least one --adapter are required")
		return 2
	}
	suite, err := loadSuiteFile(*suitePath)
	if err != nil {
		fmt.Fprintf(stderr, "matrix: %v\n", err)
		return 2
	}

	allConformant := true
	for _, spec := range adapters {
		rep, err := runOneAdapter(spec, suite, *profile, *deadline, *timeout)
		if err != nil {
			fmt.Fprintf(stderr, "matrix: %s: %v\n", spec, err)
			allConformant = false
			continue
		}
		printSummary(stdout, rep)
		if !rep.Summary.Conformant {
			allConformant = false
		}
	}
	if !allConformant {
		return 1
	}
	return 0
}

func loadSuiteFile(path string) (*tck.Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return tck.LoadSuite(data)
}

func runOneAdapter(specStr string, suite *tck.Suite, profile string, deadline, timeout time.Duration) (tck.Report, error) {
	spec, err := parseAdapterSpec(specStr)
	if err != nil {
		return tck.Report{}, err
	}
	ctx, cancel := signalContext(timeout)
	defer cancel()
	a := tck.NewAdapter(spec)
	if _, err := a.Start(ctx); err != nil {
		return tck.Report{}, err
	}
	defer a.Close()
	runner := tck.NewRunner()
	runner.CaseDeadline = deadline
	return runner.Run(ctx, a, suite, profile), nil
}
