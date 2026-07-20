// Command cap-codegen holds the checked-in generated tree to a pinned manifest.
//
//	cap-codegen check     verify the tree matches tools/codegen/manifest.json
//	cap-codegen snapshot  rebuild the manifest from the current tree
//
// Generation itself runs in the pinned container (tools/codegen/Dockerfile via
// codegen.sh / codegen.ps1); this command is the verification/authoring half.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cordum-io/cap/v2/tools/codegen"
)

const manifestPath = "tools/codegen/manifest.json"

// canonicalSources are the seven proto inputs, sorted.
var canonicalSources = []string{
	"proto/cordum/agent/v1/alert.proto",
	"proto/cordum/agent/v1/buspacket.proto",
	"proto/cordum/agent/v1/handshake.proto",
	"proto/cordum/agent/v1/heartbeat.proto",
	"proto/cordum/agent/v1/job.proto",
	"proto/cordum/agent/v1/policy.proto",
	"proto/cordum/agent/v1/safety.proto",
}

// generatorRules attribute each checked-in generated file to its generator.
// Globs are deliberately narrow — only true generated suffixes in generated
// directories — so no hand-written file is ever recorded as generated.
var generatorRules = []codegen.GeneratorRule{
	{ID: "protoc-gen-go", Globs: []string{"cordum/agent/v1/*.pb.go"}},
	{ID: "protoc-gen-cpp", Globs: []string{"cpp/cordum/agent/v1/*.pb.cc", "cpp/cordum/agent/v1/*.pb.h"}},
	{ID: "protoc-gen-js", Globs: []string{"node/*_pb.js", "node/*_pb.d.ts", "node/cordum/agent/v1/*_pb.js", "node/cordum/agent/v1/*_pb.d.ts"}},
	// Both Python surfaces emit a message module and a gRPC stub module per
	// proto. `*_pb2.py` does not match `*_pb2_grpc.py` under path.Match, so the
	// stubs need their own glob — without it 14 generated files are absent from
	// the manifest and Check cannot see them drift.
	{ID: "protoc-gen-python-root", Globs: []string{"python/cordum/agent/v1/*_pb2.py", "python/cordum/agent/v1/*_pb2_grpc.py"}},
	{ID: "protoc-gen-python-sdk", Globs: []string{"sdk/python/cap/pb/cordum/agent/v1/*_pb2.py", "sdk/python/cap/pb/cordum/agent/v1/*_pb2_grpc.py"}},
	{ID: "protoc-gen-ruby", Globs: []string{"sdk/ruby/proto/cordum/agent/v1/*_pb.rb"}},
}

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: cap-codegen check|snapshot")
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(stdout, stderr)
	case "snapshot":
		return runSnapshot(stderr)
	default:
		fmt.Fprintf(stderr, "cap-codegen: unknown command %q\n", args[0])
		return 2
	}
}

func runCheck(stdout, stderr io.Writer) int {
	m, err := loadManifest()
	if err != nil {
		fmt.Fprintf(stderr, "cap-codegen: %v\n", err)
		return 1
	}
	problems := codegen.Check(m, os.DirFS("."))
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(stderr, "DRIFT %s %s %s\n", p.Code, p.Path, p.Detail)
		}
		return 1
	}
	fmt.Fprintf(stdout, "cap-codegen: %d generated outputs match the manifest\n", len(m.Outputs))
	return 0
}

func runSnapshot(stderr io.Writer) int {
	m, err := codegen.Snapshot(os.DirFS("."), canonicalSources, generatorRules)
	if err != nil {
		fmt.Fprintf(stderr, "cap-codegen: snapshot: %v\n", err)
		return 1
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "cap-codegen: marshal: %v\n", err)
		return 1
	}
	if err := os.WriteFile(manifestPath, append(body, '\n'), 0o644); err != nil {
		fmt.Fprintf(stderr, "cap-codegen: write: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "cap-codegen: wrote %s (%d outputs)\n", manifestPath, len(m.Outputs))
	return 0
}

func loadManifest() (*codegen.Manifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var m codegen.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestPath, err)
	}
	return &m, nil
}
