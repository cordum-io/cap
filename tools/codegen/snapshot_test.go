package codegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func snapshotRules() []GeneratorRule {
	return []GeneratorRule{
		{ID: "go", Globs: []string{"gen/*.pb.go"}},
		{ID: "py", Globs: []string{"gen/*_pb2.py"}},
	}
}

func snapshotFS() fstest.MapFS {
	return fstest.MapFS{
		"proto/a.proto":    {Data: []byte("syntax=\"proto3\";")},
		"gen/a.pb.go":      {Data: []byte("// go a\n")},
		"gen/b.pb.go":      {Data: []byte("// go b\n")},
		"gen/a_pb2.py":     {Data: []byte("# py a\n")},
		"handwritten/x.go": {Data: []byte("package x\n")},
		"gen/README.md":    {Data: []byte("not generated\n")},
	}
}

func TestSnapshotThenCheckIsGreen(t *testing.T) {
	root := snapshotFS()
	m, err := Snapshot(root, []string{"proto/a.proto"}, snapshotRules())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if problems := Check(m, root); len(problems) != 0 {
		t.Fatalf("a fresh snapshot must Check clean, got: %+v", problems)
	}
}

// Snapshot must record only glob-matched generated files, never a hand-written
// file or an unrelated doc that happens to sit in a generated directory.
func TestSnapshotRecordsOnlyGeneratedFiles(t *testing.T) {
	m, err := Snapshot(snapshotFS(), []string{"proto/a.proto"}, snapshotRules())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(m.Outputs) != 3 {
		t.Fatalf("expected 3 generated outputs, got %d: %+v", len(m.Outputs), m.Outputs)
	}
	for _, o := range m.Outputs {
		if o.Path == "handwritten/x.go" || o.Path == "gen/README.md" {
			t.Fatalf("non-generated file recorded: %s", o.Path)
		}
	}
}

// After a snapshot, changing a generated file's bytes must be caught as drift.
func TestSnapshotThenMutatedFileFailsCheck(t *testing.T) {
	root := snapshotFS()
	m, err := Snapshot(root, []string{"proto/a.proto"}, snapshotRules())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	root["gen/a.pb.go"] = &fstest.MapFile{Data: []byte("// tampered\n")}
	if problems := Check(m, root); !hasCode(problems, CodeOutputStale) {
		t.Fatalf("expected stale-output drift, got: %+v", problems)
	}
}

// A new generated file appearing without a manifest update must be caught.
func TestSnapshotThenNewUnmanifestedFileFailsCheck(t *testing.T) {
	root := snapshotFS()
	m, err := Snapshot(root, []string{"proto/a.proto"}, snapshotRules())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	root["gen/c.pb.go"] = &fstest.MapFile{Data: []byte("// new\n")}
	if problems := Check(m, root); !hasCode(problems, CodeOutputUnmanifested) {
		t.Fatalf("expected unmanifested drift, got: %+v", problems)
	}
}

// A CRLF working tree (Windows autocrlf) and an LF working tree (Linux CI) of
// the same generated file must not read as drift — the manifest pins canonical
// LF content.
func TestCheckIsLineEndingIndependent(t *testing.T) {
	lf := fstest.MapFS{"gen/a.pb.go": {Data: []byte("// line1\n// line2\n")}}
	m, err := Snapshot(lf, nil, snapshotRules())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	crlf := fstest.MapFS{"gen/a.pb.go": {Data: []byte("// line1\r\n// line2\r\n")}}
	if problems := Check(m, crlf); len(problems) != 0 {
		t.Fatalf("CRLF vs LF must not be drift, got: %+v", problems)
	}
}

// The shipped manifest must match the real repository tree: this is the live
// drift-detection anchor for CI.
func TestShippedManifestMatchesTree(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("manifest.json"))
	if err != nil {
		t.Fatalf("read shipped manifest: %v", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m.Outputs) == 0 {
		t.Fatal("shipped manifest has no outputs")
	}
	root := os.DirFS(filepath.Join("..", ".."))
	if problems := Check(&m, root); len(problems) != 0 {
		t.Fatalf("shipped manifest does not match tree (drift): %+v", problems)
	}
}
