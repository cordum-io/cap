package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	gov "github.com/cordum-io/cap/v2/internal/governance"
)

const manifestRel = "governance/readiness.json"
const readinessDocRel = "governance/READINESS.md"

func loadReadiness(root string) (*gov.Manifest, gov.Readiness, error) {
	data, err := os.ReadFile(filepath.Join(root, manifestRel))
	if err != nil {
		return nil, gov.Readiness{}, err
	}
	m, err := gov.LoadManifest(data)
	if err != nil {
		return nil, gov.Readiness{}, err
	}
	return m, gov.Evaluate(m), nil
}

func runReadiness(args []string) (int, error) {
	fs := flag.NewFlagSet("readiness", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root")
	requireReady := fs.Bool("require-foundation-ready", false, "exit nonzero unless verdict is READY")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	_, r, err := loadReadiness(*root)
	if err != nil {
		return 1, err
	}
	fmt.Printf("readiness verdict: %s\n", r.Verdict)
	for _, d := range r.Dimensions {
		fmt.Printf("  %-34s %-8s %d/%d\n", d.Name, d.Status, d.Counting, d.Required)
	}
	if *requireReady && r.Verdict != gov.VerdictReady {
		return 1, fmt.Errorf("--require-foundation-ready: verdict is %s, not READY", r.Verdict)
	}
	return 0, nil
}

func runRender(args []string) (int, error) {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root")
	check := fs.Bool("check", false, "fail if the rendered doc is stale instead of writing it")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	m, r, err := loadReadiness(*root)
	if err != nil {
		return 1, err
	}
	want := gov.Render(m, r)
	path := filepath.Join(*root, readinessDocRel)
	if *check {
		got, _ := os.ReadFile(path)
		if normalizeEOL(string(got)) != normalizeEOL(want) {
			return 1, fmt.Errorf("%s is stale; run `cap-governance render`", readinessDocRel)
		}
		return 0, nil
	}
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		return 1, err
	}
	fmt.Printf("wrote %s (verdict %s)\n", readinessDocRel, r.Verdict)
	return 0, nil
}
