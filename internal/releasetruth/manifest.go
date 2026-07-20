// Package releasetruth defines the canonical machine-readable release manifest
// for CAP and a strict, network-free parser and validator. The manifest is the
// single hand-edited source of current release, wire, SDK, transport,
// toolchain, and security-support truth; docs and CI derive from it rather than
// maintaining parallel matrices that drift.
package releasetruth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Manifest is the top-level release-truth document.
type Manifest struct {
	Schema        string      `json:"$schema,omitempty"`
	SchemaVersion string      `json:"schemaVersion"`
	Release       Release     `json:"release"`
	Development   Development `json:"development"`
	Wire          Wire        `json:"wire"`
	Specs         []Spec      `json:"specs"`
	Components    []Component `json:"components"`
	Transports    []Transport `json:"transports"`
	Toolchains    Toolchains  `json:"toolchains"`
	Security      Security    `json:"security"`
	Links         []Link      `json:"links"`
}

// Release identifies the latest published artifact line.
type Release struct {
	Version string `json:"version"`
	Tag     string `json:"tag"`
	Date    string `json:"date"`
	Commit  string `json:"commit"`
	Channel string `json:"channel"`
}

// Development identifies unreleased source state, kept strictly distinct from
// Release so changed source is never relabelled as an already-published
// version.
type Development struct {
	Commit   string `json:"commit"`
	Describe string `json:"describe"`
	Released bool   `json:"released"`
}

// Wire captures the protocol version and the accepted compatibility range.
type Wire struct {
	ProtocolVersion int    `json:"protocolVersion"`
	SchemaVersion   string `json:"schemaVersion"`
	CompatMin       int    `json:"compatMin"`
	CompatMax       int    `json:"compatMax"`
}

// Spec is one ordered normative specification document.
type Spec struct {
	ID    string `json:"id"`
	File  string `json:"file"`
	Title string `json:"title"`
}

// Component is a shipped SDK or extension.
type Component struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Tier        string `json:"tier"`
	Language    string `json:"language"`
	Package     string `json:"package"`
	Registry    string `json:"registry"`
	Import      string `json:"import"`
	Version     string `json:"version"`
	Toolchain   string `json:"toolchain"`
	Publication string `json:"publication"`
	Evidence    string `json:"evidence"`
}

// Transport is a message-bus binding and its support state.
type Transport struct {
	Name        string   `json:"name"`
	State       string   `json:"state"`
	Profiles    []string `json:"profiles,omitempty"`
	Limitations string   `json:"limitations,omitempty"`
	Evidence    string   `json:"evidence,omitempty"`
}

// ToolReq is a tested/minimum toolchain pair.
type ToolReq struct {
	Tested  string `json:"tested"`
	Minimum string `json:"minimum"`
}

// Toolchains lists tested and minimum toolchains per stable language.
type Toolchains struct {
	Go     ToolReq `json:"go"`
	Node   ToolReq `json:"node"`
	Python ToolReq `json:"python"`
}

// Security lists supported release lines and the vulnerability reporting route.
type Security struct {
	SupportedLines []string `json:"supportedLines"`
	ReportingRoute string   `json:"reportingRoute"`
}

// Link is a canonical named repository or web link.
type Link struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Problem is a single validation failure carrying a stable field identifier so
// callers and tests can assert on specific rules.
type Problem struct {
	Field string
	Msg   string
}

func (p Problem) Error() string { return p.Field + ": " + p.Msg }

// Load strictly decodes manifest bytes, rejecting unknown fields and trailing
// data so drift or typos fail closed rather than being silently ignored.
func Load(data []byte) (*Manifest, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty manifest")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("unexpected trailing data after manifest object")
	}
	return &m, nil
}

// LoadFile reads and strictly decodes a manifest file.
func LoadFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	return Load(data)
}
