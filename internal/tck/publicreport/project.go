// Package publicreport projects a full TCK JSON report into a deterministic,
// privacy-safe public report (spec/tck/schema/public-report-v1.schema.json)
// that a third party can publish and anyone can check offline.
//
// Non-test code never imports internal/tck: that package pulls in nats-server.
// It decodes into its own mirror of tck.Report instead, kept in json-tag parity
// by a test.
package publicreport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// SchemaVersion is the only schemaVersion this package emits and accepts.
const SchemaVersion = "cap-public-report/v1"

// Verdicts of a public report.
const (
	VerdictPass       = "PASS"
	VerdictFail       = "FAIL"
	VerdictIncomplete = "INCOMPLETE"
)

// Case statuses, as emitted by the TCK (internal/tck/proto.go).
const (
	statusPass        = "PASS"
	statusFail        = "FAIL"
	statusError       = "ERROR"
	statusUnsupported = "UNSUPPORTED"
	statusNA          = "N/A"
)

// Subject names what was tested. The caller supplies it; nothing in it comes
// from the source report.
type Subject struct {
	TestedCommit           string   // 40 lowercase hex
	ArtifactName           string   // ^[a-z0-9][a-z0-9._-]{0,127}$
	ArtifactSha256         string   // 64 lowercase hex
	ScenarioManifestSha256 string   // 64 lowercase hex
	RequiredCaseIDs        []string // role/profile-applicable required ids from that manifest
}

// Report is the public report. Field order is the canonical byte order.
type Report struct {
	SchemaVersion          string         `json:"schemaVersion"`
	TCKToolVersion         string         `json:"tckToolVersion"`
	Profile                string         `json:"profile"`
	Role                   string         `json:"role"`
	Transport              string         `json:"transport"`
	TestedCommit           string         `json:"testedCommit"`
	Implementation         Implementation `json:"implementation"`
	ScenarioManifestSha256 string         `json:"scenarioManifestSha256"`
	Cases                  []Case         `json:"cases"`
	Verdict                string         `json:"verdict"`
}

// Implementation identifies the tested artifact by name and digest.
type Implementation struct {
	ArtifactName   string `json:"artifactName"`
	ArtifactSha256 string `json:"artifactSha256"`
}

