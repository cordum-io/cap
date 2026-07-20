package releasetruth

import (
	"strings"
	"testing"
)

func validSnapshotManifest() *Manifest {
	m := validManifest()
	m.Snapshot = &Snapshot{
		Version: "2.15.0",
		Tag:     "v2.15.0",
		Channel: "stable",
	}
	return m
}

func TestValidate_ReleaseStateSchemaCoupling(t *testing.T) {
	tests := []struct {
		name   string
		build  func() *Manifest
		schema string
		field  string
	}{
		{"candidate needs schema 1.1 or newer", validCandidateManifest, "1.0.0", "candidate.schemaVersion"},
		{"snapshot needs schema 1.2 or newer", validSnapshotManifest, "1.1.0", "snapshot.schemaVersion"},
		{"unknown schema rejected", validSnapshotManifest, "9.0.0", "schemaVersion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.build()
			m.SchemaVersion = tt.schema
			if ps := Validate(m); !hasField(ps, tt.field) {
				t.Fatalf("Validate(schema %s) = %v, want %s", tt.schema, ps, tt.field)
			}
		})
	}
}

func TestValidate_CandidateRemainsCompatibleWithSchemaOneOne(t *testing.T) {
	m := validCandidateManifest()
	m.SchemaVersion = "1.1.0"
	if ps := Validate(m); hasField(ps, "candidate.schemaVersion") || hasField(ps, "schemaVersion") {
		t.Fatalf("Validate(schema 1.1 candidate) = %v, want schema compatibility", ps)
	}
}

func TestValidate_ReleaseStatesAreMutuallyExclusive(t *testing.T) {
	m := validSnapshotManifest()
	m.Candidate = validCandidateManifest().Candidate
	if ps := Validate(m); !hasField(ps, "releaseState") {
		t.Fatalf("Validate(candidate+snapshot) = %v, want releaseState", ps)
	}
}

func TestValidate_RejectsNonCanonicalReleaseCoordinates(t *testing.T) {
	tests := []struct {
		name  string
		build func() *Manifest
		field string
	}{
		{"candidate leading zero", func() *Manifest {
			m := validCandidateManifest()
			m.Candidate.Version, m.Candidate.Tag = "02.15.0", "v02.15.0"
			return m
		}, "candidate.version"},
		{"snapshot leading zero", func() *Manifest {
			m := validSnapshotManifest()
			m.Snapshot.Version, m.Snapshot.Tag = "02.15.0", "v02.15.0"
			return m
		}, "snapshot.version"},
		{"candidate beta channel on stable tag", func() *Manifest {
			m := validCandidateManifest()
			m.Candidate.Channel = "beta"
			return m
		}, "candidate.channel"},
		{"snapshot rc channel on stable tag", func() *Manifest {
			m := validSnapshotManifest()
			m.Snapshot.Channel = "rc"
			return m
		}, "snapshot.channel"},
		{"release leading zero", func() *Manifest {
			m := validManifest()
			m.Release.Version, m.Release.Tag = "02.14.0", "v02.14.0"
			return m
		}, "release.version"},
		{"release beta channel on stable tag", func() *Manifest {
			m := validManifest()
			m.Release.Channel = "beta"
			return m
		}, "release.channel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ps := Validate(tt.build()); !hasField(ps, tt.field) {
				t.Fatalf("Validate() = %v, want %s", ps, tt.field)
			}
		})
	}
}

func TestValidate_PromotionRequiresPublishedComponentVersions(t *testing.T) {
	m := validManifest()
	m.Release.Version, m.Release.Tag = "2.15.0", "v2.15.0"
	if ps := Validate(m); !hasField(ps, "components.published.version") {
		t.Fatalf("Validate(unreconciled promotion) = %v, want components.published.version", ps)
	}
}

