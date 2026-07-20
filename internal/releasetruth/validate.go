package releasetruth

import (
	"regexp"
	"strings"
)

var (
	reSemver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	reDate   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reHexSHA = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

var (
	validChannels  = map[string]bool{"stable": true, "beta": true, "rc": true}
	validKinds     = map[string]bool{"sdk": true, "extension": true}
	validTiers     = map[string]bool{"stable": true, "experimental": true, "community": true}
	validTransport = map[string]bool{"supported": true, "experimental": true}
)

// Validate runs every semantic check and returns all problems (empty slice
// means valid). It performs no I/O or network access and is safe to run in a
// deterministic PR gate.
func Validate(m *Manifest) []Problem {
	var ps []Problem
	ps = append(ps, checkSchema(m)...)
	ps = append(ps, checkRelease(m)...)
	ps = append(ps, checkCandidate(m)...)
	ps = append(ps, checkDevelopment(m)...)
	ps = append(ps, checkWire(m)...)
	ps = append(ps, checkSpecs(m)...)
	ps = append(ps, checkComponents(m)...)
	ps = append(ps, checkTransports(m)...)
	ps = append(ps, checkToolchains(m)...)
	ps = append(ps, checkSecurity(m)...)
	ps = append(ps, checkLinks(m)...)
	return ps
}

func checkSchema(m *Manifest) []Problem {
	if !reSemver.MatchString(m.SchemaVersion) {
		return []Problem{{"schemaVersion", "must be MAJOR.MINOR.PATCH: " + m.SchemaVersion}}
	}
	return nil
}

func checkRelease(m *Manifest) []Problem {
	var ps []Problem
	r := m.Release
	if !reSemver.MatchString(r.Version) {
		ps = append(ps, Problem{"release.version", "must be MAJOR.MINOR.PATCH: " + r.Version})
	}
	if r.Tag != "v"+r.Version {
		ps = append(ps, Problem{"release.tag", "tag must equal v" + r.Version + ", got " + r.Tag})
	}
	if !reDate.MatchString(r.Date) {
		ps = append(ps, Problem{"release.date", "must be YYYY-MM-DD: " + r.Date})
	}
	if !reHexSHA.MatchString(r.Commit) {
		ps = append(ps, Problem{"release.commit", "must be a 7-40 char hex sha: " + r.Commit})
	}
	if !validChannels[r.Channel] {
		ps = append(ps, Problem{"release.channel", "unknown channel: " + r.Channel})
	}
	return ps
}

func checkCandidate(m *Manifest) []Problem {
	if m.Candidate == nil {
		return nil
	}
	c := *m.Candidate
	var ps []Problem
	if !reSemver.MatchString(c.Version) {
		ps = append(ps, Problem{"candidate.version", "must be MAJOR.MINOR.PATCH: " + c.Version})
	} else if reSemver.MatchString(m.Release.Version) && !semverGreater(c.Version, m.Release.Version) {
		ps = append(ps, Problem{"candidate.version", "must be newer than published release " + m.Release.Version})
	}
	if c.Tag != "v"+c.Version {
		ps = append(ps, Problem{"candidate.tag", "tag must equal v" + c.Version + ", got " + c.Tag})
	}
	if !validChannels[c.Channel] {
		ps = append(ps, Problem{"candidate.channel", "unknown channel: " + c.Channel})
	}
	return ps
}

func semverGreater(candidate, published string) bool {
	candidateParts := strings.Split(candidate, ".")
	publishedParts := strings.Split(published, ".")
	for i := 0; i < 3; i++ {
		candidatePart := strings.TrimLeft(candidateParts[i], "0")
		publishedPart := strings.TrimLeft(publishedParts[i], "0")
		if candidatePart == "" {
			candidatePart = "0"
		}
		if publishedPart == "" {
			publishedPart = "0"
		}
		if len(candidatePart) != len(publishedPart) {
			return len(candidatePart) > len(publishedPart)
		}
		if candidatePart != publishedPart {
			return candidatePart > publishedPart
		}
	}
	return false
}

func checkDevelopment(m *Manifest) []Problem {
	var ps []Problem
	d := m.Development
	if d.Released {
		ps = append(ps, Problem{"development.released", "development state must be unreleased (released=false)"})
	}
	if strings.TrimSpace(d.Describe) == "" {
		ps = append(ps, Problem{"development.describe", "must record git describe output"})
	}
	if d.Commit != "" && !reHexSHA.MatchString(d.Commit) {
		ps = append(ps, Problem{"development.commit", "must be a 7-40 char hex sha: " + d.Commit})
	}
	return ps
}

func checkWire(m *Manifest) []Problem {
	var ps []Problem
	w := m.Wire
	if w.ProtocolVersion < 1 {
		ps = append(ps, Problem{"wire.protocolVersion", "must be >= 1"})
	}
	if !reSemver.MatchString(w.SchemaVersion) {
		ps = append(ps, Problem{"wire.schemaVersion", "must be MAJOR.MINOR.PATCH: " + w.SchemaVersion})
	}
	if w.CompatMin < 1 || w.CompatMax < w.CompatMin ||
		w.ProtocolVersion < w.CompatMin || w.ProtocolVersion > w.CompatMax {
		ps = append(ps, Problem{"wire.compat", "protocolVersion must lie within [compatMin,compatMax] and compatMin>=1"})
	}
	return ps
}

func checkSpecs(m *Manifest) []Problem {
	if len(m.Specs) == 0 {
		return []Problem{{"specs.empty", "at least one normative spec is required"}}
	}
	var ps []Problem
	seen := map[string]bool{}
	for _, s := range m.Specs {
		if s.ID == "" || s.Title == "" {
			ps = append(ps, Problem{"specs.field", "spec id and title are required"})
		}
		if seen[s.ID] {
			ps = append(ps, Problem{"specs.id.duplicate", "duplicate spec id: " + s.ID})
		}
		seen[s.ID] = true
		if err := safeRelPath(s.File); err != nil {
			ps = append(ps, Problem{"specs.file", "unsafe spec path " + s.File + ": " + err.Error()})
		}
	}
	return ps
}

func checkComponents(m *Manifest) []Problem {
	if len(m.Components) == 0 {
		return []Problem{{"components.empty", "at least one component is required"}}
	}
	var ps []Problem
	seen := map[string]bool{}
	for _, c := range m.Components {
		ps = append(ps, checkComponent(c, seen)...)
	}
	return ps
}

func checkComponent(c Component, seen map[string]bool) []Problem {
	var ps []Problem
	if seen[c.Name] {
		ps = append(ps, Problem{"components.name.duplicate", "duplicate component name: " + c.Name})
	}
	seen[c.Name] = true
	if !validKinds[c.Kind] {
		ps = append(ps, Problem{"components.kind", "unknown kind for " + c.Name + ": " + c.Kind})
	}
	if !validTiers[c.Tier] {
		ps = append(ps, Problem{"components.tier", "unknown tier for " + c.Name + ": " + c.Tier})
	}
	if isGuard(c) && c.Kind != "extension" {
		ps = append(ps, Problem{"components.guard.kind", "Guard must be kind=extension, not a protocol SDK"})
	}
	if c.Tier == "stable" {
		ps = append(ps, checkStableComponent(c)...)
	}
	return ps
}

func checkStableComponent(c Component) []Problem {
	var ps []Problem
	if c.Package == "" || c.Registry == "" || c.Import == "" || c.Version == "" {
		ps = append(ps, Problem{"components.stable.coordinates",
			"stable component " + c.Name + " needs package, registry, import, and version"})
	}
	if strings.TrimSpace(c.Evidence) == "" {
		ps = append(ps, Problem{"components.stable.evidence",
			"stable component " + c.Name + " needs behavioral evidence"})
	}
	return ps
}

func isGuard(c Component) bool {
	return strings.Contains(strings.ToLower(c.Name), "guard") ||
		strings.Contains(strings.ToLower(c.Package), "guard")
}

func checkTransports(m *Manifest) []Problem {
	var ps []Problem
	seen := map[string]bool{}
	for _, tr := range m.Transports {
		if seen[tr.Name] {
			ps = append(ps, Problem{"transports.name.duplicate", "duplicate transport: " + tr.Name})
		}
		seen[tr.Name] = true
		if !validTransport[tr.State] {
			ps = append(ps, Problem{"transports.state", "unknown state for " + tr.Name + ": " + tr.State})
		}
		if tr.State == "supported" && strings.TrimSpace(tr.Evidence) == "" {
			ps = append(ps, Problem{"transports.supported.evidence",
				tr.Name + " marked supported without behavioral evidence"})
		}
	}
	return ps
}

func checkToolchains(m *Manifest) []Problem {
	var ps []Problem
	pairs := []struct {
		field string
		req   ToolReq
	}{
		{"toolchains.go", m.Toolchains.Go},
		{"toolchains.node", m.Toolchains.Node},
		{"toolchains.python", m.Toolchains.Python},
	}
	for _, p := range pairs {
		if strings.TrimSpace(p.req.Tested) == "" || strings.TrimSpace(p.req.Minimum) == "" {
			ps = append(ps, Problem{p.field, "tested and minimum toolchain versions are required"})
		}
	}
	return ps
}

func checkSecurity(m *Manifest) []Problem {
	var ps []Problem
	if len(m.Security.SupportedLines) == 0 {
		ps = append(ps, Problem{"security.supportedLines", "at least one supported release line is required"})
	}
	if strings.TrimSpace(m.Security.ReportingRoute) == "" {
		ps = append(ps, Problem{"security.reportingRoute", "a vulnerability reporting route is required"})
	}
	return ps
}

func checkLinks(m *Manifest) []Problem {
	var ps []Problem
	seen := map[string]bool{}
	for _, l := range m.Links {
		if seen[l.Name] {
			ps = append(ps, Problem{"links.name.duplicate", "duplicate link name: " + l.Name})
		}
		seen[l.Name] = true
		if !validLinkURL(l.URL) {
			ps = append(ps, Problem{"links.url", "link " + l.Name + " has an unsafe or empty url: " + l.URL})
		}
	}
	return ps
}

func validLinkURL(u string) bool {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "#") {
		return true
	}
	return safeRelPath(u) == nil
}
