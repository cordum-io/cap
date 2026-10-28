package tck

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// diffSuiteInventory compares the hard-coded shippedSuiteFiles list against
// the *.json files found on disk. All results are sorted. A listed entry that
// is not a plain base name can never match a directory entry, so it is
// reported as missing.
func diffSuiteInventory(listed, onDisk []string) (unlisted, missing, duplicates []string) {
	disk := map[string]bool{}
	for _, f := range onDisk {
		disk[f] = true
	}
	seen := map[string]int{}
	for _, e := range listed {
		seen[e]++
	}
	for e, n := range seen {
		if n > 1 {
			duplicates = append(duplicates, e)
		}
		if strings.ContainsAny(e, `/\`) || filepath.Base(e) != e || !disk[e] {
			missing = append(missing, e)
		}
	}
	for f := range disk {
		if seen[f] == 0 {
			unlisted = append(unlisted, f)
		}
	}
	sort.Strings(unlisted)
	sort.Strings(missing)
	sort.Strings(duplicates)
	return unlisted, missing, duplicates
}

func TestDiffSuiteInventory(t *testing.T) {
	cases := []struct {
		name                         string
		listed, onDisk               []string
		unlisted, missing, duplicate []string
	}{
		{name: "equal sets", listed: []string{"core-a.json", "core-b.json"}, onDisk: []string{"core-b.json", "core-a.json"}},
		{name: "extra on disk", listed: []string{"core-lifecycle.json"}, onDisk: []string{"core-lifecycle.json", "zz-new.json"}, unlisted: []string{"zz-new.json"}},
		{name: "listed but absent", listed: []string{"core-lifecycle.json", "core-ghost.json"}, onDisk: []string{"core-lifecycle.json"}, missing: []string{"core-ghost.json"}},
		{name: "duplicate entry", listed: []string{"core-lifecycle.json", "core-lifecycle.json"}, onDisk: []string{"core-lifecycle.json"}, duplicate: []string{"core-lifecycle.json"}},
		{name: "path entry never matches", listed: []string{"sub/core-x.json"}, onDisk: []string{"core-x.json"}, unlisted: []string{"core-x.json"}, missing: []string{"sub/core-x.json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, m, d := diffSuiteInventory(tc.listed, tc.onDisk)
			for _, p := range []struct {
				label     string
				got, want []string
			}{{"unlisted", u, tc.unlisted}, {"missing", m, tc.missing}, {"duplicates", d, tc.duplicate}} {
				if len(p.got) == 0 && len(p.want) == 0 {
					continue
				}
				if !reflect.DeepEqual(p.got, p.want) {
					t.Errorf("%s = %v, want %v", p.label, p.got, p.want)
				}
			}
		})
	}
}

// shippedSuiteFiles is a deliberate inventory; this keeps it in lockstep with
// spec/tck/scenarios so a new suite cannot escape the validity, uniqueness and
// coverage checks.
func TestShippedSuiteFilesMatchScenarioDir(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join(scenariosDir(), "*.json"))
	if err != nil {
		t.Fatalf("glob scenarios dir: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no *.json found under %s; the inventory check would be vacuous", scenariosDir())
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, filepath.Base(m))
	}
	unlisted, missing, duplicates := diffSuiteInventory(shippedSuiteFiles, names)
	for _, f := range unlisted {
		t.Errorf("spec/tck/scenarios/%s is not in shippedSuiteFiles (internal/tck/scenarios_test.go): add it so validity, uniqueness and coverage checks see it", f)
	}
	for _, f := range missing {
		t.Errorf("shippedSuiteFiles entry %q has no matching spec/tck/scenarios/*.json file: remove or fix it", f)
	}
	for _, f := range duplicates {
		t.Errorf("shippedSuiteFiles lists %q more than once: remove the duplicate", f)
	}
}
