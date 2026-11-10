package publicreport

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cordum-io/cap/v2/internal/tck"
)

var requiredIDs = []string{
	"cancellation/worker-honors-cancel",
	"lifecycle/worker-running-to-failed",
	"lifecycle/worker-running-to-succeeded",
	"malformed/truncated-wire",
}

func testSubject() Subject {
	return Subject{
		TestedCommit:           "0123456789abcdef0123456789abcdef01234567",
		ArtifactName:           "cap-sdk-go",
		ArtifactSha256:         strings.Repeat("0123456789abcdef", 4),
		ScenarioManifestSha256: strings.Repeat("fedcba9876543210", 4),
		RequiredCaseIDs:        append([]string(nil), requiredIDs...),
	}
}

// readTestdata folds a Windows core.autocrlf checkout back to LF. Canonical
// bytes never contain a raw CR, so this cannot mask a real difference.
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

func source(t *testing.T, name string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(readTestdata(t, "sources.json"), &m); err != nil {
		t.Fatalf("sources.json: %v", err)
	}
	raw, ok := m[name]
	if !ok {
		t.Fatalf("sources.json has no %q", name)
	}
	return raw
}

func mustProject(t *testing.T, src []byte, s Subject) (Report, []byte) {
	t.Helper()
	r, b, err := Project(src, s)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	return r, b
}

// mutateJSON decodes b generically, applies f and re-encodes it.
func mutateJSON(t *testing.T, b []byte, f func(m map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("mutateJSON: %v", err)
	}
	f(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("mutateJSON: %v", err)
	}
	return out
}

func casesOf(m map[string]any) []any                { return m["cases"].([]any) }
func caseAt(m map[string]any, i int) map[string]any { return casesOf(m)[i].(map[string]any) }
func summaryOf(m map[string]any) map[string]any     { return m["summary"].(map[string]any) }

func assertNoReport(t *testing.T, r Report, b []byte) {
	t.Helper()
	if !reflect.ValueOf(r).IsZero() || b != nil {
		t.Fatalf("error returned a partial report: %+v %q", r, b)
	}
}

