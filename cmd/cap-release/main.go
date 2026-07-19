// Command cap-release validates the canonical CAP release manifest. It is the
// deterministic, network-free gate that keeps release, wire, SDK, transport,
// toolchain, and security-support truth internally consistent and in sync with
// the specs on disk.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cordum-io/cap/v2/internal/releasetruth"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: cap-release <check|render|links> [flags]")
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:])
	case "render":
		return runRender(args[1:])
	case "links":
		return runLinks(args[1:])
	case "release-check":
		return runReleaseCheck(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (supported: check, render, links, release-check)\n", args[0])
		return 2
	}
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "release/manifest.json", "path to the release manifest")
	repoRoot := fs.String("repo-root", ".", "repository root for spec-on-disk checks")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	m, err := releasetruth.LoadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		return 1
	}
	problems := releasetruth.Validate(m)
	problems = append(problems, releasetruth.CheckSpecsOnDisk(m, *repoRoot)...)
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "release-truth check FAILED: %d problem(s)\n", len(problems))
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", p.Error())
		}
		return 1
	}
	fmt.Printf("release-truth check OK: %s v%s (wire %d), %d specs, %d components\n",
		*manifestPath, m.Release.Version, m.Wire.ProtocolVersion, len(m.Specs), len(m.Components))
	return 0
}
