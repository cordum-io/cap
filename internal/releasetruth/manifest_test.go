package releasetruth

import "testing"

func TestLoad_Valid(t *testing.T) {
	m, err := Load(validManifestJSON(t))
	if err != nil {
		t.Fatalf("Load(valid) unexpected error: %v", err)
	}
	if m.Release.Version != "2.14.0" {
		t.Errorf("Release.Version = %q, want 2.14.0", m.Release.Version)
	}
	if m.Wire.ProtocolVersion != 1 {
		t.Errorf("Wire.ProtocolVersion = %d, want 1", m.Wire.ProtocolVersion)
	}
	if len(m.Specs) != 2 {
		t.Errorf("len(Specs) = %d, want 2", len(m.Specs))
	}
}

func TestLoad_RejectsUnknownField(t *testing.T) {
	bad := []byte(`{"schemaVersion":"1.0.0","bogusField":true}`)
	if _, err := Load(bad); err == nil {
		t.Fatal("Load rejected nothing; want error for unknown field bogusField")
	}
}

func TestLoad_RejectsMalformed(t *testing.T) {
	if _, err := Load([]byte("{not json")); err == nil {
		t.Fatal("Load accepted malformed JSON; want error")
	}
}

func TestLoad_RejectsEmpty(t *testing.T) {
	if _, err := Load(nil); err == nil {
		t.Fatal("Load accepted empty input; want error")
	}
}
