// Command cap-tck runs CAP conformance scenarios against adapter processes and
// emits deterministic JSON/JUnit evidence. Reports are local/CI evidence, not
// certification or an external interoperability claim.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// dispatch routes a subcommand. The hidden __adapter mode runs the built-in
// reference adapter so the harness can be exercised end to end from one binary.
func dispatch(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "run":
		return cmdRun(args[1:], stdout, stderr)
	case "list":
		return cmdList(args[1:], stdout, stderr)
	case "matrix":
		return cmdMatrix(args[1:], stdout, stderr)
	case "__adapter":
		return runRefAdapter(args[1:], stdin, stdout)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "cap-tck: unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `cap-tck — CAP Technical Conformance Kit

Usage:
  cap-tck run    --suite <path> --adapter 'label=argv0|arg1|...' [--profile core] [--json -] [--junit out.xml]
  cap-tck matrix --suite <path> --adapter A --adapter B [--profile core]
  cap-tck list   --suite <path> [--profile core]

An adapter is an external process speaking adapter-v1 (see spec/tck/adapter-v1.md).
The built-in reference adapter (cap-tck __adapter --role worker) tests the harness
itself and is not an independent conformant implementation.

Exit codes: 0 conformant · 1 non-conformant or adapter failure · 2 usage error.
`)
}
