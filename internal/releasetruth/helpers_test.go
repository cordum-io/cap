package releasetruth

import (
	"encoding/json"
	"testing"
)

// validManifest returns a fully-valid manifest used as the baseline for
// negative tests: each case mutates exactly one field and asserts the
// corresponding problem is reported. Its values mirror the verified
// release truth at origin/main@ed0d8bd (v2.14.0).
func validManifest() *Manifest {
	return &Manifest{
		SchemaVersion: "1.1.0",
		Release: Release{
			Version: "2.14.0",
			Tag:     "v2.14.0",
			Date:    "2026-06-02",
			Commit:  "e65a0a8392f552a8e14969a4efb4fe8d4b11a901",
			Channel: "stable",
		},
		Development: Development{
			Commit:   "ed0d8bdcc45a45352a3eaeb612744aaeaf45b96d",
			Describe: "v2.14.0-8-ged0d8bd",
			Released: false,
		},
		Wire: Wire{ProtocolVersion: 1, SchemaVersion: "1.0.0", CompatMin: 1, CompatMax: 1},
		Specs: []Spec{
			{ID: "01", File: "spec/01-overview.md", Title: "Overview"},
			{ID: "04b", File: "spec/04b-context-and-memory.md", Title: "Context and Memory"},
		},
		Components: []Component{goComponent(), guardComponent()},
		Transports: []Transport{
			{Name: "nats", State: "supported", Profiles: []string{"core"}, Evidence: "ci-e2e-nats"},
			{Name: "kafka", State: "experimental"},
		},
		Toolchains: Toolchains{
			Go:     ToolReq{Tested: "1.25.12", Minimum: "1.24"},
			Node:   ToolReq{Tested: "22", Minimum: "20"},
			Python: ToolReq{Tested: "3.12", Minimum: "3.10"},
		},
		Security: Security{SupportedLines: []string{"2.14.x"}, ReportingRoute: "SECURITY.md"},
		Links:    []Link{{Name: "spec-index", URL: "spec/00-index.md"}},
	}
}

func goComponent() Component {
	return Component{
		Name: "cap-go", Kind: "sdk", Tier: "stable", Language: "go",
		Package:     "github.com/cordum-io/cap/v2",
		Registry:    "proxy.golang.org",
		Import:      "github.com/cordum-io/cap/v2/sdk/go",
		Version:     "2.14.0",
		Toolchain:   "go1.25.12",
		Publication: "published",
		Evidence:    "ci-e2e-nats",
	}
}

func guardComponent() Component {
	return Component{
		Name: "cordum-guard", Kind: "extension", Tier: "stable", Language: "python",
		Package:     "cordum-guard",
		Registry:    "pypi.org",
		Import:      "cordum_guard",
		Version:     "2.14.0",
		Toolchain:   "python3.12",
		Publication: "published",
		Evidence:    "unit",
	}
}

// validManifestJSON serialises the baseline manifest to JSON so decode tests
// exercise the real strict parser rather than a hand-maintained literal.
func validManifestJSON(t *testing.T) []byte {
	t.Helper()
	return mustMarshalManifest(t, validManifest())
}

func mustMarshalManifest(t *testing.T, m *Manifest) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal baseline manifest: %v", err)
	}
	return b
}

// hasField reports whether any problem carries the exact field identifier.
func hasField(ps []Problem, field string) bool {
	for _, p := range ps {
		if p.Field == field {
			return true
		}
	}
	return false
}
