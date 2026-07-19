package tck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Scenario is one declarative conformance case. It is black-box: the runner
// sends ID+Params to an adapter and the adapter reports the outcome. The
// scenario carries only the metadata the runner needs to schedule and grade it.
type Scenario struct {
	ID       string            `json:"id"`
	Title    string            `json:"title,omitempty"`
	Role     Role              `json:"role"`
	Profile  string            `json:"profile"`
	Required bool              `json:"required,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
}

// Suite is a loaded, validated set of scenarios in deterministic order.
type Suite struct {
	Version   int        `json:"version"`
	Scenarios []Scenario `json:"scenarios"`
}

// LoadSuite parses and validates a scenario suite, rejecting unknown fields so
// contract drift cannot pass silently, and sorts scenarios by ID so case order
// and report output are deterministic regardless of authoring order.
func LoadSuite(data []byte) (*Suite, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s Suite
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("tck: parse suite: %w", err)
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	sort.Slice(s.Scenarios, func(i, j int) bool { return s.Scenarios[i].ID < s.Scenarios[j].ID })
	return &s, nil
}

// validate enforces suite invariants, reporting the first offending field.
func (s *Suite) validate() error {
	if s.Version < 1 {
		return fmt.Errorf("tck: suite version must be >= 1, got %d", s.Version)
	}
	seen := map[string]bool{}
	for _, sc := range s.Scenarios {
		if sc.ID == "" {
			return fmt.Errorf("tck: scenario id must not be empty")
		}
		if seen[sc.ID] {
			return fmt.Errorf("tck: duplicate scenario id %q", sc.ID)
		}
		seen[sc.ID] = true
		if sc.Role != RoleWorker && sc.Role != RoleControlPlane {
			return fmt.Errorf("tck: scenario %q has unknown role %q", sc.ID, sc.Role)
		}
		if sc.Profile == "" {
			return fmt.Errorf("tck: scenario %q has empty profile", sc.ID)
		}
	}
	return nil
}

// Select returns the scenarios in a profile, in deterministic ID order.
func (s *Suite) Select(profile string) []Scenario {
	var out []Scenario
	for _, sc := range s.Scenarios {
		if sc.Profile == profile {
			out = append(out, sc)
		}
	}
	return out
}

// Profiles returns the distinct profiles present, sorted.
func (s *Suite) Profiles() []string {
	set := map[string]bool{}
	for _, sc := range s.Scenarios {
		set[sc.Profile] = true
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
