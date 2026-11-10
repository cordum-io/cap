// Package publicreport projects a full TCK JSON report into a deterministic,
// privacy-safe public report (spec/tck/schema/public-report-v1.schema.json)
// that a third party can publish and anyone can check offline.
//
// Non-test code never imports internal/tck: that package pulls in nats-server.
// It decodes into its own mirror of tck.Report instead, kept in json-tag parity
// by a test.
package publicreport

// SchemaVersion is the only schemaVersion this package emits and accepts.
const SchemaVersion = "cap-public-report/v1"

// Verdicts of a public report.
const (
	VerdictPass       = "PASS"
	VerdictFail       = "FAIL"
	VerdictIncomplete = "INCOMPLETE"
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
// bytes.
func Project(source []byte, s Subject) (Report, []byte, error) {
	return Report{}, nil, ErrNotImplemented
}

func verdict(cases []Case) string { return "" }
