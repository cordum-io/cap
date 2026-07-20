package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"testing/fstest"
)

const (
	protoPath = "proto/cordum/agent/v1/job.proto"
	goOutPath = "cordum/agent/v1/job.pb.go"
	pyOutPath = "python/cordum/agent/v1/job_pb2.py"
)

var (
	goOutBody = []byte("// generated go\n")
	pyOutBody = []byte("# generated python\n")
)

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// validRoot holds exactly the sources and outputs validManifest declares.
func validRoot() fstest.MapFS {
	return fstest.MapFS{
		protoPath: {Data: []byte("syntax = \"proto3\";\n")},
		goOutPath: {Data: goOutBody},
		pyOutPath: {Data: pyOutBody},
	}
}

func validManifest() *Manifest {
	return &Manifest{
		Version: 1,
		Sources: []string{protoPath},
		Outputs: []Output{
			{Path: goOutPath, SHA256: digest(goOutBody)},
			{Path: pyOutPath, SHA256: digest(pyOutBody)},
		},
		GeneratedGlobs: []string{"cordum/agent/v1/*.pb.go", "python/cordum/agent/v1/*_pb2.py"},
		Generators: []Generator{
			{ID: "protoc-gen-go", Outputs: []string{goOutPath}},
			{ID: "protoc-gen-python", Outputs: []string{pyOutPath}},
		},
	}
}

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

func TestCheckAcceptsExactlyMatchingTree(t *testing.T) {
	problems := Check(validManifest(), validRoot())
	if len(problems) != 0 {
		t.Fatalf("expected 0 problems for a matching tree, got %d: %+v", len(problems), problems)
	}
}

// Sorted output paths keep review diffs stable and make duplicates detectable.
func TestCheckRejectsUnsortedOutputPaths(t *testing.T) {
	m := validManifest()
	m.Outputs[0], m.Outputs[1] = m.Outputs[1], m.Outputs[0]
	assertCode(t, Check(m, validRoot()), CodeUnsortedPaths)
}

func TestCheckRejectsDuplicateOutputPath(t *testing.T) {
	m := validManifest()
	m.Outputs = append(m.Outputs, m.Outputs[0])
	assertCode(t, Check(m, validRoot()), CodeDuplicatePath)
}

// A manifest path is attacker-influenced input to a writer; escaping the tree
// must be refused rather than normalized away.
func TestCheckRejectsPathTraversal(t *testing.T) {
	m := validManifest()
	m.Outputs[0].Path = "../outside/job.pb.go"
	assertCode(t, Check(m, validRoot()), CodePathTraversal)
}

func TestCheckRejectsAbsolutePath(t *testing.T) {
	m := validManifest()
	m.Outputs[0].Path = "/etc/job.pb.go"
	assertCode(t, Check(m, validRoot()), CodeAbsolutePath)
}

func TestCheckRejectsMissingSource(t *testing.T) {
	m := validManifest()
	m.Sources = append(m.Sources, "proto/cordum/agent/v1/ghost.proto")
	assertCode(t, Check(m, validRoot()), CodeSourceMissing)
}

func TestCheckRejectsDeclaredOutputMissingOnDisk(t *testing.T) {
	root := validRoot()
	delete(root, pyOutPath)
	assertCode(t, Check(validManifest(), root), CodeOutputMissing)
}

// Stale output is the false-green this checker exists to catch: the file is
// present, so a naive existence check passes, but its bytes are not what the
// pinned generator produces.
func TestCheckRejectsStaleOutputBytes(t *testing.T) {
	root := validRoot()
	root[goOutPath] = &fstest.MapFile{Data: []byte("// hand-edited\n")}
	assertCode(t, Check(validManifest(), root), CodeOutputStale)
}

func TestCheckRejectsEmptyOutput(t *testing.T) {
	root := validRoot()
	root[goOutPath] = &fstest.MapFile{Data: []byte{}}
	m := validManifest()
	m.Outputs[0].SHA256 = digest(nil)
	assertCode(t, Check(m, root), CodeOutputEmpty)
}

// An artifact on disk that the manifest does not declare means generation and
// declaration have diverged; it must fail rather than be ignored.
func TestCheckRejectsUnmanifestedGeneratedArtifact(t *testing.T) {
	root := validRoot()
	root["cordum/agent/v1/policy.pb.go"] = &fstest.MapFile{Data: []byte("// stray\n")}
	assertCode(t, Check(validManifest(), root), CodeOutputUnmanifested)
}

// A generator declaring no outputs is a skipped generator wearing a green
// badge. This is the exact false-green in the current make_protos.sh.
func TestCheckRejectsGeneratorWithNoOutputs(t *testing.T) {
	m := validManifest()
	m.Generators = append(m.Generators, Generator{ID: "protoc-gen-cpp"})
	assertCode(t, Check(m, validRoot()), CodeGeneratorNoOutputs)
}

func TestCheckRejectsGeneratorReferencingUnknownOutput(t *testing.T) {
	m := validManifest()
	m.Generators[0].Outputs = []string{"cordum/agent/v1/nope.pb.go"}
	assertCode(t, Check(m, validRoot()), CodeUnknownOutputRef)
}

// Every declared output must be claimed by some generator, otherwise an
// artifact can be checked in that no pinned generator reproduces. This is the
// reverse of CodeUnknownOutputRef and must report its own code.
func TestCheckRejectsOutputNotClaimedByAnyGenerator(t *testing.T) {
	m := validManifest()
	m.Generators = m.Generators[:1] // drops the python generator
	assertCode(t, Check(m, validRoot()), CodeOutputUnclaimed)
}
