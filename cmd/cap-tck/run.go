package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/cordum-io/cap/v2/internal/tck"
)

// cmdRun runs one adapter against one profile of a suite and exits non-zero if
// the run is not conformant.
func cmdRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	suitePath := fs.String("suite", "", "path to the scenario suite JSON (required)")
	profile := fs.String("profile", "core", "conformance profile to run")
	adapter := fs.String("adapter", "", "adapter as 'label=argv0|arg1|...' (required)")
	jsonOut := fs.String("json", "", "write JSON report to this path ('-' for stdout)")
	junitOut := fs.String("junit", "", "write JUnit XML report to this path ('-' for stdout)")
	deadline := fs.Duration("deadline", 30*time.Second, "per-case deadline")
	timeout := fs.Duration("timeout", 5*time.Minute, "global run deadline")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *suitePath == "" || *adapter == "" {
		fmt.Fprintln(stderr, "run: --suite and --adapter are required")
		return 2
	}
	suite, spec, err := loadRunInputs(*suitePath, *adapter)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	workdir, err := os.MkdirTemp("", "cap-tck-run-*")
	if err != nil {
		fmt.Fprintf(stderr, "run: temp dir: %v\n", err)
		return 1
	}
	defer os.RemoveAll(workdir)
	spec.Dir = workdir

	ctx, cancel := signalContext(*timeout)
	defer cancel()
	a := tck.NewAdapter(spec)
	if _, err := a.Start(ctx); err != nil {
		fmt.Fprintf(stderr, "run: start adapter: %v\n", err)
		return 1
	}
	defer a.Close()

	runner := tck.NewRunner()
	runner.CaseDeadline = *deadline
	rep := runner.Run(ctx, a, suite, *profile)
	if err := writeReports(rep, *jsonOut, *junitOut, stdout); err != nil {
		fmt.Fprintf(stderr, "run: write report: %v\n", err)
		return 1
	}
	printSummary(stdout, rep)
	if !rep.Summary.Conformant {
		return 1
	}
	return 0
}

func loadRunInputs(suitePath, adapter string) (*tck.Suite, tck.AdapterSpec, error) {
	data, err := os.ReadFile(suitePath)
	if err != nil {
		return nil, tck.AdapterSpec{}, fmt.Errorf("read suite: %w", err)
	}
	suite, err := tck.LoadSuite(data)
	if err != nil {
		return nil, tck.AdapterSpec{}, err
	}
	spec, err := parseAdapterSpec(adapter)
	if err != nil {
		return nil, tck.AdapterSpec{}, err
	}
	return suite, spec, nil
}

// signalContext returns a context cancelled on interrupt or after timeout so a
// run never wedges and always tears its adapter down.
func signalContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	base, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	ctx, cancel := context.WithTimeout(base, timeout)
	return ctx, func() {
		cancel()
		stop()
	}
}

func writeReports(rep tck.Report, jsonPath, junitPath string, stdout io.Writer) error {
	if jsonPath != "" {
		b, err := rep.JSON()
		if err != nil {
			return err
		}
		if err := emitReport(jsonPath, b, stdout); err != nil {
			return err
		}
	}
	if junitPath != "" {
		b, err := rep.JUnit()
		if err != nil {
			return err
		}
		if err := emitReport(junitPath, b, stdout); err != nil {
			return err
		}
	}
	return nil
}

func emitReport(path string, body []byte, stdout io.Writer) error {
	if path == "-" {
		_, err := stdout.Write(body)
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func printSummary(w io.Writer, rep tck.Report) {
	verdict := "CONFORMANT"
	if !rep.Summary.Conformant {
		verdict = "NON-CONFORMANT"
	}
	s := rep.Summary
	fmt.Fprintf(w, "%s [%s] %s: %d cases  pass=%d fail=%d error=%d unsupported=%d na=%d\n",
		rep.Adapter, rep.Profile, verdict, s.Total, s.Pass, s.Fail, s.Error, s.Unsupported, s.NA)
}
