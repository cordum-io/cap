// Command cap-support verifies the machine-readable SDK support manifest against
// real evidence and generates the support table in docs from it.
//
//	cap-support check    verify sdk/support-tiers.json + doc-table freshness
//	cap-support render   regenerate the doc table from the manifest
//
// The manifest never carries hand-set pass booleans: every claim is a gate that
// names a real workflow, test, or package file, and Verify resolves each path.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	sdksupport "github.com/cordum-io/cap/v2/tools/sdk-support"
)

const (
	manifestPath = "sdk/support-tiers.json"
	docPath      = "docs/sdk-support.md"
)

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: cap-support check|render")
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(stdout, stderr)
	case "render":
		return runRender(stderr)
	default:
		fmt.Fprintf(stderr, "cap-support: unknown command %q\n", args[0])
		return 2
	}
}

func runCheck(stdout, stderr io.Writer) int {
	m, err := loadManifest()
	if err != nil {
		fmt.Fprintf(stderr, "cap-support: %v\n", err)
		return 1
	}
	if problems := sdksupport.Verify(m, os.DirFS(".")); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(stderr, "UNVERIFIED %s %s %s\n", p.EntryID, p.Code, p.Detail)
		}
		return 1
	}
	doc, err := os.ReadFile(docPath)
	if err != nil {
		fmt.Fprintf(stderr, "cap-support: %v\n", err)
		return 1
	}
	have, err := currentTable(string(doc))
	if err != nil {
		fmt.Fprintf(stderr, "cap-support: %s: %v\n", docPath, err)
		return 1
	}
	if have != renderTable(m) {
		fmt.Fprintf(stderr, "cap-support: %s table is stale; run 'cap-support render'\n", docPath)
		return 1
	}
	fmt.Fprintf(stdout, "cap-support: %d entries verified; doc table current\n", len(m.Entries))
	return 0
}

func runRender(stderr io.Writer) int {
	m, err := loadManifest()
	if err != nil {
		fmt.Fprintf(stderr, "cap-support: %v\n", err)
		return 1
	}
	doc, err := os.ReadFile(docPath)
	if err != nil {
		fmt.Fprintf(stderr, "cap-support: %v\n", err)
		return 1
	}
	updated, err := spliceTable(string(doc), renderTable(m))
	if err != nil {
		fmt.Fprintf(stderr, "cap-support: %s: %v\n", docPath, err)
		return 1
	}
	if err := os.WriteFile(docPath, []byte(updated), 0o644); err != nil {
		fmt.Fprintf(stderr, "cap-support: write %s: %v\n", docPath, err)
		return 1
	}
	fmt.Fprintf(stderr, "cap-support: rendered %s\n", docPath)
	return 0
}

func loadManifest() (*sdksupport.Manifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var m sdksupport.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestPath, err)
	}
	return &m, nil
}
