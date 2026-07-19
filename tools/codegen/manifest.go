// Package codegen models the hermetic code-generation manifest and checks the
// checked-in generated tree against it.
//
// The manifest is the single declaration of which proto sources exist and
// exactly which generated artifacts they must produce. Check compares that
// declaration against the real tree in both directions: every declared output
// must exist with the expected bytes, and no generated artifact may exist that
// the manifest does not declare. A generator that produces nothing is a hard
// failure rather than a silent skip.
package codegen

import "io/fs"

// Problem codes reported by Check.
const (
	CodeUnsortedPaths      = "unsorted_paths"
	CodeDuplicatePath      = "duplicate_path"
	CodePathTraversal      = "path_traversal"
	CodeAbsolutePath       = "absolute_path"
	CodeSourceMissing      = "source_missing"
	CodeOutputMissing      = "output_missing"
	CodeOutputStale        = "output_stale"
	CodeOutputEmpty        = "output_empty"
	CodeOutputUnmanifested = "output_unmanifested"
	CodeGeneratorNoOutputs = "generator_no_outputs"
	CodeUnknownOutputRef   = "unknown_output_ref"
	CodeOutputUnclaimed    = "output_unclaimed"
)

// Output is one expected checked-in generated artifact, pinned by content.
type Output struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Generator is a required generation step. Outputs lists the Output paths it is
// responsible for; an empty list means the generator was skipped.
type Generator struct {
	ID      string   `json:"id"`
	Outputs []string `json:"outputs"`
}

// Manifest declares the whole generation contract.
type Manifest struct {
	Version int      `json:"version"`
	Sources []string `json:"sources"`
	Outputs []Output `json:"outputs"`
	// GeneratedGlobs bound the search for artifacts that exist on disk but are
	// absent from Outputs, so stray or stale generated files cannot hide.
	GeneratedGlobs []string    `json:"generatedGlobs"`
	Generators     []Generator `json:"generators"`
}

// Problem is a single check failure.
type Problem struct {
	Code   string
	Path   string
	Detail string
}

// Check validates the manifest's internal shape and compares it against root.
// An empty result means the generated tree exactly matches the declaration.
//
// NOT IMPLEMENTED: returns nil so the behavioral tests fail RED first.
func Check(m *Manifest, root fs.FS) []Problem {
	return nil
}
