package releasetruth

import (
	"strings"
	"testing"
)

func validCandidateManifest() *Manifest {
	m := validManifest()
	m.Candidate = &Candidate{
		Version: "2.15.0",
		Tag:     "v2.15.0",
		Channel: "stable",
	}
	return m
}

func TestValidate_CandidateRules(t *testing.T) {
	if ps := Validate(validCandidateManifest()); len(ps) != 0 {
		t.Fatalf("Validate(valid candidate) = %v, want none", ps)
	}

	cases := []struct {
		name   string
		mutate func(*Manifest)
		field  string
	}{
		{"non-semver version", func(m *Manifest) { m.Candidate.Version = "2.15" }, "candidate.version"},
		{"tag not v+version", func(m *Manifest) { m.Candidate.Tag = "v2.15.1" }, "candidate.tag"},
		{"same as published release", func(m *Manifest) { m.Candidate.Version = "2.14.0"; m.Candidate.Tag = "v2.14.0" }, "candidate.version"},
		{"older than published release", func(m *Manifest) { m.Candidate.Version = "2.13.0"; m.Candidate.Tag = "v2.13.0" }, "candidate.version"},
		{"unknown channel", func(m *Manifest) { m.Candidate.Channel = "prod" }, "candidate.channel"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validCandidateManifest()
			tc.mutate(m)
			if ps := Validate(m); !hasField(ps, tc.field) {
				t.Fatalf("Validate(candidate) = %v, want field %q", ps, tc.field)
			}
		})
	}
}

func TestValidate_CandidateVersionComparisonDoesNotOverflow(t *testing.T) {
	m := validCandidateManifest()
	m.Release.Version = "999999999999999999999999999998.0.0"
	m.Release.Tag = "v" + m.Release.Version
	m.Candidate.Version = "999999999999999999999999999999.0.0"
	m.Candidate.Tag = "v" + m.Candidate.Version
	if ps := Validate(m); hasField(ps, "candidate.version") {
		t.Fatalf("Validate(large newer candidate) = %v, want no candidate.version problem", ps)
	}
}

func TestLoad_Candidate(t *testing.T) {
	want := validCandidateManifest()
	b := mustMarshalManifest(t, want)
	got, err := Load(b)
	if err != nil {
		t.Fatalf("Load(candidate) error: %v", err)
	}
	if got.Candidate == nil || *got.Candidate != *want.Candidate {
		t.Fatalf("Load(candidate) = %#v, want %#v", got.Candidate, want.Candidate)
	}
}

func TestCheckSourceMetadata_AllowsExactCandidateVersion(t *testing.T) {
	m := validCandidateManifest()
	root := writeReleaseTree(t, m.Candidate.Version, m.Candidate.Tag)
	if ps := CheckSourceMetadata(m, root); len(ps) != 0 {
		t.Fatalf("CheckSourceMetadata(candidate) = %v, want none", ps)
	}
}

func TestCheckSourceMetadata_RejectsStableVersionWithoutCandidate(t *testing.T) {
	m := validManifest()
	root := writeReleaseTree(t, "2.15.0", "v2.15.0")
	if ps := CheckSourceMetadata(m, root); !hasField(ps, "development.source.version") {
		t.Fatalf("CheckSourceMetadata(stable development) = %v, want development.source.version", ps)
	}
}

func TestReleaseCheck_CandidateCannotBeTagged(t *testing.T) {
	m := validCandidateManifest()
	root := writeReleaseTree(t, m.Candidate.Version, m.Candidate.Tag)
	if ps := ReleaseCheck(m, root, m.Candidate.Tag); !hasField(ps, "snapshot.required") {
		t.Fatalf("ReleaseCheck(candidate) = %v, want snapshot.required", ps)
	}
	if m.Release.Version != "2.14.0" {
		t.Fatalf("published release mutated to %q", m.Release.Version)
	}
}

func TestRenderBlock_LabelsCandidateAsNotPublished(t *testing.T) {
	m := validCandidateManifest()
	for _, id := range []string{"release-status", "version-policy"} {
		got, err := RenderBlock(m, id)
		if err != nil {
			t.Fatalf("RenderBlock(%s) error: %v", id, err)
		}
		if !strings.Contains(got, "2.14.0") || !strings.Contains(got, "2.15.0") {
			t.Fatalf("RenderBlock(%s) = %q, want published and candidate versions", id, got)
		}
		if !strings.Contains(strings.ToLower(got), "candidate") || !strings.Contains(strings.ToLower(got), "not published") {
			t.Fatalf("RenderBlock(%s) = %q, want explicit candidate/not-published label", id, got)
		}
	}
}
