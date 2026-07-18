package releasetruth

import "testing"

func TestValidate_ValidHasNoProblems(t *testing.T) {
	if ps := Validate(validManifest()); len(ps) != 0 {
		t.Fatalf("Validate(valid) = %d problems, want 0: %v", len(ps), ps)
	}
}

// TestValidate_Rules mutates exactly one field of the valid baseline per case
// and asserts the specific problem field is reported. Each case is
// mutation-resistant: reverting the guard in production code fails exactly one.
func TestValidate_Rules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Manifest)
		field  string
	}{
		{"non-semver version", func(m *Manifest) { m.Release.Version = "2.14" }, "release.version"},
		{"tag not v+version", func(m *Manifest) { m.Release.Tag = "v2.13.0" }, "release.tag"},
		{"non-iso date", func(m *Manifest) { m.Release.Date = "June 2 2026" }, "release.date"},
		{"short commit", func(m *Manifest) { m.Release.Commit = "abc" }, "release.commit"},
		{"unknown channel", func(m *Manifest) { m.Release.Channel = "prod" }, "release.channel"},
		{"dev marked released", func(m *Manifest) { m.Development.Released = true }, "development.released"},
		{"empty describe", func(m *Manifest) { m.Development.Describe = "" }, "development.describe"},
		{"protocol below one", func(m *Manifest) { m.Wire.ProtocolVersion = 0 }, "wire.protocolVersion"},
		{"wire schema not semver", func(m *Manifest) { m.Wire.SchemaVersion = "1.0" }, "wire.schemaVersion"},
		{"compat excludes protocol", func(m *Manifest) { m.Wire.CompatMin = 2; m.Wire.CompatMax = 3 }, "wire.compat"},
		{"no specs", func(m *Manifest) { m.Specs = nil }, "specs.empty"},
		{"duplicate spec id", func(m *Manifest) { m.Specs[1].ID = "01" }, "specs.id.duplicate"},
		{"traversal spec path", func(m *Manifest) { m.Specs[0].File = "../etc/passwd" }, "specs.file"},
		{"absolute spec path", func(m *Manifest) { m.Specs[0].File = "/etc/passwd" }, "specs.file"},
		{"no components", func(m *Manifest) { m.Components = nil }, "components.empty"},
		{"duplicate component name", func(m *Manifest) { m.Components[1].Name = "cap-go" }, "components.name.duplicate"},
		{"unknown kind", func(m *Manifest) { m.Components[0].Kind = "library" }, "components.kind"},
		{"unknown tier", func(m *Manifest) { m.Components[0].Tier = "gold" }, "components.tier"},
		{"stable missing coordinates", func(m *Manifest) { m.Components[0].Package = "" }, "components.stable.coordinates"},
		{"stable missing evidence", func(m *Manifest) { m.Components[0].Evidence = "" }, "components.stable.evidence"},
		{"guard classed as sdk", func(m *Manifest) { m.Components[1].Kind = "sdk" }, "components.guard.kind"},
		{"duplicate transport", func(m *Manifest) { m.Transports[1].Name = "nats" }, "transports.name.duplicate"},
		{"unknown transport state", func(m *Manifest) { m.Transports[0].State = "beta" }, "transports.state"},
		{"supported without evidence", func(m *Manifest) { m.Transports[0].Evidence = "" }, "transports.supported.evidence"},
		{"kafka promoted no evidence", func(m *Manifest) { m.Transports[1].State = "supported" }, "transports.supported.evidence"},
		{"go toolchain missing", func(m *Manifest) { m.Toolchains.Go.Tested = "" }, "toolchains.go"},
		{"node toolchain missing", func(m *Manifest) { m.Toolchains.Node.Minimum = "" }, "toolchains.node"},
		{"python toolchain missing", func(m *Manifest) { m.Toolchains.Python.Tested = "" }, "toolchains.python"},
		{"no security lines", func(m *Manifest) { m.Security.SupportedLines = nil }, "security.supportedLines"},
		{"no reporting route", func(m *Manifest) { m.Security.ReportingRoute = "" }, "security.reportingRoute"},
		{"duplicate link", func(m *Manifest) { m.Links = append(m.Links, Link{Name: "spec-index", URL: "x.md"}) }, "links.name.duplicate"},
		{"unsafe link url", func(m *Manifest) { m.Links[0].URL = "../../secret" }, "links.url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.mutate(m)
			ps := Validate(m)
			if !hasField(ps, tc.field) {
				t.Fatalf("mutation %q: expected problem field %q, got %v", tc.name, tc.field, ps)
			}
		})
	}
}
