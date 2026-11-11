package publicreport

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
)

// Sentinel errors. Every input rejection wraps exactly one of them, so callers
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
	statuses = []string{statusPass, statusFail, statusError, statusUnsupported, statusNA}
	verdicts = []string{VerdictPass, VerdictFail, VerdictIncomplete}
)

// Validate strictly decodes public report bytes and checks every field, the
// case order, the verdict (recomputed from the cases) and that b is exactly
// the canonical encoding. On any error it returns the zero Report.
func Validate(b []byte) (Report, error) {
	r, err := decodeStrict[Report](b, "public report")
	if err != nil {
		return Report{}, err
	}
	if err := checkReport(r); err != nil {
		return Report{}, err
	}
	want, err := canonical(r)
	if err != nil {
		return Report{}, err
	}
	if !bytes.Equal(b, want) {
		return Report{}, fmt.Errorf("%w: re-encoding differs", ErrNotCanonical)
	}
	return r, nil
}

func checkReport(r Report) error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: want %s", ErrSchemaVersion, SchemaVersion)
	}
	if err := checkHeader(r.TCKToolVersion, r.Profile, r.Role, r.Transport); err != nil {
		return err
	}
	impl := r.Implementation
	if err := checkIdentity(r.TestedCommit, impl.ArtifactName, impl.ArtifactSha256, r.ScenarioManifestSha256); err != nil {
		return err
	}
	if err := checkSortedCases(r.Cases); err != nil {
		return err
	}
	if got := verdict(r.Cases); r.Verdict != got {
		return fmt.Errorf("%w: cases give %s", ErrVerdictMismatch, got)
	}
	return nil
}

// checkSortedCases requires valid, strictly ascending case ids and at least
// one required case.
func checkSortedCases(cases []Case) error {
	required := false
	for i, c := range cases {
		if err := checkCase(c.ID, c.Status, "public", i); err != nil {
			return err
		}
		if i > 0 && cases[i-1].ID == c.ID {
			return fmt.Errorf("%w: public case %s", ErrDuplicateCase, c.ID)
		}
		if i > 0 && cases[i-1].ID > c.ID {
			return fmt.Errorf("%w: public cases[%d]", ErrUnsortedCases, i)
		}
		required = required || c.Required
	}
	if !required {
		return fmt.Errorf("%w: public report", ErrNoRequiredCases)
	}
	return nil
}

// checkHeader validates the tokens copied from a source report.
func checkHeader(toolVersion, profile, role, transport string) error {
	switch {
	case !reToolVersion.MatchString(toolVersion):
		return fmt.Errorf("%w: tool version", ErrBadToken)
	case !reToken.MatchString(profile):
		return fmt.Errorf("%w: profile", ErrBadToken)
	case !slices.Contains(roles, role):
		return fmt.Errorf("%w: role", ErrBadToken)
	case !reToken.MatchString(transport):
		return fmt.Errorf("%w: transport", ErrBadToken)
	}
	return nil
}

// checkIdentity validates the caller-supplied subject fields.
func checkIdentity(commit, artifactName, artifactSha256, manifestSha256 string) error {
	switch {
	case !reCommit.MatchString(commit):
		return fmt.Errorf("%w: testedCommit", ErrBadCommit)
	case !reArtifactName.MatchString(artifactName):
		return fmt.Errorf("%w: artifactName", ErrBadArtifactName)
	case !reDigest.MatchString(artifactSha256):
		return fmt.Errorf("%w: artifactSha256", ErrBadDigest)
	case !reDigest.MatchString(manifestSha256):
		return fmt.Errorf("%w: scenarioManifestSha256", ErrBadDigest)
	}
	return nil
}

func checkRequiredIDs(ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("%w: subject RequiredCaseIDs is empty", ErrNoRequiredCases)
	}
	seen := make(map[string]bool, len(ids))
	for i, id := range ids {
		if !reCaseID.MatchString(id) {
			return fmt.Errorf("%w: subject RequiredCaseIDs[%d]", ErrBadCaseID, i)
		}
		if seen[id] {
			return fmt.Errorf("%w: subject RequiredCaseIDs %s", ErrDuplicateCase, id)
		}
		seen[id] = true
	}
	return nil
}

func checkCase(id, status, where string, i int) error {
	if !reCaseID.MatchString(id) {
		return fmt.Errorf("%w: %s cases[%d].id", ErrBadCaseID, where, i)
	}
	if !slices.Contains(statuses, status) {
		return fmt.Errorf("%w: %s cases[%d].status", ErrBadStatus, where, i)
	}
	return nil
}
