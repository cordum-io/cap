// Command cap-governance runs CAP's machine-checkable governance and readiness
// validators. It is offline and read-only: it makes no network calls and mutates only
// files it is explicitly asked to render.
package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Fprint(os.Stderr, `cap-governance — CAP governance & readiness checks

usage:
  cap-governance check      --root DIR [--as-of YYYY-MM-DD]
      Validate RFC metadata/chronology, lint outward claims + draft banners,
      and structurally validate the readiness manifest. Nonzero on any problem.

  cap-governance dco        --range BASE..HEAD
      Verify DCO sign-off for every non-allowlisted commit in the range.

  cap-governance readiness  --root DIR [--require-foundation-ready]
      Print the computed readiness verdict. Exits 0 for a truthful BLOCKED/UNKNOWN
      manifest; --require-foundation-ready exits nonzero until every dimension PASSes.

  cap-governance render     --root DIR [--check]
      Regenerate governance/READINESS.md from the manifest. --check fails if stale.
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	run := map[string]func([]string) (int, error){
		"check":     runCheck,
		"dco":       runDCO,
		"readiness": runReadiness,
		"render":    runRender,
		"triage":    runTriage,
	}[os.Args[1]]
	if run == nil {
		usage()
		os.Exit(2)
	}
	code, err := run(os.Args[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	os.Exit(code)
}
