package sdksupport

import (
	"testing"
	"testing/fstest"
)

// validRoot contains every evidence path a fully specified stable entry cites.
func validRoot() fstest.MapFS {
	return fstest.MapFS{
		"sdk/go/README.md":                    {Data: []byte("docs")},
		".github/workflows/publish-go.yml":    {Data: []byte("name: publish")},
		".github/workflows/sdk-gates.yml":     {Data: []byte("name: gates")},
		"sdk/go/conformance_test.go":          {Data: []byte("package sdkgo")},
		"internal/tck/nats_transport_test.go": {Data: []byte("package tck")},
	}
}

func stableGates() []Gate {
	return []Gate{
		{ID: "publish", Kind: GateWorkflow, Path: ".github/workflows/publish-go.yml"},
		{ID: "install", Kind: GateWorkflow, Path: ".github/workflows/sdk-gates.yml", Job: "go-install"},
		{ID: "compat", Kind: GateTest, Path: "sdk/go/conformance_test.go"},
		{ID: "tck", Kind: GateTest, Path: "sdk/go/conformance_test.go"},
		{ID: "real-nats", Kind: GateTest, Path: "internal/tck/nats_transport_test.go"},
	}
}

func stableEntry() Entry {
	return Entry{
		ID:        "go",
		Kind:      KindProtocolSDK,
		Tier:      TierStable,
		Owner:     "@yaront1111",
		OwnerPath: "sdk/go/",
		Package:   &Package{Registry: "go", Coordinate: "github.com/cordum-io/cap/v2"},
		Docs:      "sdk/go/README.md",
		Gates:     stableGates(),
	}
}

func manifestOf(entries ...Entry) *Manifest {
	return &Manifest{Version: 1, Entries: entries}
}

// hasCode reports whether any problem carries the given code.
func hasCode(problems []Problem, code string) bool {
	for _, p := range problems {
		if p.Code == code {
			return true
		}
	}
	return false
}

func assertCode(t *testing.T, problems []Problem, want string) {
	t.Helper()
	if !hasCode(problems, want) {
		t.Fatalf("expected a %q problem, got %d problems: %+v", want, len(problems), problems)
	}
}

func TestVerifyAcceptsFullyEvidencedStableEntry(t *testing.T) {
	problems := Verify(manifestOf(stableEntry()), validRoot())
	if len(problems) != 0 {
		t.Fatalf("expected 0 problems for a fully evidenced entry, got %d: %+v", len(problems), problems)
	}
}

func TestVerifyRejectsUnknownTier(t *testing.T) {
	e := stableEntry()
	e.Tier = Tier("production-ready")
	assertCode(t, Verify(manifestOf(e), validRoot()), CodeUnknownTier)
}

func TestVerifyRejectsUnknownKind(t *testing.T) {
	e := stableEntry()
	e.Kind = Kind("sdk")
	assertCode(t, Verify(manifestOf(e), validRoot()), CodeUnknownKind)
}

func TestVerifyRejectsDuplicateEntryID(t *testing.T) {
	a, b := stableEntry(), stableEntry()
	assertCode(t, Verify(manifestOf(a, b), validRoot()), CodeDuplicateID)
}

func TestVerifyRejectsDuplicatePackageCoordinate(t *testing.T) {
	a, b := stableEntry(), stableEntry()
	b.ID = "go-alias" // distinct id, same coordinate
	assertCode(t, Verify(manifestOf(a, b), validRoot()), CodeDuplicatePackage)
}

func TestVerifyRejectsMissingOwner(t *testing.T) {
	e := stableEntry()
	e.Owner = ""
	assertCode(t, Verify(manifestOf(e), validRoot()), CodeMissingOwner)
}

func TestVerifyRejectsMissingDocs(t *testing.T) {
	e := stableEntry()
	e.Docs = ""
	assertCode(t, Verify(manifestOf(e), validRoot()), CodeMissingDocs)
}

// A docs path that does not resolve is the difference between a claim and
// evidence; the verifier must not accept a filename it cannot open.
func TestVerifyRejectsDocsPathThatDoesNotExist(t *testing.T) {
	e := stableEntry()
	e.Docs = "sdk/go/NOPE.md"
	assertCode(t, Verify(manifestOf(e), validRoot()), CodeDocsPathMissing)
}

func TestVerifyRejectsGatePathThatDoesNotExist(t *testing.T) {
	e := stableEntry()
	e.Gates[0].Path = ".github/workflows/does-not-exist.yml"
	assertCode(t, Verify(manifestOf(e), validRoot()), CodeGatePathMissing)
}

// Each required gate must be individually enforced, so drop them one at a time
// rather than asserting a single representative case.
func TestVerifyRejectsStableEntryMissingAnyRequiredGate(t *testing.T) {
	for _, missing := range RequiredStableGates {
		t.Run(missing, func(t *testing.T) {
			e := stableEntry()
			kept := make([]Gate, 0, len(e.Gates))
			for _, g := range e.Gates {
				if g.ID != missing {
					kept = append(kept, g)
				}
			}
			e.Gates = kept
			assertCode(t, Verify(manifestOf(e), validRoot()), CodeMissingRequiredGate)
		})
	}
}

func TestVerifyRejectsUnknownGateKind(t *testing.T) {
	e := stableEntry()
	e.Gates[0].Kind = GateKind("manual")
	assertCode(t, Verify(manifestOf(e), validRoot()), CodeUnknownGateKind)
}

func TestVerifyRejectsDuplicateGateIDWithinEntry(t *testing.T) {
	e := stableEntry()
	e.Gates = append(e.Gates, e.Gates[0])
	assertCode(t, Verify(manifestOf(e), validRoot()), CodeDuplicateGateID)
}

// An experimental entry makes no publishing/transport claim, so it must pass
// without the stable gate set. This pins the tier distinction itself.
func TestVerifyAllowsExperimentalEntryWithoutStableGates(t *testing.T) {
	e := Entry{
		ID:        "rust",
		Kind:      KindProtocolSDK,
		Tier:      TierExperimental,
		Owner:     "@yaront1111",
		OwnerPath: "sdk/rust/",
		Docs:      "sdk/go/README.md",
	}
	problems := Verify(manifestOf(e), validRoot())
	if hasCode(problems, CodeMissingRequiredGate) {
		t.Fatalf("experimental entry must not require stable gates, got: %+v", problems)
	}
}

// An extension is not a protocol SDK and must not be dragged into the wire
// matrix by tier alone.
func TestVerifyAllowsExtensionKindWithoutStableGates(t *testing.T) {
	e := Entry{
		ID:        "python-guard",
		Kind:      KindExtension,
		Tier:      TierCommunity,
		Owner:     "@yaront1111",
		OwnerPath: "sdk/python-guard/",
		Docs:      "sdk/go/README.md",
	}
	problems := Verify(manifestOf(e), validRoot())
	if hasCode(problems, CodeMissingRequiredGate) {
		t.Fatalf("extension must not require protocol-sdk stable gates, got: %+v", problems)
	}
}
