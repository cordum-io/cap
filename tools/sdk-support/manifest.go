// Package sdksupport models the machine-readable SDK support manifest and
// verifies it against evidence that actually exists in the repository.
//
// The manifest never carries hand-set pass booleans: every claim an entry makes
// is expressed as a gate that names a real workflow or test path, and Verify
// resolves each of those paths against the repository tree.
package sdksupport

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
	GateManifest GateKind = "manifest" // a package manifest proving a publish coordinate
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
	CodeOwnerPathMissing    = "owner_path_missing"
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
	// Pending marks a gate whose evidence file exists but whose content is not
	// yet earned (e.g. a real-NATS test that still skips). BlockedBy names the
	// task that will complete it. A pending gate is surfaced honestly rather
	// than presented as satisfied.
	Pending   bool   `json:"pending,omitempty"`
	BlockedBy string `json:"blockedBy,omitempty"`
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

// PendingGates returns the gates this entry marks as not-yet-earned, so a
// caller can report exactly which evidence is still outstanding.
func (e Entry) PendingGates() []Gate {
	var out []Gate
	for _, g := range e.Gates {
		if g.Pending {
			out = append(out, g)
		}
	}
	return out
}

// FullyEarned reports whether every gate on the entry is satisfied (none
// pending). Only a fully-earned stable entry is claim-clean.
func (e Entry) FullyEarned() bool { return len(e.PendingGates()) == 0 }

// Problem is a single verification failure, tied to the entry that caused it.
type Problem struct {
	EntryID string
	Code    string
	Detail  string
}
