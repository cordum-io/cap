package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// LoadManifest strictly decodes a readiness manifest. Unknown fields are rejected so
// that a hand-set "verdict" or "ready" boolean cannot sneak in — the verdict is always
// computed. Structural Validate runs before the manifest is returned.
func LoadManifest(data []byte) (*Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

var validSurfaceStatus = map[string]bool{"pass": true, "broken": true, "unknown": true}

// Validate checks structural integrity only — not the verdict. It fails on schema drift,
// unknown enum values, and duplicate identifiers.
func Validate(m *Manifest) error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %q (want %q)", m.SchemaVersion, SchemaVersion)
	}
	if err := uniqueIDs("implementation", implIDs(m.Implementations)); err != nil {
		return err
	}
	seenSurface := map[string]bool{}
	for _, s := range m.OnboardingSurfaces {
		if !validSurfaceStatus[s.Status] {
			return fmt.Errorf("onboarding surface %q: invalid status %q (want pass|broken|unknown)", s.ID, s.Status)
		}
		if seenSurface[s.ID] {
			return fmt.Errorf("duplicate onboarding surface id %q", s.ID)
		}
		seenSurface[s.ID] = true
	}
	return nil
}

func implIDs(impls []Implementation) []string {
	ids := make([]string, len(impls))
	for i, im := range impls {
		ids[i] = im.ID
	}
	return ids
}

func uniqueIDs(kind string, ids []string) error {
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" {
			return fmt.Errorf("%s with empty id", kind)
		}
		if seen[id] {
			return fmt.Errorf("duplicate %s id %q", kind, id)
		}
		seen[id] = true
	}
	return nil
}

// Render produces the generated READINESS.md body from evidence + computed verdict.
func Render(m *Manifest, r Readiness) string {
	var b strings.Builder
	b.WriteString("# CAP Foundation / Adoption Readiness\n\n")
	b.WriteString("> Generated from `governance/readiness.json`. Do not edit by hand. The verdict is\n")
	b.WriteString("> COMPUTED from evidence; it is never a stored boolean.\n\n")
	fmt.Fprintf(&b, "## Verdict: **%s**\n\n", r.Verdict)
	if r.Verdict == VerdictBlocked {
		b.WriteString("Readiness is **BLOCKED**. This is the honest current state: one or more required\n")
		b.WriteString("dimensions is not satisfied by verifiable, consented, non-Cordum evidence.\n\n")
	}
	b.WriteString("| Dimension | Status | Counting / Required |\n|---|---|---|\n")
	for _, d := range r.Dimensions {
		fmt.Fprintf(&b, "| %s | **%s** | %d / %d |\n", d.Name, d.Status, d.Counting, d.Required)
	}
	b.WriteString("\n## Why evidence did not count\n\n")
	notes := collectNotes(r)
	if len(notes) == 0 {
		b.WriteString("_All submitted evidence counted._\n")
	}
	for _, n := range notes {
		fmt.Fprintf(&b, "- %s\n", n)
	}
	b.WriteString("\n## What never counts\n\n")
	b.WriteString("The CAP reference scheduler, Cordum, framework adapters, SDK wrappers, private\n")
	b.WriteString("interest, stars/downloads, target lists, and design-partner drafts are excluded by\n")
	b.WriteString("rule and can never move this verdict. A PASS here is necessary evidence, never\n")
	b.WriteString("permission to launch, announce, or submit to a foundation.\n")
	return b.String()
}

func collectNotes(r Readiness) []string {
	var notes []string
	for _, d := range r.Dimensions {
		notes = append(notes, d.Notes...)
	}
	sort.Strings(notes)
	return notes
}
