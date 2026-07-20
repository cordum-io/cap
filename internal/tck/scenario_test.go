package tck

import (
	"strings"
	"testing"
)

const goodSuite = `{
  "version": 1,
  "scenarios": [
    {"id": "lifecycle/terminal-succeeded", "title": "b", "role": "worker", "profile": "core", "required": true},
    {"id": "lifecycle/happy-path", "title": "a", "role": "worker", "profile": "core", "required": true},
    {"id": "safety/deny-blocks-dispatch", "title": "c", "role": "control-plane", "profile": "cap-production", "required": true, "params": {"decision": "DENY"}}
  ]
}`

func TestLoadSuiteParsesAndSortsById(t *testing.T) {
	s, err := LoadSuite([]byte(goodSuite))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := []string{s.Scenarios[0].ID, s.Scenarios[1].ID, s.Scenarios[2].ID}
	want := []string{"lifecycle/happy-path", "lifecycle/terminal-succeeded", "safety/deny-blocks-dispatch"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scenario order[%d]=%q want %q (full %v)", i, got[i], want[i], got)
		}
	}
	if s.Scenarios[2].Params["decision"] != "DENY" {
		t.Fatalf("params not preserved: %+v", s.Scenarios[2])
	}
}

func TestLoadSuiteRejectsMalformedJSON(t *testing.T) {
	if _, err := LoadSuite([]byte("{not json")); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestLoadSuiteRejectsUnknownField(t *testing.T) {
	if _, err := LoadSuite([]byte(`{"version":1,"scenarios":[],"surprise":true}`)); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestLoadSuiteRejectsDuplicateID(t *testing.T) {
	dup := `{"version":1,"scenarios":[
      {"id":"x","role":"worker","profile":"core"},
      {"id":"x","role":"worker","profile":"core"}]}`
	if _, err := LoadSuite([]byte(dup)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
}

func TestLoadSuiteRejectsEmptyID(t *testing.T) {
	bad := `{"version":1,"scenarios":[{"id":"","role":"worker","profile":"core"}]}`
	if _, err := LoadSuite([]byte(bad)); err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("expected empty id error, got %v", err)
	}
}

func TestLoadSuiteRejectsUnknownRole(t *testing.T) {
	bad := `{"version":1,"scenarios":[{"id":"x","role":"gremlin","profile":"core"}]}`
	if _, err := LoadSuite([]byte(bad)); err == nil || !strings.Contains(err.Error(), "role") {
		t.Fatalf("expected role error, got %v", err)
	}
}

func TestLoadSuiteRejectsEmptyProfile(t *testing.T) {
	bad := `{"version":1,"scenarios":[{"id":"x","role":"worker","profile":""}]}`
	if _, err := LoadSuite([]byte(bad)); err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("expected profile error, got %v", err)
	}
}

func TestLoadSuiteRejectsBadVersion(t *testing.T) {
	if _, err := LoadSuite([]byte(`{"version":0,"scenarios":[]}`)); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("expected version error, got %v", err)
	}
}

func TestSelectFiltersByProfileDeterministically(t *testing.T) {
	s, err := LoadSuite([]byte(goodSuite))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	core := s.Select("core")
	if len(core) != 2 {
		t.Fatalf("expected 2 core scenarios, got %d", len(core))
	}
	if core[0].ID != "lifecycle/happy-path" || core[1].ID != "lifecycle/terminal-succeeded" {
		t.Fatalf("core selection not sorted: %v", []string{core[0].ID, core[1].ID})
	}
	if got := s.Select("cap-production"); len(got) != 1 || got[0].ID != "safety/deny-blocks-dispatch" {
		t.Fatalf("unexpected cap-production selection: %+v", got)
	}
}
