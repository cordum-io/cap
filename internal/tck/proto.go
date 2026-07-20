// Package tck is the Cordum Agent Protocol Technical Conformance Kit: a
// reusable harness that drives conformance scenarios against external adapter
// processes over a versioned line-delimited JSON contract (adapter-v1).
//
// The harness is transport- and implementation-agnostic. An adapter is any
// executable that speaks adapter-v1 on stdin/stdout; the runner never links an
// SDK directly, so the same suite exercises Go, Python, Node, or any future
// implementation identically. The bundled reference adapter tests the harness
// itself and is explicitly not an independent conformant implementation.
package tck

// ProtocolVersion is the adapter-v1 wire version the runner speaks. An adapter
// declaring a different version is rejected rather than guessed at.
const ProtocolVersion = 1

// HarnessVersion identifies the TCK build in emitted reports.
const HarnessVersion = "cap-tck/1"

// MsgType tags each line on the adapter-v1 wire.
type MsgType string

const (
	// MsgHello is the runner's opening request for an adapter handshake.
	MsgHello MsgType = "hello"
	// MsgHandshake is the adapter's declaration of identity and capability.
	MsgHandshake MsgType = "handshake"
	// MsgRun asks the adapter to execute one scenario case.
	MsgRun MsgType = "run"
	// MsgResult is the adapter's outcome for a single case.
	MsgResult MsgType = "result"
	// MsgBye asks the adapter to shut down cleanly.
	MsgBye MsgType = "bye"
)

// Status is the outcome of a single conformance case. FAIL and ERROR are kept
// distinct: FAIL is a conformance violation the adapter detected, ERROR is a
// harness/adapter fault. UNSUPPORTED and NA are non-outcomes — a required case
// reporting either makes the run fail rather than pass vacuously.
type Status string

const (
	StatusPass        Status = "PASS"
	StatusFail        Status = "FAIL"
	StatusError       Status = "ERROR"
	StatusUnsupported Status = "UNSUPPORTED"
	StatusNA          Status = "N/A"
)

// Role is the component boundary an adapter implements. A worker adapter runs
// worker-owned behavior; a control-plane adapter runs scheduler-owned
// transitions. Running a case against the wrong role yields N/A, never PASS.
type Role string

const (
	RoleWorker       Role = "worker"
	RoleControlPlane Role = "control-plane"
)

// ValidRole reports whether r is a role defined by adapter-v1. An unrecognized
// role must be rejected at the handshake: runCase maps a non-matching role to
// N/A, so an adapter that declares a bogus role would have every scenario ruled
// not-applicable and could exit 0 without ever running a required case.
func ValidRole(r Role) bool { return r == RoleWorker || r == RoleControlPlane }

// Command is a runner->adapter message.
type Command struct {
	Type     MsgType           `json:"type"`
	ID       string            `json:"id,omitempty"`
	Scenario string            `json:"scenario,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
}

// AdapterMessage is an adapter->runner message. It carries both handshake and
// result fields; Type selects which are meaningful. Params/Evidence are bounded
// string maps by design — the contract never carries an arbitrary payload.
type AdapterMessage struct {
	Type MsgType `json:"type"`
	ID   string  `json:"id,omitempty"`

	// Handshake fields.
	ProtocolVersion int      `json:"protocolVersion,omitempty"`
	Role            Role     `json:"role,omitempty"`
	Profiles        []string `json:"profiles,omitempty"`
	Features        []string `json:"features,omitempty"`
	SDK             string   `json:"sdk,omitempty"`
	Transport       string   `json:"transport,omitempty"`

	// Result fields.
	Status   Status            `json:"status,omitempty"`
	Detail   string            `json:"detail,omitempty"`
	Evidence map[string]string `json:"evidence,omitempty"`
}

// ValidStatus reports whether s is a known outcome.
func ValidStatus(s Status) bool {
	switch s {
	case StatusPass, StatusFail, StatusError, StatusUnsupported, StatusNA:
		return true
	default:
		return false
	}
}

// IsRequiredNonPass reports whether an outcome fails a case that was required
// and applicable to the adapter. Only PASS satisfies such a case; FAIL, ERROR,
// UNSUPPORTED, and an adapter-reported N/A are all failures — a suite must not
// pass by declining its own hardest cases. Inapplicability (a scenario the
// adapter's role does not own) is handled by the runner before this is
// consulted, so it never reaches here as a false failure.
func IsRequiredNonPass(s Status) bool {
	return s != StatusPass
}
