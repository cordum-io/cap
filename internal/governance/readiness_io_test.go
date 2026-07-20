package governance

import (
	"strings"
	"testing"
)

const minimalTruthfulManifest = `{
  "schemaVersion": "readiness-v1",
  "implementations": [],
  "adopters": [],
  "maintainers": [],
  "onboardingSurfaces": []
}`

func TestLoadManifestValid(t *testing.T) {
	m, err := LoadManifest([]byte(minimalTruthfulManifest))
	if err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if Evaluate(m).Verdict != VerdictBlocked {
		t.Fatal("empty-but-truthful manifest must evaluate BLOCKED")
	}
}

func TestLoadManifestRejectsWrongSchema(t *testing.T) {
	_, err := LoadManifest([]byte(`{"schemaVersion":"readiness-v2","implementations":[],"adopters":[],"maintainers":[],"onboardingSurfaces":[]}`))
	if err == nil {
		t.Fatal("wrong schemaVersion must be rejected")
	}
}

func TestLoadManifestRejectsUnknownField(t *testing.T) {
	_, err := LoadManifest([]byte(`{"schemaVersion":"readiness-v1","verdict":"READY","implementations":[],"adopters":[],"maintainers":[],"onboardingSurfaces":[]}`))
	if err == nil {
		t.Fatal("a hand-set verdict (unknown field) must be rejected — verdict is computed, never trusted")
	}
}

func TestValidateRejectsUnknownOnboardingStatus(t *testing.T) {
	m := &Manifest{SchemaVersion: SchemaVersion, OnboardingSurfaces: []OnboardingSurface{{ID: "go", Status: "green"}}}
	if err := Validate(m); err == nil {
		t.Fatal("unknown onboarding status must be rejected")
	}
}

func TestValidateRejectsDuplicateImplID(t *testing.T) {
	m := &Manifest{SchemaVersion: SchemaVersion, Implementations: []Implementation{{ID: "x"}, {ID: "x"}}}
	if err := Validate(m); err == nil {
		t.Fatal("duplicate implementation id must be rejected")
	}
}

func TestRenderShowsComputedVerdictAndExclusions(t *testing.T) {
	m := fullPass()
	m.Implementations[0].Affiliation = "Cordum"
	out := Render(m, Evaluate(m))
	if !strings.Contains(out, VerdictBlocked) {
		t.Fatal("render must show the computed BLOCKED verdict")
	}
	if !strings.Contains(out, "Cordum-controlled/affiliated") {
		t.Fatal("render must surface why evidence was excluded")
	}
	if strings.Contains(out, VerdictReady) {
		t.Fatal("render must not print READY for a blocked manifest")
	}
}