// Case is one projected case outcome.
type Case struct {
	ID       string `json:"id"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
}

// sourceReport mirrors tck.Report (internal/tck/report.go).
type sourceReport struct {
	Adapter     string        `json:"adapter"`
	SDK         string        `json:"sdk"`
	Role        string        `json:"role"`
	Profile     string        `json:"profile"`
	Transport   string        `json:"transport"`
	ToolVersion string        `json:"toolVersion"`
	Summary     sourceSummary `json:"summary"`
	Cases       []sourceCase  `json:"cases"`
}

// sourceSummary mirrors tck.Summary.
type sourceSummary struct {
	Total       int  `json:"total"`
	Pass        int  `json:"pass"`
	Fail        int  `json:"fail"`
	Error       int  `json:"error"`
	Unsupported int  `json:"unsupported"`
	NA          int  `json:"na"`
	Conformant  bool `json:"conformant"`
}

// sourceCase mirrors tck.CaseResult.
type sourceCase struct {
	ID         string            `json:"id"`
	Profile    string            `json:"profile"`
	Role       string            `json:"role"`
	Required   bool              `json:"required"`
	Applicable bool              `json:"applicable"`
	Status     string            `json:"status"`
	Detail     string            `json:"detail,omitempty"`
	DurationMs int64             `json:"durationMs"`
	Evidence   map[string]string `json:"evidence,omitempty"`
}

// Project turns a full TCK JSON report into a public report and its canonical
// bytes. Only the caller's Subject and pattern-validated tokens from the source
// are copied; no free text survives. On any error it returns the zero Report
// and nil bytes, never a partial report.
func Project(source []byte, s Subject) (Report, []byte, error) {
	r, err := project(source, s)
	if err != nil {
		return Report{}, nil, err
	}
	b, err := canonical(r)
	if err != nil {
		return Report{}, nil, err
	}
	return r, b, nil
}

func project(source []byte, s Subject) (Report, error) {
	err := checkIdentity(s.TestedCommit, s.ArtifactName, s.ArtifactSha256, s.ScenarioManifestSha256)
	if err == nil {
		err = checkRequiredIDs(s.RequiredCaseIDs)
	}
	if err != nil {
		return Report{}, err
	}
	src, err := decodeStrict[sourceReport](source, "source")
	if err != nil {
		return Report{}, err
	}
	if err := checkHeader(src.ToolVersion, src.Profile, src.Role, src.Transport); err != nil {
		return Report{}, err
	}
	cases, err := projectCases(src, s.RequiredCaseIDs)
	if err != nil {
		return Report{}, err
	}
	return Report{
		SchemaVersion:          SchemaVersion,
		TCKToolVersion:         src.ToolVersion,
		Profile:                src.Profile,
		Role:                   src.Role,
		Transport:              src.Transport,
		TestedCommit:           s.TestedCommit,
		Implementation:         Implementation{ArtifactName: s.ArtifactName, ArtifactSha256: s.ArtifactSha256},
		ScenarioManifestSha256: s.ScenarioManifestSha256,
		Cases:                  cases,
		Verdict:                verdict(cases),
	}, nil
}

// projectCases checks every source case and the summary, then returns the
// public cases sorted by id. A case is required if the caller lists it OR the
// source marks it required && applicable, so a caller can add requirements but
// never remove them.
func projectCases(src sourceReport, requiredIDs []string) ([]Case, error) {
	index := make(map[string]int, len(src.Cases))
	cases := make([]Case, 0, len(src.Cases))
	tally := sourceSummary{Total: len(src.Cases), Conformant: true}
	for i, c := range src.Cases {
		if err := checkCase(c.ID, c.Status, "source", i); err != nil {
			return nil, err
		}
		if _, dup := index[c.ID]; dup {
			return nil, fmt.Errorf("%w: source case %s", ErrDuplicateCase, c.ID)
		}
		index[c.ID] = len(cases)
		required := c.Required && c.Applicable
		if required && c.Status != statusPass {
			tally.Conformant = false
		}
		countStatus(&tally, c.Status)
		cases = append(cases, Case{ID: c.ID, Required: required, Status: c.Status})
	}
	if tally != src.Summary {
		return nil, fmt.Errorf("%w: summary does not match the cases", ErrInconsistentSource)
	}
	for _, id := range requiredIDs {
		i, ok := index[id]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingRequiredCase, id)
		}
		cases[i].Required = true
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	return cases, nil
}

func countStatus(s *sourceSummary, status string) {
	switch status {
	case statusPass:
		s.Pass++
	case statusFail:
		s.Fail++
	case statusError:
		s.Error++
	case statusUnsupported:
		s.Unsupported++
	case statusNA:
		s.NA++
	}
}

// verdict is a pure function of the public cases: FAIL if any case is FAIL or
// ERROR, otherwise INCOMPLETE if any required case is not PASS, otherwise PASS.
func verdict(cases []Case) string {
	v := VerdictPass
	for _, c := range cases {
		switch {
		case c.Status == statusFail || c.Status == statusError:
			return VerdictFail
		case c.Required && c.Status != statusPass:
			v = VerdictIncomplete
		}
	}
	return v
}

// canonical is the one byte form of a Report: encoding/json of the fixed
// struct, no HTML escaping, and the single trailing newline Encode adds.
func canonical(r Report) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return nil, fmt.Errorf("publicreport: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// decodeStrict decodes exactly one JSON value into a T and rejects unknown
// fields. A loose decode runs first so malformed JSON and unknown fields map to
// distinct errors.
func decodeStrict[T any](b []byte, what string) (T, error) {
	var zero, loose, strict T
	if err := decodeOne(b, &loose, false); err != nil {
		return zero, fmt.Errorf("%w: %s: %v", ErrMalformedJSON, what, err)
	}
	if err := decodeOne(b, &strict, true); err != nil {
		return zero, fmt.Errorf("%w: %s: %v", ErrUnknownField, what, err)
	}
	return strict, nil
}

func decodeOne(b []byte, v any, strict bool) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after the JSON value")
	}
	return nil
}
