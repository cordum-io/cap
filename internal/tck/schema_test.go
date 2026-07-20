package tck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func schemaDir() string { return filepath.Join("..", "..", "spec", "tck", "schema") }

func loadSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(schemaDir(), name))
	if err != nil {
		t.Fatalf("read schema %s: %v", name, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("schema %s is not valid JSON: %v", name, err)
	}
	return m
}

func propertyNames(t *testing.T, schema map[string]any) map[string]bool {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties object")
	}
	out := map[string]bool{}
	for k := range props {
		out[k] = true
	}
	return out
}

func TestSchemasAreValidAndTitled(t *testing.T) {
	for _, name := range []string{"scenario.schema.json", "adapter-message.schema.json", "report.schema.json"} {
		s := loadSchema(t, name)
		if s["$schema"] == nil || s["title"] == nil {
			t.Fatalf("%s missing $schema or title", name)
		}
	}
}

// jsonKeys marshals v and returns its top-level object keys.
func jsonKeys(t *testing.T, v any) map[string]bool {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// TestReportMatchesSchema guards drift: every field the report emits must be a
// declared schema property. A new Go field without a schema update fails here.
func TestReportMatchesSchema(t *testing.T) {
	schema := loadSchema(t, "report.schema.json")
	props := propertyNames(t, schema)
	rep := NewReport("go", sampleHS(), "core", "v", sampleCases())
	for k := range jsonKeys(t, rep) {
		if !props[k] {
			t.Fatalf("report field %q is not declared in report.schema.json", k)
		}
	}
	for _, req := range []string{"adapter", "profile", "summary", "cases"} {
		if !jsonKeys(t, rep)[req] {
			t.Fatalf("report missing schema-required field %q", req)
		}
	}
}

// Guards adapter-v1 wire drift: every field AdapterMessage or Command emits must
// be a declared property of adapter-message.schema.json.
func TestAdapterMessageMatchesSchema(t *testing.T) {
	schema := loadSchema(t, "adapter-message.schema.json")
	props := propertyNames(t, schema)
	full := AdapterMessage{
		Type: MsgResult, ID: "c1", ProtocolVersion: 1, Role: RoleWorker,
		Profiles: []string{"core"}, Features: []string{"cancel"}, SDK: "go",
		Transport: "nats", Status: StatusPass, Detail: "d", Evidence: map[string]string{"a": "b"},
	}
	cmd := Command{Type: MsgRun, ID: "c1", Scenario: "s", Params: map[string]string{"a": "b"}}
	for _, keys := range []map[string]bool{jsonKeys(t, full), jsonKeys(t, cmd)} {
		for k := range keys {
			if !props[k] {
				t.Fatalf("wire field %q is not declared in adapter-message.schema.json", k)
			}
		}
	}
}

func TestScenarioMatchesSchema(t *testing.T) {
	schema := loadSchema(t, "scenario.schema.json")
	defs := schema["$defs"].(map[string]any)
	scen := defs["scenario"].(map[string]any)
	props := propertyNames(t, scen)
	full := Scenario{ID: "x", Title: "t", Role: RoleWorker, Profile: "core", Required: true, Params: map[string]string{"a": "b"}}
	for k := range jsonKeys(t, full) {
		if !props[k] {
			t.Fatalf("scenario field %q is not declared in scenario.schema.json", k)
		}
	}
}
