package governance

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/cordum-io/cap/v2/internal/tck/publicreport"
)

//go:embed testdata/attestations/pass-report.json
var passFixture []byte

func variant(mut func(*publicreport.Report)) []byte {
	r, err := publicreport.Validate(bytes.ReplaceAll(passFixture, []byte("\r\n"), []byte("\n")))
	if err != nil {
		panic(err)
	}
	mut(&r)
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(r); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func passReports() [][]byte {
	return [][]byte{variant(func(*publicreport.Report) {}), variant(func(r *publicreport.Report) {
		r.Implementation.ArtifactName = "beta-cap-worker"
		r.TestedCommit = strings.Repeat("b", 40)
	})}
}

func digest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func writeEvidence(t *testing.T, dir string, b []byte) string {
	t.Helper()
	d := digest(b)
	if err := os.WriteFile(filepath.Join(dir, d+".json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	return d
}

func passEvidence(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, b := range passReports() {
		writeEvidence(t, dir, b)
	}
	return dir
}

func assertImplementations(t *testing.T, r Readiness, count int, notes ...string) {
	t.Helper()
	d := dim(r, DimImplementations)
	want := StatusUnknown
	if count >= 2 {
		want = StatusPass
	}
	if d.Status != want || d.Counting != count || d.Required != 2 || strings.Join(d.Notes, "\n") != strings.Join(notes, "\n") {
		t.Fatalf("implementations = %+v; want %s %d/2, notes %q", d, want, count, notes)
	}
}

// fullPass builds a manifest whose evidence satisfies every readiness dimension.
// Individual tests mutate one aspect to prove each exclusion rule flips the verdict.
func fullPass() *Manifest {
	return &Manifest{
		SchemaVersion: SchemaVersion,
		Implementations: []Implementation{
			{ID: "impl-a", Name: "Acme CAP", URL: "https://ex.com/a", Affiliation: "Acme",
				Attestation: &TCKAttestation{Profile: "core", Version: "2.14.0", TCKVersion: "cap-tck/1", ReportDigest: digest(passReports()[0]), URL: "https://ex.com/a/report"}},
			{ID: "impl-b", Name: "Beta CAP", URL: "https://ex.com/b", Affiliation: "Beta Inc",
				Attestation: &TCKAttestation{Profile: "core", Version: "2.14.0", TCKVersion: "cap-tck/1", ReportDigest: digest(passReports()[1]), URL: "https://ex.com/b/report"}},
		},
		Adopters: []Adopter{
			{Name: "Adopter One", StatementURL: "https://ex.com/1", Consent: true, Profile: "worker-core", Version: "2.14.0", NonDemo: true},
			{Name: "Adopter Two", StatementURL: "https://ex.com/2", Consent: true, Profile: "worker-core", Version: "2.14.0", NonDemo: true},
		},
		Maintainers: []Maintainer{
			{Handle: "@indie", Affiliation: "Independent", IndependentOfCordum: true, RightsEvidenceURL: "https://ex.com/rights"},
		},
		OnboardingSurfaces: []OnboardingSurface{
			{ID: "root", Status: "pass", Commit: "ed0d8bd", Evidence: "gate log"},
			{ID: "go", Status: "pass", Commit: "ed0d8bd", Evidence: "gate log"},
			{ID: "python", Status: "pass", Commit: "ed0d8bd", Evidence: "gate log"},
			{ID: "node", Status: "pass", Commit: "ed0d8bd", Evidence: "gate log"},
		},
	}
}

func TestFullEvidenceIsReady(t *testing.T) {
	r := EvaluateWithEvidence(fullPass(), passEvidence(t))
	assertImplementations(t, r, 2)
	if r.Verdict != VerdictReady {
		t.Fatalf("want READY, got %s (%+v)", r.Verdict, r.Dimensions)
	}
}

func TestEmptyManifestIsBlocked(t *testing.T) {
	r := Evaluate(&Manifest{SchemaVersion: SchemaVersion})
	if r.Verdict != VerdictBlocked {
		t.Fatalf("empty manifest must be BLOCKED, got %s", r.Verdict)
	}
	for _, d := range r.Dimensions {
		if d.Status == StatusPass {
			t.Errorf("dimension %q must not PASS on empty evidence", d.Name)
		}
	}
}

func TestCordumImplementationsDoNotCount(t *testing.T) {
	m := fullPass()
	m.Implementations[0].Affiliation = "Cordum"
	m.Implementations[1].ControlledByCordum = true
	r := EvaluateWithEvidence(m, passEvidence(t))
	assertImplementations(t, r, 0, "impl-a: Cordum-controlled/affiliated — excluded", "impl-b: Cordum-controlled/affiliated — excluded")
	if d := dim(r, DimImplementations); d.Status == StatusPass {
		t.Fatalf("Cordum-controlled/affiliated impls must not satisfy the dimension: %+v", d)
	}
	if r.Verdict != VerdictBlocked {
		t.Fatalf("want BLOCKED, got %s", r.Verdict)
	}
}

func TestForkAndMissingAttestationDoNotCount(t *testing.T) {
	m := fullPass()
	m.Implementations[0].IsFork = true     // forks never count
	m.Implementations[1].Attestation = nil // missing attestation -> UNKNOWN
	r := EvaluateWithEvidence(m, passEvidence(t))
	assertImplementations(t, r, 0, "impl-a: fork — excluded", "impl-b: missing/stale TCK attestation — UNKNOWN")
	if dim(r, DimImplementations).Status == StatusPass {
		t.Fatalf("fork + missing attestation must not PASS")
	}
}

func TestAdopterNeedsConsentStatementAndNonDemo(t *testing.T) {
	for _, mut := range []func(*Adopter){
		func(a *Adopter) { a.Consent = false },
		func(a *Adopter) { a.StatementURL = "" },
		func(a *Adopter) { a.NonDemo = false },
		func(a *Adopter) { a.ControlledByCordum = true },
	} {
		m := fullPass()
		mut(&m.Adopters[0])
		if dim(Evaluate(m), DimAdopters).Status == StatusPass {
			t.Fatalf("adopter dimension passed despite an invalid adopter (only 1 valid remains)")
		}
	}
}

func TestMaintainerMustBeIndependent(t *testing.T) {
	m := fullPass()
	m.Maintainers[0].IndependentOfCordum = false
	if dim(Evaluate(m), DimMaintainer).Status == StatusPass {
		t.Fatal("non-independent maintainer must not satisfy the dimension")
	}
	m2 := fullPass()
	m2.Maintainers[0].RightsEvidenceURL = ""
	if dim(Evaluate(m2), DimMaintainer).Status != StatusUnknown {
		t.Fatal("maintainer without rights evidence must be UNKNOWN")
	}
}

func TestBrokenOnboardingIsFail(t *testing.T) {
	m := fullPass()
	m.OnboardingSurfaces[2].Status = "broken"
	d := dim(Evaluate(m), DimOnboarding)
	if d.Status != StatusFail {
		t.Fatalf("a broken onboarding surface must FAIL the dimension, got %s", d.Status)
	}
}

func TestSkippedOnboardingIsUnknownNotPass(t *testing.T) {
	m := fullPass()
	m.OnboardingSurfaces[1].Skipped = true
	d := dim(Evaluate(m), DimOnboarding)
	if d.Status == StatusPass {
		t.Fatal("a skipped gate must not count as a passing onboarding surface")
	}
}

func TestMissingOnboardingSurfaceIsUnknown(t *testing.T) {
	m := fullPass()
	m.OnboardingSurfaces = m.OnboardingSurfaces[:3] // drop node
	if dim(Evaluate(m), DimOnboarding).Status != StatusUnknown {
		t.Fatal("a missing required onboarding surface must be UNKNOWN")
	}
}

func dim(r Readiness, name string) Dimension {
	for _, d := range r.Dimensions {
		if d.Name == name {
			return d
		}
	}
	return Dimension{Name: name, Status: "MISSING"}
}

func TestPassFixtureValidates(t *testing.T) {
	r, err := publicreport.Validate(bytes.ReplaceAll(passFixture, []byte("\r\n"), []byte("\n")))
	if err != nil || r.Verdict != publicreport.VerdictPass || r.Implementation.ArtifactName != "acme-cap-worker" || capOwnedArtifacts[r.Implementation.ArtifactName] {
		t.Fatalf("fixture: %+v, %v", r, err)
	}
}

func TestEvaluateWithoutEvidenceCountsNoImplementation(t *testing.T) {
	assertImplementations(t, Evaluate(fullPass()), 0,
		"impl-a: no evidence directory — UNKNOWN", "impl-b: no evidence directory — UNKNOWN")
}

func TestUnavailableEvidenceDirectory(t *testing.T) {
	for _, dir := range []string{filepath.Join(t.TempDir(), "absent"), filepath.Join(passEvidence(t), digest(passReports()[0])+".json")} {
		assertImplementations(t, EvaluateWithEvidence(fullPass(), dir), 0,
			"impl-a: evidence directory unavailable — UNKNOWN", "impl-b: evidence directory unavailable — UNKNOWN")
	}
}

func TestEvidenceRuleViolationsDoNotCount(t *testing.T) {
	cases := []struct{ name, reason string }{
		{"missing-file", "evidence file missing — UNKNOWN"},
		{"symlink", "evidence is not a regular file — UNKNOWN"},
		{"not-a-file", "evidence is not a regular file — UNKNOWN"},
		{"digest-mismatch", "evidence SHA-256 does not match reportDigest — UNKNOWN"},
		{"schema-invalid", "evidence is not a valid public report v1 — UNKNOWN"},
		{"verdict-fail", "public report verdict FAIL, not PASS — UNKNOWN"},
		{"verdict-incomplete", "public report verdict INCOMPLETE, not PASS — UNKNOWN"},
		{"profile-mismatch", "public report profile does not match the attestation — UNKNOWN"},
		{"tck-version-mismatch", "public report TCK version does not match the attestation — UNKNOWN"},
		{"oversize", "evidence file over 1 MiB — UNKNOWN"},
		{"bad-digest", "reportDigest is not 64 lowercase hex — UNKNOWN"},
		{"cap-owned", "CAP-owned artifact cap-adapter-langgraph — excluded"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, dir := fullPass(), passEvidence(t)
			breakEvidence(t, dir, m.Implementations[1].Attestation, c.name)
			assertImplementations(t, EvaluateWithEvidence(m, dir), 1, "impl-b: "+c.reason)
		})
	}
}

func breakEvidence(t *testing.T, dir string, a *TCKAttestation, defect string) {
	t.Helper()
	path := filepath.Join(dir, a.ReportDigest+".json")
	switch defect {
	case "missing-file", "symlink", "not-a-file":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if defect == "not-a-file" {
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		}
		if defect == "symlink" {
			target := filepath.Join(dir, "target.json")
			if err := os.WriteFile(target, passReports()[1], 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("symlink unavailable: %v", err)
				}
				t.Fatal(err)
			}
		}
	case "digest-mismatch":
		if err := os.WriteFile(path, passReports()[0], 0600); err != nil {
			t.Fatal(err)
		}
	case "schema-invalid":
		a.ReportDigest = writeEvidence(t, dir, bytes.Replace(passReports()[1], []byte("{\"schemaVersion\""), []byte("{\"unknown\":true,\"schemaVersion\""), 1))
	case "oversize":
		a.ReportDigest = writeEvidence(t, dir, bytes.Repeat([]byte("x"), (1<<20)+1))
	case "bad-digest":
		a.ReportDigest = "sha256:" + a.ReportDigest
	default:
		a.ReportDigest = writeEvidence(t, dir, validDefectReport(t, defect))
	}
}

func validDefectReport(t *testing.T, defect string) []byte {
	t.Helper()
	want := publicreport.VerdictPass
	b := variant(func(r *publicreport.Report) {
		r.Implementation.ArtifactName = "beta-cap-worker"
		switch defect {
		case "verdict-fail":
			r.Cases[1].Status, r.Verdict = "FAIL", publicreport.VerdictFail
			want = publicreport.VerdictFail
		case "verdict-incomplete":
			r.Cases[1].Status, r.Verdict = "UNSUPPORTED", publicreport.VerdictIncomplete
			want = publicreport.VerdictIncomplete
		case "profile-mismatch":
			r.Profile = "cap-production"
		case "tck-version-mismatch":
			r.TCKToolVersion = "cap-tck/2"
		case "cap-owned":
			r.Implementation.ArtifactName = "cap-adapter-langgraph"
		default:
			t.Fatalf("unknown defect: %s", defect)
		}
	})
	r, err := publicreport.Validate(b)
	if err != nil || r.Verdict != want {
		t.Fatalf("variant: %+v, %v", r, err)
	}
	if defect == "profile-mismatch" && r.Profile != "cap-production" {
		t.Fatal(r.Profile)
	}
	if defect == "tck-version-mismatch" && r.TCKToolVersion != "cap-tck/2" {
		t.Fatal(r.TCKToolVersion)
	}
	if defect == "cap-owned" && r.Implementation.ArtifactName != "cap-adapter-langgraph" {
		t.Fatal(r.Implementation)
	}
	return b
}

func TestDuplicateEvidenceCountsOnce(t *testing.T) {
	for _, sameReport := range []bool{true, false} {
		t.Run(fmt.Sprintf("same-report-%t", sameReport), func(t *testing.T) {
			m, dir := fullPass(), passEvidence(t)
			b := passReports()[0]
			if !sameReport {
				b = variant(func(r *publicreport.Report) { r.TestedCommit = strings.Repeat("c", 40) })
			}
			m.Implementations[1].Attestation.ReportDigest = writeEvidence(t, dir, b)
			assertImplementations(t, EvaluateWithEvidence(m, dir), 1, "impl-b: duplicate of impl-a (same report or artifact) — excluded")
		})
	}
}

func TestCAPOwnedArtifactsList(t *testing.T) {
	want := map[string]bool{"cap-reference-scheduler": true, "cap-reference-submit": true,
		"cap-adapter-langgraph": true, "cap-adapter-crewai": true, "cap-tck-reference-adapter": true,
		"cap-framework-tck": true, "cap-sdk-go": true, "cap-sdk-python": true, "cap-sdk-node": true}
	if !reflect.DeepEqual(capOwnedArtifacts, want) {
		t.Fatalf("CAP-owned list = %v", capOwnedArtifacts)
	}
	for _, name := range []string{"acme-cap-worker", "cap-sdk-go-fork"} {
		m, dir := fullPass(), passEvidence(t)
		b := variant(func(r *publicreport.Report) { r.Implementation.ArtifactName = name })
		m.Implementations[0].Attestation.ReportDigest = writeEvidence(t, dir, b)
		assertImplementations(t, EvaluateWithEvidence(m, dir), 2)
	}
}

func TestBadDigestForms(t *testing.T) {
	for _, bad := range []string{strings.Repeat("A", 64), "../report", "/report", "C:\\report", strings.Repeat("a", 63)} {
		m, dir := fullPass(), passEvidence(t)
		m.Implementations[1].Attestation.ReportDigest = bad
		assertImplementations(t, EvaluateWithEvidence(m, dir), 1, "impl-b: reportDigest is not 64 lowercase hex — UNKNOWN")
	}
}
