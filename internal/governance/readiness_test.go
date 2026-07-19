package governance

import "testing"

// fullPass builds a manifest whose evidence satisfies every readiness dimension.
// Individual tests mutate one aspect to prove each exclusion rule flips the verdict.
func fullPass() *Manifest {
	return &Manifest{
		SchemaVersion: SchemaVersion,
		Implementations: []Implementation{
			{ID: "impl-a", Name: "Acme CAP", URL: "https://ex.com/a", Affiliation: "Acme",
				Attestation: &TCKAttestation{Profile: "worker-core", Version: "2.14.0", TCKVersion: "1", ReportDigest: "sha256:aa", URL: "https://ex.com/a/report"}},
			{ID: "impl-b", Name: "Beta CAP", URL: "https://ex.com/b", Affiliation: "Beta Inc",
				Attestation: &TCKAttestation{Profile: "worker-core", Version: "2.14.0", TCKVersion: "1", ReportDigest: "sha256:bb", URL: "https://ex.com/b/report"}},
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
	r := Evaluate(fullPass())
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
	r := Evaluate(m)
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
	r := Evaluate(m)
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
