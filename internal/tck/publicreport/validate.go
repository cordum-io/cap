package publicreport

import (
	"errors"
	"regexp"
)

// Sentinel errors. Every returned error wraps exactly one of them, so callers
// match with errors.Is.
var (
	ErrSchemaVersion       = errors.New("publicreport: unknown schemaVersion")
	ErrUnknownField        = errors.New("publicreport: unknown field")
	ErrMalformedJSON       = errors.New("publicreport: malformed JSON")
	ErrDuplicateCase       = errors.New("publicreport: duplicate case id")
	ErrMissingRequiredCase = errors.New("publicreport: required case missing from source")
	ErrNoRequiredCases     = errors.New("publicreport: no required cases")
	ErrBadCommit           = errors.New("publicreport: testedCommit is not 40 lowercase hex")
	ErrBadDigest           = errors.New("publicreport: digest is not 64 lowercase hex")
	ErrBadArtifactName     = errors.New("publicreport: bad artifactName")
	ErrBadCaseID           = errors.New("publicreport: bad case id")
	ErrBadToken            = errors.New("publicreport: bad role, profile, transport or tool version")
	ErrBadStatus           = errors.New("publicreport: bad case status")
	ErrUnsortedCases       = errors.New("publicreport: cases not sorted by id")
	ErrNotCanonical        = errors.New("publicreport: bytes are not the canonical encoding")
	ErrVerdictMismatch     = errors.New("publicreport: verdict does not match cases")
	ErrInconsistentSource  = errors.New("publicreport: source summary inconsistent with its cases")
	ErrNotImplemented      = errors.New("publicreport: not implemented")
)

// Patterns, kept in parity with spec/tck/schema/public-report-v1.schema.json.
var (
	reCommit       = regexp.MustCompile(`^[0-9a-f]{40}$`)
	reDigest       = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reArtifactName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	reCaseID       = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*/[a-z0-9]+(-[a-z0-9]+)*$`)
	reToken        = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	reToolVersion  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+-]{0,63}(/[A-Za-z0-9][A-Za-z0-9.+-]{0,63})?$`)
)

// Enumerations, kept in parity with the schema.
var (
	roles    = []string{"worker", "control-plane"}
	statuses = []string{"PASS", "FAIL", "ERROR", "UNSUPPORTED", "N/A"}
	verdicts = []string{VerdictPass, VerdictFail, VerdictIncomplete}
)

// Validate strictly decodes and checks public report bytes.
func Validate(b []byte) (Report, error) {
	return Report{}, ErrNotImplemented
}
