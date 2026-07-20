package sdksupport

import (
	"io/fs"
	"strings"
)

var validTiers = map[Tier]bool{
	TierStable:       true,
	TierCommunity:    true,
	TierExperimental: true,
}

var validKinds = map[Kind]bool{
	KindProtocolSDK: true,
	KindExtension:   true,
}

var validGateKinds = map[GateKind]bool{
	GateWorkflow: true,
	GateTest:     true,
	GateManifest: true,
}

// Verify checks the manifest against evidence resolvable in root and returns
// every problem found. An empty result means the manifest is fully backed by
// files that exist. It never trusts a hand-set boolean: every claim is a path
// resolved against root.
func Verify(m *Manifest, root fs.FS) []Problem {
	var problems []Problem
	seenID := map[string]bool{}
	seenPkg := map[string]bool{}
	for _, e := range m.Entries {
		problems = append(problems, verifyEntry(e, root)...)
		if seenID[e.ID] {
			problems = append(problems, prob(e.ID, CodeDuplicateID, e.ID))
		}
		seenID[e.ID] = true
		if key, ok := packageKey(e.Package); ok {
			if seenPkg[key] {
				problems = append(problems, prob(e.ID, CodeDuplicatePackage, key))
			}
			seenPkg[key] = true
		}
	}
	return problems
}

// verifyEntry checks a single entry's shape, ownership, and documentation.
func verifyEntry(e Entry, root fs.FS) []Problem {
	var p []Problem
	if !validTiers[e.Tier] {
		p = append(p, prob(e.ID, CodeUnknownTier, string(e.Tier)))
	}
	if !validKinds[e.Kind] {
		p = append(p, prob(e.ID, CodeUnknownKind, string(e.Kind)))
	}
	if e.Owner == "" {
		p = append(p, prob(e.ID, CodeMissingOwner, ""))
	}
	if e.OwnerPath == "" {
		p = append(p, prob(e.ID, CodeOwnerPathMissing, "empty"))
	} else if !exists(root, strings.TrimSuffix(e.OwnerPath, "/")) {
		p = append(p, prob(e.ID, CodeOwnerPathMissing, e.OwnerPath))
	}
	if e.Docs == "" {
		p = append(p, prob(e.ID, CodeMissingDocs, ""))
	} else if !exists(root, e.Docs) {
		p = append(p, prob(e.ID, CodeDocsPathMissing, e.Docs))
	}
	return append(p, verifyGates(e, root)...)
}

// verifyGates resolves each declared gate and enforces the required stable set.
// Required gates apply only to a stable protocol SDK: an extension is never
// forced to carry a wire-conformance gate it has no business declaring.
func verifyGates(e Entry, root fs.FS) []Problem {
	var p []Problem
	seen := map[string]bool{}
	have := map[string]bool{}
	for _, g := range e.Gates {
		if !validGateKinds[g.Kind] {
			p = append(p, prob(e.ID, CodeUnknownGateKind, string(g.Kind)))
		}
		if seen[g.ID] {
			p = append(p, prob(e.ID, CodeDuplicateGateID, g.ID))
		}
		seen[g.ID] = true
		have[g.ID] = true
		if g.Path == "" || !exists(root, g.Path) {
			p = append(p, prob(e.ID, CodeGatePathMissing, g.Path))
		}
	}
	if e.Tier == TierStable && e.Kind == KindProtocolSDK {
		for _, req := range RequiredStableGates {
			if !have[req] {
				p = append(p, prob(e.ID, CodeMissingRequiredGate, req))
			}
		}
	}
	return p
}

func prob(id, code, detail string) Problem {
	return Problem{EntryID: id, Code: code, Detail: detail}
}

// packageKey returns a stable identity for a package coordinate, or ok=false
// when the entry declares no package (nothing to collide on).
func packageKey(p *Package) (string, bool) {
	if p == nil {
		return "", false
	}
	return p.Registry + "\x00" + p.Coordinate, true
}

// exists reports whether name resolves to a file in root. A path that is not a
// valid slash-rooted FS path can never be evidence, so it is treated as absent.
func exists(root fs.FS, name string) bool {
	if !fs.ValidPath(name) {
		return false
	}
	_, err := fs.Stat(root, name)
	return err == nil
}
