package tck

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
)

// CaseResult is the normalized outcome of one scenario case.
type CaseResult struct {
	ID         string            `json:"id"`
	Profile    string            `json:"profile"`
	Role       Role              `json:"role"`
	Required   bool              `json:"required"`
	Applicable bool              `json:"applicable"`
	Status     Status            `json:"status"`
	Detail     string            `json:"detail,omitempty"`
	DurationMs int64             `json:"durationMs"`
	Evidence   map[string]string `json:"evidence,omitempty"`
}

// Report is the full result of running one adapter against one profile. Field
// order is fixed and maps are sorted at the margins so serialization is
// deterministic — the same results always produce byte-identical output.
type Report struct {
	Adapter     string       `json:"adapter"`
	SDK         string       `json:"sdk"`
	Role        Role         `json:"role"`
	Profile     string       `json:"profile"`
	Transport   string       `json:"transport"`
	ToolVersion string       `json:"toolVersion"`
	Summary     Summary      `json:"summary"`
	Cases       []CaseResult `json:"cases"`
}

// Summary counts outcomes and records the overall conformance verdict.
type Summary struct {
	Total       int  `json:"total"`
	Pass        int  `json:"pass"`
	Fail        int  `json:"fail"`
	Error       int  `json:"error"`
	Unsupported int  `json:"unsupported"`
	NA          int  `json:"na"`
	Conformant  bool `json:"conformant"`
}

// NewReport assembles a report from cases in deterministic id order and
// computes the summary. A run is conformant only if every applicable required
// case reported PASS.
func NewReport(adapter string, hs AdapterMessage, profile, toolVersion string, cases []CaseResult) Report {
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	s := Summary{Total: len(cases), Conformant: true}
	for _, c := range cases {
		countCase(&s, c)
		if c.Applicable && c.Required && IsRequiredNonPass(c.Status) {
			s.Conformant = false
		}
	}
	return Report{
		Adapter: adapter, SDK: hs.SDK, Role: hs.Role, Profile: profile,
		Transport: hs.Transport, ToolVersion: toolVersion, Summary: s, Cases: cases,
	}
}

func countCase(s *Summary, c CaseResult) {
	switch c.Status {
	case StatusPass:
		s.Pass++
	case StatusFail:
		s.Fail++
	case StatusError:
		s.Error++
	case StatusUnsupported:
		s.Unsupported++
	case StatusNA:
		s.NA++
	}
}

// JSON renders the report as indented, deterministic JSON.
func (r Report) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("tck: marshal report: %w", err)
	}
	return append(b, '\n'), nil
}

// JUnit renders the report as a JUnit XML suite. Non-applicable cases become
// <skipped>; FAIL and required non-pass become <failure>; ERROR becomes
// <error>. xml.MarshalIndent escapes all text, so adapter detail cannot break
// the document.
func (r Report) JUnit() ([]byte, error) {
	suite := junitSuite{
		Name:  "cap-tck." + r.Profile,
		Tests: r.Summary.Total,
	}
	for _, c := range r.Cases {
		tc := junitCase{Name: c.ID, Classname: r.Profile, TimeSec: float64(c.DurationMs) / 1000}
		applyJUnitStatus(&suite, &tc, c)
		suite.Cases = append(suite.Cases, tc)
	}
	body, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("tck: marshal junit: %w", err)
	}
	return append([]byte(xml.Header), append(body, '\n')...), nil
}

func applyJUnitStatus(s *junitSuite, tc *junitCase, c CaseResult) {
	switch {
	case !c.Applicable:
		// role mismatch: genuinely not applicable to this adapter.
		tc.Skipped = &junitMsg{Message: string(c.Status)}
		s.Skipped++
	case c.Status == StatusPass:
		// passing case carries no child element.
	case c.Status == StatusError:
		tc.Error = &junitMsg{Message: c.Detail, Type: string(StatusError)}
		s.Errors++
	case c.Required && IsRequiredNonPass(c.Status):
		// Required applicable non-PASS (incl. adapter-reported N/A/UNSUPPORTED)
		// is a failure, never a skip — no laundering the declined-case loophole.
		tc.Failure = &junitMsg{Message: c.Detail, Type: string(c.Status)}
		s.Failures++
	case c.Status == StatusNA:
		tc.Skipped = &junitMsg{Message: string(c.Status)} // optional, declined
		s.Skipped++
	default:
		tc.Failure = &junitMsg{Message: c.Detail, Type: string(c.Status)} // non-required FAIL/UNSUPPORTED
		s.Failures++
	}
}

type junitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string    `xml:"name,attr"`
	Classname string    `xml:"classname,attr"`
	TimeSec   float64   `xml:"time,attr"`
	Skipped   *junitMsg `xml:"skipped,omitempty"`
	Failure   *junitMsg `xml:"failure,omitempty"`
	Error     *junitMsg `xml:"error,omitempty"`
}

type junitMsg struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr,omitempty"`
}
