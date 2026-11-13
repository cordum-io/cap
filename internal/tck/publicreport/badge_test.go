package publicreport

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

func badgeReport(t *testing.T, change func(*Report)) []byte {
	t.Helper()
	r, err := Validate(readTestdata(t, "pass.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	change(&r)
	b, err := canonical(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBadgeGoldenB1(t *testing.T) {
	got, err := Badge(readTestdata(t, "pass.golden.json"))
	if err != nil || !bytes.Equal(got, readTestdata(t, "badge-pass.golden.svg")) {
		t.Fatalf("Badge = %s, %v", got, err)
	}
}

func TestBadgeProductionB2(t *testing.T) {
	b := badgeReport(t, func(r *Report) { r.Profile = "cap-production" })
	got, err := Badge(b)
	golden := readTestdata(t, "badge-pass.golden.svg")
	want := bytes.ReplaceAll(golden, []byte("cap-tck core"), []byte("cap-tck cap-production"))
	if err != nil || !bytes.Equal(got, want) || bytes.Equal(got, golden) {
		t.Fatalf("Badge = %s, %v", got, err)
	}
}

func TestBadgeRefusalsB3(t *testing.T) {
	cases := invalidSigningInputs(t)
	cases = append(cases, struct {
		name string
		data []byte
		want error
	}{"malformed", []byte("{"), ErrMalformedJSON})
	for _, name := range []string{"fail", "incomplete"} {
		cases = append(cases, struct {
			name string
			data []byte
			want error
		}{name, readTestdata(t, name+".golden.json"), ErrBadgeVerdict})
	}
	for _, profile := range []string{"zz-unlisted", "script"} {
		b := badgeReport(t, func(r *Report) { r.Profile = profile })
		if _, err := Validate(b); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, struct {
			name string
			data []byte
			want error
		}{profile, b, ErrBadgeProfile})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Badge(tc.data)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("Badge = %s, %v; want nil, %v", got, err, tc.want)
			}
		})
	}
}

func TestBadgeHostileStringsB4(t *testing.T) {
	fields := []string{"profile", "transport", "tckToolVersion", "role", "testedCommit", "artifactName", "artifactSha256", "scenarioManifestSha256", "id", "status", "verdict"}
	values := []string{`<script>alert(1)</script>`, `"><svg onload=alert(1)>`, "javascript:alert(1)", "]]><x>", "&amp;", "%[1]s", "{{.}}", "\u202e", "../../etc/passwd", "certified"}
	for _, field := range fields {
		for i, value := range values {
			t.Run(fmt.Sprintf("%s/%d", field, i), func(t *testing.T) {
				b := mutateJSON(t, readTestdata(t, "pass.golden.json"), func(m map[string]any) {
					switch field {
					case "artifactName", "artifactSha256":
						m["implementation"].(map[string]any)[field] = value
					case "id", "status":
						caseAt(m, 0)[field] = value
					default:
						m[field] = value
					}
				})
				got, err := Badge(b)
				if got != nil || err == nil {
					t.Fatalf("hostile input produced SVG: %s, %v", got, err)
				}
			})
		}
	}
}

func TestBadgeCanariesB5(t *testing.T) {
	b := badgeReport(t, func(r *Report) {
		r.Transport, r.TCKToolVersion = "zzcanarytransport", "zzcanary/9"
		r.Implementation.ArtifactName = "zzcanary-artifact"
		r.TestedCommit = "c0ffee" + strings.Repeat("0", 34)
		r.Implementation.ArtifactSha256 = strings.Repeat("deadbeef", 8)
		r.ScenarioManifestSha256 = strings.Repeat("badc0ffe", 8)
		r.Cases = append(r.Cases, Case{ID: "zzcanary/case-a", Status: statusPass})
	})
	if _, err := Validate(b); err != nil {
		t.Fatal(err)
	}
	got, err := Badge(b)
	if err != nil || !bytes.Equal(got, readTestdata(t, "badge-pass.golden.svg")) {
		t.Fatalf("Badge = %s, %v", got, err)
	}
	for _, canary := range []string{"zzcanary", "c0ffee", "deadbeef", "badc0ffe"} {
		if bytes.Contains(got, []byte(canary)) {
			t.Fatalf("SVG leaks %s", canary)
		}
	}
}

func assertInertSVG(t *testing.T, b []byte) {
	t.Helper()
	allowed := map[string]bool{"svg": true, "title": true, "rect": true, "g": true, "text": true}
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if el, ok := token.(xml.StartElement); ok {
			if !allowed[el.Name.Local] {
				t.Fatalf("unexpected element: %s", el.Name.Local)
			}
			for _, a := range el.Attr {
				if strings.HasPrefix(a.Name.Local, "on") || a.Name.Local == "href" || strings.Contains(a.Name.Space, "xlink") {
					t.Fatalf("unsafe attribute: %+v", a)
				}
			}
		}
	}
	for _, bad := range []string{"url(", "<script", "@import", "javascript:"} {
		if bytes.Contains(bytes.ToLower(b), []byte(bad)) {
			t.Fatalf("unsafe SVG: %s", bad)
		}
	}
}

func TestBadgeInertnessB6(t *testing.T) {
	production, err := Badge(badgeReport(t, func(r *Report) { r.Profile = "cap-production" }))
	if err != nil {
		t.Fatal(err)
	}
	golden := readTestdata(t, "badge-pass.golden.svg")
	assertInertSVG(t, golden)
	assertInertSVG(t, production)
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	banned := regexp.MustCompile(`(?i)certified|compliant|official`)
	for _, b := range [][]byte{golden, production, []byte(badgeTemplate), readme} {
		if banned.Match(b) {
			t.Fatal("disallowed wording")
		}
	}
}