func TestValidate_PreReleaseStateCannotAdvertiseCandidateComponents(t *testing.T) {
	for _, build := range []func() *Manifest{validCandidateManifest, validSnapshotManifest} {
		m := build()
		for i := range m.Components {
			if m.Components[i].Tier == "stable" {
				m.Components[i].Version = "2.15.0"
				break
			}
		}
		if ps := Validate(m); !hasField(ps, "components.published.version") {
			t.Fatalf("Validate(pre-bumped component) = %v, want components.published.version", ps)
		}
	}
}

func TestCheckSourceMetadata_GatesBothPackageLockVersions(t *testing.T) {
	for _, tt := range []struct {
		name, top, root string
	}{
		{"top-level lock version", "2.15.0-dev.0", "2.15.0"},
		{"root package lock version", "2.15.0", "2.15.0-dev.0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := validCandidateManifest()
			root := writeReleaseTree(t, "2.15.0", "v2.15.0")
			writeNodePackageLock(t, root, tt.top, tt.root)
			if ps := CheckSourceMetadata(m, root); !hasField(ps, "candidate.source.version") {
				t.Fatalf("CheckSourceMetadata(stale lock) = %v, want candidate.source.version", ps)
			}
		})
	}
}

func TestCheckSourceMetadata_RejectsIncompletePackageLock(t *testing.T) {
	m := validCandidateManifest()
	root := writeReleaseTree(t, "2.15.0", "v2.15.0")
	writeNodePackageLock(t, root, "2.15.0", "")
	if ps := CheckSourceMetadata(m, root); !hasField(ps, "development.source.read") {
		t.Fatalf("CheckSourceMetadata(incomplete lock) = %v, want development.source.read", ps)
	}
}

func TestReleaseCheck_RequiresSnapshotInsteadOfCandidate(t *testing.T) {
	m := validCandidateManifest()
	root := writeReleaseTree(t, "2.15.0", "v2.15.0")
	if ps := ReleaseCheck(m, root, "v2.15.0"); !hasField(ps, "snapshot.required") {
		t.Fatalf("ReleaseCheck(candidate) = %v, want snapshot.required", ps)
	}
}

func TestReleaseSnapshot_IsTaggableWithoutClaimingPublication(t *testing.T) {
	m := validSnapshotManifest()
	root := writeReleaseTree(t, "2.15.0", "v2.15.0")
	if ps := Validate(m); len(ps) != 0 {
		t.Fatalf("Validate(snapshot) = %v", ps)
	}
	if ps := CheckSourceMetadata(m, root); len(ps) != 0 {
		t.Fatalf("CheckSourceMetadata(snapshot) = %v", ps)
	}
	if ps := ReleaseCheck(m, root, "v2.15.0"); len(ps) != 0 {
		t.Fatalf("ReleaseCheck(snapshot) = %v", ps)
	}
	for _, id := range []string{"release-status", "version-policy"} {
		got, err := RenderBlock(m, id)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "Prepared release snapshot") || !strings.Contains(got, "publication status is not asserted") {
			t.Fatalf("RenderBlock(%s) = %q, want neutral snapshot status", id, got)
		}
		if strings.Contains(got, "snapshot (not published)") {
			t.Fatalf("RenderBlock(%s) falsely claims snapshot is unpublished: %q", id, got)
		}
	}
}

func TestPostPublicationPromotion_RestoresDevelopmentState(t *testing.T) {
	m := validSnapshotManifest()
	m.Release.Version, m.Release.Tag = "2.15.0", "v2.15.0"
	m.Snapshot = nil
	for i := range m.Components {
		if m.Components[i].Tier == "stable" {
			m.Components[i].Version = m.Release.Version
		}
	}
	root := writeReleaseTree(t, "2.16.0-dev.0", "v2.15.0")
	if ps := Validate(m); len(ps) != 0 {
		t.Fatalf("Validate(promoted release) = %v", ps)
	}
	if ps := CheckSourceMetadata(m, root); len(ps) != 0 {
		t.Fatalf("CheckSourceMetadata(next development) = %v", ps)
	}
}
