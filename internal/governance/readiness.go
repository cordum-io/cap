// Package governance provides machine-checkable validators for CAP's community
// governance and adoption/foundation-readiness evidence. Nothing here trusts a
// hand-set verdict: the readiness verdict is COMPUTED from evidence every time.
package governance

import "strings"

// SchemaVersion is the only readiness manifest schema this tool understands.
const SchemaVersion = "readiness-v1"

// Dimension names (stable identifiers used in output and tests).
const (
	DimImplementations = "two-independent-implementations"
	DimAdopters        = "two-public-external-adopters"
	DimMaintainer      = "one-independent-maintainer"
	DimOnboarding      = "zero-broken-onboarding-surfaces"
)

// Dimension status values. UNKNOWN means "evidence is missing/stale/unverifiable",
// which is deliberately NOT the same as a passing zero.
type DimStatus string

const (
	StatusPass    DimStatus = "PASS"
	StatusUnknown DimStatus = "UNKNOWN"
	StatusFail    DimStatus = "FAIL"
)

// Aggregate verdicts.
const (
	VerdictReady   = "READY"
	VerdictBlocked = "BLOCKED"
)

// Required minimums per the epic definition of done.
const (
	reqImplementations = 2
	reqAdopters        = 2
	reqMaintainers     = 1
)

// requiredSurfaces are the primary onboarding surfaces that must all be healthy.
var requiredSurfaces = []string{"root", "go", "python", "node"}

// TCKAttestation is a self-hosted TCK report reference. Its presence is necessary
// evidence for an implementation; it is never proof of certification or endorsement.
type TCKAttestation struct {
	Profile      string `json:"profile"`
	Version      string `json:"version"`
	TCKVersion   string `json:"tckVersion"`
	ReportDigest string `json:"reportDigest"`
	URL          string `json:"url"`
}

func (a *TCKAttestation) complete() bool {
	return a != nil && a.Profile != "" && a.Version != "" && a.TCKVersion != "" && a.ReportDigest != ""
}

// Implementation is a claimed independent CAP implementation.
type Implementation struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	URL                string          `json:"url"`
	Affiliation        string          `json:"affiliation"`
	IsFork             bool            `json:"isFork"`
	ControlledByCordum bool            `json:"controlledByCordum"`
	Attestation        *TCKAttestation `json:"tckAttestation"`
}

// Adopter is a claimed adopter-controlled public statement of real use.
type Adopter struct {
	Name               string `json:"name"`
	StatementURL       string `json:"statementUrl"`
	Consent            bool   `json:"consent"`
	Profile            string `json:"profile"`
	Version            string `json:"version"`
	NonDemo            bool   `json:"nonDemo"`
	ControlledByCordum bool   `json:"controlledByCordum"`
}

// Maintainer is a claimed project maintainer with independent standing.
type Maintainer struct {
	Handle              string `json:"handle"`
	Affiliation         string `json:"affiliation"`
	IndependentOfCordum bool   `json:"independentOfCordum"`
	RightsEvidenceURL   string `json:"rightsEvidenceUrl"`
}

// OnboardingSurface is a primary onboarding gate result at the current commit.
type OnboardingSurface struct {
	ID       string `json:"id"`
	Status   string `json:"status"` // pass | broken | unknown
	Skipped  bool   `json:"skipped"`
	Commit   string `json:"commit"`
	Evidence string `json:"evidence"`
}

// Manifest is the raw readiness evidence registry. It carries no verdict field on
// purpose — the verdict is always computed by Evaluate.
type Manifest struct {
	SchemaVersion      string              `json:"schemaVersion"`
	Implementations    []Implementation    `json:"implementations"`
	Adopters           []Adopter           `json:"adopters"`
	Maintainers        []Maintainer        `json:"maintainers"`
	OnboardingSurfaces []OnboardingSurface `json:"onboardingSurfaces"`
}

// Dimension is the evaluated status of one readiness dimension.
type Dimension struct {
	Name     string    `json:"name"`
	Status   DimStatus `json:"status"`
	Required int       `json:"required"`
	Counting int       `json:"counting"`
	Notes    []string  `json:"notes"`
}

// Readiness is the computed aggregate.
type Readiness struct {
	Verdict    string      `json:"verdict"`
	Dimensions []Dimension `json:"dimensions"`
}