func TestProjectGoldens(t *testing.T) {
	for _, name := range []string{"pass", "fail", "incomplete"} {
		t.Run(name, func(t *testing.T) {
			_, got := mustProject(t, source(t, name), testSubject())
			if want := readTestdata(t, name+".golden.json"); !bytes.Equal(got, want) {
				t.Fatalf("projection differs from golden\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// Each canary is planted in the canary source's adapter, sdk, detail or
// evidence fields.
var canaries = []string{
	"canary",                       // marker shared by every planted value below but the timestamp
	"canary-host.corp.example",     // hostname
	"/home/canary-user",            // POSIX absolute path
	`C:\Users\canary-user`,         // Windows absolute path
	"canary-user",                  // username / principal
	"CAP_SECRET=canary-env-value",  // env assignment
	"Bearer canary-token-9f8e7d6c", // bearer token
	"tenant-canary-42",             // tenant id
	"2026-11-10T19:03:17",          // RFC3339 timestamp
	"/usr/bin/python3",             // argv
	"cap-sdk-python",               // sdk string
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func TestProjectStripsPrivateText(t *testing.T) {
	src := source(t, "canary")
	for _, c := range canaries {
		if !bytes.Contains(src, []byte(jsonEscape(c))) {
			t.Fatalf("fixture does not carry canary %q", c)
		}
	}
	_, got := mustProject(t, src, testSubject())
	lower := bytes.ToLower(got)
	for _, c := range canaries {
		for _, form := range []string{c, jsonEscape(c)} {
			if bytes.Contains(lower, []byte(strings.ToLower(form))) {
				t.Errorf("projection leaks canary %q", form)
			}
		}
	}
	if want := readTestdata(t, "pass.golden.json"); !bytes.Equal(got, want) {
		t.Fatalf("canary projection must equal the pass golden\n got: %s", got)
	}
}

func TestProjectRejects(t *testing.T) {
	pass := source(t, "pass")
	src := func(f func(m map[string]any)) []byte { return mutateJSON(t, pass, f) }
	subj := func(f func(s *Subject)) Subject { s := testSubject(); f(&s); return s }
	ok := testSubject()
	rows := []struct {
		name string
		src  []byte
		subj Subject
		want error
	}{
		{"truncated json", pass[:len(pass)/2], ok, ErrMalformedJSON},
		{"trailing value", append(append([]byte{}, pass...), "{}"...), ok, ErrMalformedJSON},
		{"wrong json type", src(func(m map[string]any) { m["summary"] = "x" }), ok, ErrMalformedJSON},
		{"unknown top-level field", src(func(m map[string]any) { m["hostname"] = "h" }), ok, ErrUnknownField},
		{"unknown summary field", src(func(m map[string]any) { summaryOf(m)["skipped"] = 0 }), ok, ErrUnknownField},
		{"unknown case field", src(func(m map[string]any) { caseAt(m, 1)["stderr"] = "x" }), ok, ErrUnknownField},
		{"duplicate source case", src(func(m map[string]any) {
			m["cases"] = append(casesOf(m), caseAt(m, 1))
			summaryOf(m)["total"], summaryOf(m)["pass"] = 6, 5
		}), ok, ErrDuplicateCase},
		{"duplicate required id", pass, subj(func(s *Subject) { s.RequiredCaseIDs = append(s.RequiredCaseIDs, requiredIDs[0]) }), ErrDuplicateCase},
		{"missing required case", pass, subj(func(s *Subject) { s.RequiredCaseIDs = append(s.RequiredCaseIDs, "safety/deny-blocks-dispatch") }), ErrMissingRequiredCase},
		{"empty required ids", pass, subj(func(s *Subject) { s.RequiredCaseIDs = nil }), ErrNoRequiredCases},
		{"uppercase commit", pass, subj(func(s *Subject) { s.TestedCommit = strings.ToUpper(s.TestedCommit) }), ErrBadCommit},
		{"short commit", pass, subj(func(s *Subject) { s.TestedCommit = s.TestedCommit[:39] }), ErrBadCommit},
		{"long artifact digest", pass, subj(func(s *Subject) { s.ArtifactSha256 += "0" }), ErrBadDigest},
		{"uppercase manifest digest", pass, subj(func(s *Subject) { s.ScenarioManifestSha256 = strings.ToUpper(s.ScenarioManifestSha256) }), ErrBadDigest},
		{"artifact name with space", pass, subj(func(s *Subject) { s.ArtifactName = "cap sdk" }), ErrBadArtifactName},
		{"artifact name path", pass, subj(func(s *Subject) { s.ArtifactName = "../cap" }), ErrBadArtifactName},
		{"empty artifact name", pass, subj(func(s *Subject) { s.ArtifactName = "" }), ErrBadArtifactName},
		{"bad required case id", pass, subj(func(s *Subject) { s.RequiredCaseIDs[0] = "Lifecycle/Worker" }), ErrBadCaseID},
		{"bad source case id", src(func(m map[string]any) { caseAt(m, 0)["id"] = "/home/alice" }), ok, ErrBadCaseID},
		{"bad source status", src(func(m map[string]any) { caseAt(m, 1)["status"] = "SKIPPED" }), ok, ErrBadStatus},
		{"empty role", src(func(m map[string]any) { m["role"] = "" }), ok, ErrBadToken},
		{"unknown role", src(func(m map[string]any) { m["role"] = "admin" }), ok, ErrBadToken},
		{"bad profile", src(func(m map[string]any) { m["profile"] = "Core" }), ok, ErrBadToken},
		{"bad transport", src(func(m map[string]any) { m["transport"] = "nats tcp" }), ok, ErrBadToken},
		{"path-like tool version", src(func(m map[string]any) { m["toolVersion"] = "cap-tck/home/alice" }), ok, ErrBadToken},
		{"tool version with space", src(func(m map[string]any) { m["toolVersion"] = "cap tck" }), ok, ErrBadToken},
		{"summary count mismatch", src(func(m map[string]any) { summaryOf(m)["pass"] = 3 }), ok, ErrInconsistentSource},
		{"summary total mismatch", src(func(m map[string]any) { summaryOf(m)["total"] = 6 }), ok, ErrInconsistentSource},
		{"summary conformant flipped", src(func(m map[string]any) { summaryOf(m)["conformant"] = false }), ok, ErrInconsistentSource},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			r, b, err := Project(row.src, row.subj)
			if !errors.Is(err, row.want) {
				t.Fatalf("err = %v, want %v", err, row.want)
			}
			assertNoReport(t, r, b)
		})
	}
}

// pub re-encodes a golden after f with encoding/json directly, so the rows do
// not depend on the canonical encoder under test.
func pub(t *testing.T, golden string, f func(r *Report)) []byte {
	t.Helper()
	var r Report
	if err := json.Unmarshal(readTestdata(t, golden+".golden.json"), &r); err != nil {
		t.Fatalf("pub: %v", err)
	}
	f(&r)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("pub: %v", err)
	}
	return append(b, '\n')
}

func TestValidateRejects(t *testing.T) {
	golden := readTestdata(t, "pass.golden.json")
	edit := func(old, repl string) []byte {
		if !bytes.Contains(golden, []byte(old)) {
			t.Fatalf("golden lacks %q", old)
		}
		return bytes.Replace(golden, []byte(old), []byte(repl), 1)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, bytes.TrimSpace(golden), "", "  "); err != nil {
		t.Fatal(err)
	}
	indented.WriteByte('\n')
	reordered := append(mutateJSON(t, golden, func(map[string]any) {}), '\n')
	trimmed := golden[:len(golden)-1]
	rows := []struct {
		name string
		b    []byte
		want error
	}{
		{"unknown schemaVersion", pub(t, "pass", func(r *Report) { r.SchemaVersion = "cap-public-report/v2" }), ErrSchemaVersion},
		{"empty schemaVersion", pub(t, "pass", func(r *Report) { r.SchemaVersion = "" }), ErrSchemaVersion},
		{"truncated json", golden[:len(golden)/2], ErrMalformedJSON},
		{"trailing value", append(append([]byte{}, golden...), "{}\n"...), ErrMalformedJSON},
		{"unknown top-level field", edit(`{"schemaVersion"`, `{"detail":"x","schemaVersion"`), ErrUnknownField},
		{"unknown implementation field", edit(`"artifactName"`, `"path":"/home/alice","artifactName"`), ErrUnknownField},
		{"unknown case field", edit(`"status":"PASS"}`, `"status":"PASS","detail":"x"}`), ErrUnknownField},
		{"duplicate case id", pub(t, "pass", func(r *Report) { r.Cases = append(r.Cases[:2:2], r.Cases[1:]...) }), ErrDuplicateCase},
		{"unsorted cases", pub(t, "pass", func(r *Report) { r.Cases[1], r.Cases[2] = r.Cases[2], r.Cases[1] }), ErrUnsortedCases},
		{"bad commit", pub(t, "pass", func(r *Report) { r.TestedCommit = "HEAD" }), ErrBadCommit},
		{"bad artifact digest", pub(t, "pass", func(r *Report) { r.Implementation.ArtifactSha256 = "sha256:abc" }), ErrBadDigest},
		{"bad manifest digest", pub(t, "pass", func(r *Report) { r.ScenarioManifestSha256 = "" }), ErrBadDigest},
		{"bad artifact name", pub(t, "pass", func(r *Report) { r.Implementation.ArtifactName = "/opt/cap" }), ErrBadArtifactName},
		{"bad case id", pub(t, "pass", func(r *Report) { r.Cases[0].ID = "no-slash" }), ErrBadCaseID},
		{"bad status", pub(t, "pass", func(r *Report) { r.Cases[1].Status = "SKIPPED" }), ErrBadStatus},
		{"bad role", pub(t, "pass", func(r *Report) { r.Role = "" }), ErrBadToken},
		{"bad profile", pub(t, "pass", func(r *Report) { r.Profile = "core profile" }), ErrBadToken},
		{"bad transport", pub(t, "pass", func(r *Report) { r.Transport = "NATS" }), ErrBadToken},
		{"bad tool version", pub(t, "pass", func(r *Report) { r.TCKToolVersion = "/cap-tck" }), ErrBadToken},
		{"no cases", pub(t, "pass", func(r *Report) { r.Cases = []Case{} }), ErrNoRequiredCases},
		{"no required cases", pub(t, "pass", func(r *Report) {
			for i := range r.Cases {
				r.Cases[i].Required = false
			}
		}), ErrNoRequiredCases},
		{"FAIL relabelled PASS", pub(t, "fail", func(r *Report) { r.Verdict = VerdictPass }), ErrVerdictMismatch},
		{"INCOMPLETE relabelled PASS", pub(t, "incomplete", func(r *Report) { r.Verdict = VerdictPass }), ErrVerdictMismatch},
		{"unknown verdict", pub(t, "pass", func(r *Report) { r.Verdict = "MAYBE" }), ErrVerdictMismatch},
		{"indented", indented.Bytes(), ErrNotCanonical},
		{"reordered keys", reordered, ErrNotCanonical},
		{"missing trailing newline", trimmed, ErrNotCanonical},
		{"CRLF line ending", append(append([]byte{}, trimmed...), "\r\n"...), ErrNotCanonical},
		{"omitted false flag", edit(`"required":false,`, ``), ErrNotCanonical},
		{"case-folded key", edit(`"verdict"`, `"Verdict"`), ErrNotCanonical},
		{"duplicate key", edit(`"verdict":"PASS"`, `"verdict":"FAIL","verdict":"PASS"`), ErrNotCanonical},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			r, err := Validate(row.b)
			if !errors.Is(err, row.want) {
				t.Fatalf("err = %v, want %v", err, row.want)
			}
			assertNoReport(t, r, nil)
		})
	}
}

func TestVerdict(t *testing.T) {
	c := func(req bool, st string) Case { return Case{ID: "a/b", Required: req, Status: st} }
	rows := []struct {
		name  string
		cases []Case
		want  string
	}{
		{"required pass", []Case{c(true, "PASS")}, VerdictPass},
		{"required fail", []Case{c(true, "FAIL")}, VerdictFail},
		{"required error", []Case{c(true, "ERROR")}, VerdictFail},
		{"optional error still fails", []Case{c(true, "PASS"), c(false, "ERROR")}, VerdictFail},
		{"optional fail still fails", []Case{c(true, "PASS"), c(false, "FAIL")}, VerdictFail},
		{"fail beats incomplete", []Case{c(true, "UNSUPPORTED"), c(true, "FAIL")}, VerdictFail},
		{"required unsupported", []Case{c(true, "PASS"), c(true, "UNSUPPORTED")}, VerdictIncomplete},
		{"required n/a", []Case{c(true, "N/A")}, VerdictIncomplete},
		{"optional non-outcomes", []Case{c(true, "PASS"), c(false, "N/A"), c(false, "UNSUPPORTED")}, VerdictPass},
	}
	for _, row := range rows {
		if got := verdict(row.cases); got != row.want {
			t.Errorf("%s: verdict = %q, want %q", row.name, got, row.want)
		}
	}
}

func findCase(t *testing.T, r Report, id string) Case {
	t.Helper()
	for _, c := range r.Cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no case %q", id)
	return Case{}
}

func TestRequiredSetIsUnion(t *testing.T) {
	t.Run("subject cannot drop source-required cases", func(t *testing.T) {
		s := testSubject()
		s.RequiredCaseIDs = []string{"lifecycle/worker-running-to-succeeded", "malformed/truncated-wire"}
		r, b := mustProject(t, source(t, "incomplete"), s)
		if r.Verdict != VerdictIncomplete || !findCase(t, r, "cancellation/worker-honors-cancel").Required {
			t.Fatalf("narrowed subject changed the result: %s", b)
		}
		if want := readTestdata(t, "incomplete.golden.json"); !bytes.Equal(b, want) {
			t.Fatalf("narrowed subject changed the bytes\n got: %s\nwant: %s", b, want)
		}
	})
	t.Run("subject can add a requirement", func(t *testing.T) {
		s := testSubject()
		s.RequiredCaseIDs = append(s.RequiredCaseIDs, "cancellation/cp-cancel-from-pending")
		r, b := mustProject(t, source(t, "pass"), s)
		if r.Verdict != VerdictIncomplete || !findCase(t, r, "cancellation/cp-cancel-from-pending").Required {
			t.Fatalf("added requirement ignored: %s", b)
		}
	})
}

func TestProjectDeterministic(t *testing.T) {
	for _, name := range []string{"pass", "fail", "incomplete"} {
		raw, want := source(t, name), readTestdata(t, name+".golden.json")
		n := 0
		mutateJSON(t, raw, func(m map[string]any) { n = len(casesOf(m)) })
		reorder := func(perm func(cs []any) []any) []byte {
			return mutateJSON(t, raw, func(m map[string]any) { m["cases"] = perm(casesOf(m)) })
		}
		var inputs [][]byte
		inputs = append(inputs, raw, raw, raw, reorder(func(cs []any) []any {
			out := make([]any, 0, len(cs))
			for i := len(cs) - 1; i >= 0; i-- {
				out = append(out, cs[i])
			}
			return out
		}))
		for k := 1; k < n; k++ {
			inputs = append(inputs, reorder(func(cs []any) []any { return append(cs[k:len(cs):len(cs)], cs[:k]...) }))
		}
		for i, in := range inputs {
			if _, got := mustProject(t, in, testSubject()); !bytes.Equal(got, want) {
				t.Fatalf("%s input %d: bytes differ\n got: %s\nwant: %s", name, i, got, want)
			}
		}
	}
}

func TestValidateRoundTrip(t *testing.T) {
	for _, name := range []string{"pass", "fail", "incomplete"} {
		r, b := mustProject(t, source(t, name), testSubject())
		got, err := Validate(b)
		if err != nil || !reflect.DeepEqual(got, r) {
			t.Fatalf("%s: Validate(Project) = %+v, %v; want %+v", name, got, err, r)
		}
		if _, err := Validate(readTestdata(t, name+".golden.json")); err != nil {
			t.Fatalf("%s golden does not validate: %v", name, err)
		}
	}
}

// TestProjectsRealTCKReport feeds the exact bytes tck.Report.JSON emits.
func TestProjectsRealTCKReport(t *testing.T) {
	cases := []tck.CaseResult{
		{ID: "lifecycle/worker-running-to-succeeded", Profile: "core", Role: tck.RoleWorker, Required: true, Applicable: true,
			Status: tck.StatusPass, Detail: "ok", DurationMs: 3, Evidence: map[string]string{"k": "v"}},
		{ID: "cancellation/cp-cancel-from-pending", Profile: "core", Role: tck.RoleControlPlane, Required: true, Status: tck.StatusNA},
	}
	hs := tck.AdapterMessage{SDK: "go", Role: tck.RoleWorker, Transport: "nats"}
	src, err := tck.NewReport("go-ref", hs, "core", tck.HarnessVersion, cases).JSON()
	if err != nil {
		t.Fatal(err)
	}
	s := testSubject()
	s.RequiredCaseIDs = []string{"lifecycle/worker-running-to-succeeded"}
	r, _ := mustProject(t, src, s)
	if r.TCKToolVersion != tck.HarnessVersion || r.Verdict != VerdictPass || len(r.Cases) != 2 {
		t.Fatalf("unexpected projection of a real report: %+v", r)
	}
}

// fieldShape lists each field's json tag and kind, in declaration order.
func fieldShape(typ reflect.Type) []string {
	var out []string
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		out = append(out, f.Tag.Get("json")+":"+f.Type.Kind().String())
	}
	return out
}

func TestMirrorMatchesTCKReport(t *testing.T) {
	for _, p := range []struct{ tck, mirror reflect.Type }{
		{reflect.TypeOf(tck.Report{}), reflect.TypeOf(sourceReport{})},
		{reflect.TypeOf(tck.Summary{}), reflect.TypeOf(sourceSummary{})},
		{reflect.TypeOf(tck.CaseResult{}), reflect.TypeOf(sourceCase{})},
	} {
		if got, want := fieldShape(p.mirror), fieldShape(p.tck); !reflect.DeepEqual(got, want) {
			t.Errorf("%s drifted from %s\n got: %v\nwant: %v", p.mirror, p.tck, got, want)
		}
	}
}

func loadPublicSchema(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "spec", "tck", "schema", "public-report-v1.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	return m
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func jsonNames(typ reflect.Type) []string {
	var out []string
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func TestSchemaMatchesGoTypes(t *testing.T) {
	schema := loadPublicSchema(t)
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["$id"] == nil || schema["title"] == nil {
		t.Fatal("schema lacks draft 2020-12 $schema, $id or title")
	}
	defs := schema["$defs"].(map[string]any)
	for _, o := range []struct {
		name string
		obj  map[string]any
		typ  reflect.Type
	}{
		{"report", schema, reflect.TypeOf(Report{})},
		{"implementation", defs["implementation"].(map[string]any), reflect.TypeOf(Implementation{})},
		{"case", defs["case"].(map[string]any), reflect.TypeOf(Case{})},
	} {
		props := sortedKeys(o.obj["properties"].(map[string]any))
		if want := jsonNames(o.typ); !reflect.DeepEqual(props, want) {
			t.Errorf("%s: schema properties %v, Go json names %v", o.name, props, want)
		}
		var req []string
		for _, r := range o.obj["required"].([]any) {
			req = append(req, r.(string))
		}
		sort.Strings(req)
		if !reflect.DeepEqual(req, props) || o.obj["additionalProperties"] != false {
			t.Errorf("%s: every property must be required and additionalProperties false", o.name)
		}
	}
}

func TestSchemaPatternsMatchGo(t *testing.T) {
	schema := loadPublicSchema(t)
	defs := schema["$defs"].(map[string]any)
	top := schema["properties"].(map[string]any)
	impl := defs["implementation"].(map[string]any)["properties"].(map[string]any)
	cs := defs["case"].(map[string]any)["properties"].(map[string]any)
	field := func(m map[string]any, k, attr string) any { return m[k].(map[string]any)[attr] }
	for _, p := range []struct {
		name string
		m    map[string]any
		re   *regexp.Regexp
	}{
		{"tckToolVersion", top, reToolVersion}, {"profile", top, reToken}, {"transport", top, reToken},
		{"testedCommit", top, reCommit}, {"scenarioManifestSha256", top, reDigest},
		{"artifactName", impl, reArtifactName}, {"artifactSha256", impl, reDigest}, {"id", cs, reCaseID},
	} {
		if got := field(p.m, p.name, "pattern"); got != p.re.String() {
			t.Errorf("%s: schema pattern %v, Go %s", p.name, got, p.re)
		}
	}
	for _, e := range []struct {
		name string
		m    map[string]any
		want []string
	}{{"role", top, roles}, {"verdict", top, verdicts}, {"status", cs, statuses}} {
		var got []string
		for _, v := range field(e.m, e.name, "enum").([]any) {
			got = append(got, v.(string))
		}
		if !reflect.DeepEqual(got, e.want) {
			t.Errorf("%s: schema enum %v, Go %v", e.name, got, e.want)
		}
	}
	if field(top, "schemaVersion", "const") != SchemaVersion {
		t.Errorf("schemaVersion const differs from %q", SchemaVersion)
	}
}
