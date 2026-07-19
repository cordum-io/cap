// Package sdksupport models the machine-readable SDK support manifest and
// verifies it against evidence that actually exists in the repository.
//
// The manifest never carries hand-set pass booleans: every claim an entry makes
// is expressed as a gate that names a real workflow or test path, and Verify
// resolves each of those paths against the repository tree.
package sdksupport

import "io/fs"

// Kind separates what a component *is* from how well it is supported. A Python
// Guard extension is not a protocol SDK and must never appear in wire matrices.
type Kind string

const (
	KindProtocolSDK Kind = "protocol-sdk"
	KindExtension   Kind = "extension"
)

// Tier is the support level a component claims.
type Tier string

const (
	TierStable       Tier = "stable"
	TierCommunity    Tier = "community"
	TierExperimental Tier = "experimental"
)

// GateKind describes how a gate's evidence is located.
type GateKind string

const (
	GateWorkflow GateKind = "workflow"
	GateTest     GateKind = "test"
)

// RequiredStableGates are the gate IDs a stable entry must declare. A stable
// claim without every one of these is not verifiable and must fail.
var RequiredStableGates = []string{"publish", "install", "compat", "tck", "real-nats"}

// Problem codes reported by Verify.
const (
	CodeUnknownTier         = "unknown_tier"
	CodeUnknownKind         = "unknown_kind"
	CodeDuplicateID         = "duplicate_id"
	CodeDuplicatePackage    = "duplicate_package"
	CodeMissingOwner        = "missing_owner"
	CodeMissingDocs         = "missing_docs"
	CodeDocsPathMissing     = "docs_path_missing"
	CodeGatePathMissing     = "gate_path_missing"
	CodeMissingRequiredGate = "missing_required_gate"
	CodeUnknownGateKind     = "unknown_gate_kind"
	CodeDuplicateGateID     = "duplicate_gate_id"
)

// Gate names a single piece of executable evidence.
type Gate struct {
	ID   string   `json:"id"`
	Kind GateKind `json:"kind"`
	Path string   `json:"path"`
	Job  string   `json:"job,omitempty"`
}

// Package is the registry coordinate a component publishes under.
type Package struct {
	Registry   string `json:"registry"`
	Coordinate string `json:"coordinate"`
}

// Entry is one component in the manifest.
type Entry struct {
	ID        string   `json:"id"`
	Kind      Kind     `json:"kind"`
	Tier      Tier     `json:"tier"`
	Owner     string   `json:"owner"`
	OwnerPath string   `json:"ownerPath"`
	Package   *Package `json:"package,omitempty"`
	Docs      string   `json:"docs"`
	Gates     []Gate   `json:"gates"`
}

// Manifest is the whole support-tiers document.
type Manifest struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

// Problem is a single verification failure, tied to the entry that caused it.
type Problem struct {
	EntryID string
	Code    string
	Detail  string
}

// Verify checks the manifest against evidence resolvable in root and returns
// every problem found. An empty result means the manifest is fully backed by
// files that exist.
//
// NOT IMPLEMENTED: returns nil so the behavioral tests fail RED first.
func Verify(m *Manifest, root fs.FS) []Problem {
	return nil
}
