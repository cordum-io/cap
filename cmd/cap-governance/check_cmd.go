package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	gov "github.com/cordum-io/cap/v2/internal/governance"
)

// outwardDirs are scanned for fabricated marketing claims. partnershipDirs must also
// carry the DRAFT banner on every asset.
var outwardDirs = []string{"launch-kit", "partnerships"}
var partnershipDirs = []string{"partnerships"}

func runCheck(args []string) (int, error) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root")
	asOfStr := fs.String("as-of", "", "reference date YYYY-MM-DD (default today)")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	now := time.Now()
	if *asOfStr != "" {
		t, err := time.Parse("2006-01-02", *asOfStr)
		if err != nil {
			return 2, fmt.Errorf("bad --as-of: %w", err)
		}
		now = t
	}
	var problems []string
	problems = append(problems, checkRFCs(*root, now)...)
	problems = append(problems, checkClaims(*root)...)
	problems = append(problems, checkReadinessStructure(*root)...)

	if len(problems) == 0 {
		fmt.Println("governance check: OK")
		return 0, nil
	}
	fmt.Printf("governance check: %d problem(s)\n", len(problems))
	for _, p := range problems {
		fmt.Println("  - " + p)
	}
	return 1, nil
}

func checkRFCs(root string, now time.Time) []string {
	var problems []string
	dir := filepath.Join(root, "rfcs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // no rfcs dir yet is not a failure
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("rfcs", e.Name()))
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			problems = append(problems, rel+": "+err.Error())
			continue
		}
		r, err := gov.ParseRFC(rel, data)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		for _, p := range r.Validate(now) {
			problems = append(problems, rel+": "+p)
		}
	}
	return problems
}

func checkClaims(root string) []string {
	var problems []string
	for _, d := range outwardDirs {
		_ = filepath.WalkDir(filepath.Join(root, d), func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			data, _ := os.ReadFile(path)
			rel, _ := filepath.Rel(root, path)
			for _, h := range gov.LintClaims(filepath.ToSlash(rel), data) {
				problems = append(problems, fmt.Sprintf("%s:%d: %s (%q)", h.File, h.Line, h.Rule, h.Text))
			}
			return nil
		})
	}
	for _, d := range partnershipDirs {
		_ = filepath.WalkDir(filepath.Join(root, d), func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			data, _ := os.ReadFile(path)
			rel, _ := filepath.Rel(root, path)
			if gov.RequireDraftBanner(data) {
				problems = append(problems, filepath.ToSlash(rel)+": missing required DRAFT banner")
			}
			return nil
		})
	}
	return problems
}

func checkReadinessStructure(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, manifestRel))
	if err != nil {
		return nil // absence handled elsewhere; check focuses on validity when present
	}
	if _, err := gov.LoadManifest(data); err != nil {
		return []string{manifestRel + ": " + err.Error()}
	}
	return nil
}