func isCordum(affiliation string) bool {
	return strings.EqualFold(strings.TrimSpace(affiliation), "cordum")
}

// Evaluate computes the readiness verdict purely from evidence. The verdict is READY
// only if every dimension PASSes; any UNKNOWN or FAIL yields BLOCKED.
func Evaluate(m *Manifest) Readiness {
	dims := []Dimension{
		evalImplementations(m.Implementations),
		evalAdopters(m.Adopters),
		evalMaintainers(m.Maintainers),
		evalOnboarding(m.OnboardingSurfaces),
	}
	verdict := VerdictReady
	for _, d := range dims {
		if d.Status != StatusPass {
			verdict = VerdictBlocked
			break
		}
	}
	return Readiness{Verdict: verdict, Dimensions: dims}
}

// countThreshold turns a counting/required pair into PASS or UNKNOWN. Absence of
// enough positive evidence is UNKNOWN, never a passing zero.
func countThreshold(name string, counting, required int, notes []string) Dimension {
	st := StatusUnknown
	if counting >= required {
		st = StatusPass
	}
	return Dimension{Name: name, Status: st, Required: required, Counting: counting, Notes: notes}
}

func evalImplementations(impls []Implementation) Dimension {
	counting, notes := 0, []string{}
	for _, i := range impls {
		switch {
		case i.IsFork:
			notes = append(notes, i.ID+": fork — excluded")
		case i.ControlledByCordum || isCordum(i.Affiliation):
			notes = append(notes, i.ID+": Cordum-controlled/affiliated — excluded")
		case i.URL == "":
			notes = append(notes, i.ID+": no public source/artifact URL — UNKNOWN")
		case !i.Attestation.complete():
			notes = append(notes, i.ID+": missing/stale TCK attestation — UNKNOWN")
		default:
			counting++
		}
	}
	return countThreshold(DimImplementations, counting, reqImplementations, notes)
}

func evalAdopters(adopters []Adopter) Dimension {
	counting, notes := 0, []string{}
	for _, a := range adopters {
		switch {
		case a.ControlledByCordum:
			notes = append(notes, a.Name+": Cordum-controlled — excluded")
		case !a.Consent:
			notes = append(notes, a.Name+": no recorded consent — UNKNOWN")
		case a.StatementURL == "":
			notes = append(notes, a.Name+": no public statement — UNKNOWN")
		case !a.NonDemo:
			notes = append(notes, a.Name+": demo-only use — excluded")
		case a.Profile == "" || a.Version == "":
			notes = append(notes, a.Name+": missing profile/version — UNKNOWN")
		default:
			counting++
		}
	}
	return countThreshold(DimAdopters, counting, reqAdopters, notes)
}

func evalMaintainers(ms []Maintainer) Dimension {
	counting, notes := 0, []string{}
	for _, m := range ms {
		switch {
		case !m.IndependentOfCordum || isCordum(m.Affiliation):
			notes = append(notes, m.Handle+": not independent of Cordum — excluded")
		case m.RightsEvidenceURL == "":
			notes = append(notes, m.Handle+": no rights/affiliation evidence — UNKNOWN")
		default:
			counting++
		}
	}
	return countThreshold(DimMaintainer, counting, reqMaintainers, notes)
}

// evalOnboarding fails on any broken surface (a provable negative) and is UNKNOWN
// when a required surface is missing, skipped, or unproven.
func evalOnboarding(surfaces []OnboardingSurface) Dimension {
	byID := map[string]OnboardingSurface{}
	for _, s := range surfaces {
		byID[s.ID] = s
	}
	notes, healthy, broken, unknown := []string{}, 0, false, false
	for _, id := range requiredSurfaces {
		s, ok := byID[id]
		switch {
		case !ok:
			notes, unknown = append(notes, id+": no gate evidence — UNKNOWN"), true
		case s.Status == "broken":
			notes, broken = append(notes, id+": onboarding gate BROKEN — FAIL"), true
		case s.Skipped || s.Status != "pass":
			notes, unknown = append(notes, id+": gate skipped/unproven — UNKNOWN"), true
		default:
			healthy++
		}
	}
	st := StatusPass
	if broken {
		st = StatusFail
	} else if unknown {
		st = StatusUnknown
	}
	return Dimension{Name: DimOnboarding, Status: st, Required: len(requiredSurfaces), Counting: healthy, Notes: notes}
}
