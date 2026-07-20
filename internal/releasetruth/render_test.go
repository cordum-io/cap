package releasetruth

import (
	"strings"
	"testing"
)

// lineWith returns the first line of s that contains sub, or "" if none.
func lineWith(s, sub string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.Contains(ln, sub) {
			return ln
		}
	}
	return ""
}

func twoBlockDoc() string {
	return strings.Join([]string{
		"# Title",                               // 1
		"",                                      // 2
		"<!-- cap-release:begin:spec-count -->", // 3
		"OLD-COUNT",                             // 4
		"<!-- cap-release:end -->",              // 5
		"",                                      // 6
		"prose between",                         // 7
		"<!-- cap-release:begin:transport-table -->", // 8
		"OLD-TABLE",                // 9
		"<!-- cap-release:end -->", // 10
		"tail",                     // 11
	}, "\n")
}

func TestFindBlocks_DiscoversInOrderWithLines(t *testing.T) {
	blocks, err := FindBlocks(twoBlockDoc())
	if err != nil {
		t.Fatalf("FindBlocks unexpected error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("FindBlocks found %d blocks, want 2: %+v", len(blocks), blocks)
	}
	if blocks[0].ID != "spec-count" || blocks[0].BeginLine != 3 || blocks[0].EndLine != 5 || blocks[0].Inner != "OLD-COUNT" {
		t.Errorf("block[0] = %+v, want {spec-count 3 5 OLD-COUNT}", blocks[0])
	}
	if blocks[1].ID != "transport-table" || blocks[1].BeginLine != 8 || blocks[1].EndLine != 10 || blocks[1].Inner != "OLD-TABLE" {
		t.Errorf("block[1] = %+v, want {transport-table 8 10 OLD-TABLE}", blocks[1])
	}
}

func TestFindBlocks_RejectsMalformed(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"begin without end", "<!-- cap-release:begin:x -->\ninner"},
		{"end without begin", "prose\n<!-- cap-release:end -->"},
		{"nested begin", "<!-- cap-release:begin:x -->\n<!-- cap-release:begin:y -->\n<!-- cap-release:end -->"},
		{"duplicate id", "<!-- cap-release:begin:x -->\n<!-- cap-release:end -->\n<!-- cap-release:begin:x -->\n<!-- cap-release:end -->"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := FindBlocks(tc.content); err == nil {
				t.Fatalf("FindBlocks(%q) = nil error, want error", tc.name)
			}
		})
	}
}

func TestRenderBlock_SpecCountIsDerived(t *testing.T) {
	m := validManifest() // baseline has exactly 2 specs
	got, err := RenderBlock(m, "spec-count")
	if err != nil {
		t.Fatalf("RenderBlock(spec-count) error: %v", err)
	}
	if got != "2" {
		t.Errorf("RenderBlock(spec-count) = %q, want %q", got, "2")
	}
}

func TestRenderBlock_TransportTableMarksStatesCorrectly(t *testing.T) {
	m := validManifest() // nats supported, kafka experimental
	got, err := RenderBlock(m, "transport-table")
	if err != nil {
		t.Fatalf("RenderBlock(transport-table) error: %v", err)
	}
	natsLine := lineWith(got, "nats")
	if !strings.Contains(natsLine, "supported") {
		t.Errorf("nats row = %q, want it to contain 'supported'", natsLine)
	}
	kafkaLine := lineWith(got, "kafka")
	if !strings.Contains(kafkaLine, "experimental") {
		t.Errorf("kafka row = %q, want it to contain 'experimental'", kafkaLine)
	}
	if strings.Contains(kafkaLine, "supported") {
		t.Errorf("kafka row = %q wrongly marks kafka supported", kafkaLine)
	}
}

func TestRenderBlock_UnknownIDErrorsNamingID(t *testing.T) {
	_, err := RenderBlock(validManifest(), "no-such-block")
	if err == nil {
		t.Fatal("RenderBlock(no-such-block) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "no-such-block") {
		t.Errorf("error %q does not name the offending id", err.Error())
	}
}

func TestRender_ReplacesInnerPreservingProse(t *testing.T) {
	m := validManifest()
	in := strings.Join([]string{
		"intro",
		"<!-- cap-release:begin:spec-count -->",
		"OLD",
		"<!-- cap-release:end -->",
		"outro",
	}, "\n")
	want := strings.Join([]string{
		"intro",
		"<!-- cap-release:begin:spec-count -->",
		"2",
		"<!-- cap-release:end -->",
		"outro",
	}, "\n")
	got, err := Render(m, in)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if got != want {
		t.Errorf("Render() =\n%q\nwant\n%q", got, want)
	}
}

func TestRender_IsIdempotent(t *testing.T) {
	m := validManifest()
	once, err := Render(m, twoBlockDoc())
	if err != nil {
		t.Fatalf("Render first pass: %v", err)
	}
	if !strings.Contains(once, "\n2\n") {
		t.Fatalf("first render did not derive spec-count into the block: %q", once)
	}
	twice, err := Render(m, once)
	if err != nil {
		t.Fatalf("Render second pass: %v", err)
	}
	if once != twice {
		t.Errorf("Render not idempotent:\nfirst=%q\nsecond=%q", once, twice)
	}
}

func TestRender_DetectsOneByteDrift(t *testing.T) {
	m := validManifest()
	clean, err := Render(m, twoBlockDoc())
	if err != nil {
		t.Fatalf("Render clean: %v", err)
	}
	// Flip the derived spec-count from 2 to 3 (a one-byte drift inside a block).
	drift := strings.Replace(clean, "\n2\n", "\n3\n", 1)
	if drift == clean {
		t.Fatal("test setup failed: drift string equals clean")
	}
	restored, err := Render(m, drift)
	if err != nil {
		t.Fatalf("Render drift: %v", err)
	}
	if restored == drift {
		t.Error("Render did not detect one-byte drift (check mode would pass on a drifted file)")
	}
	if restored != clean {
		t.Errorf("Render did not restore to canonical form:\ngot=%q\nwant=%q", restored, clean)
	}
}

func TestRender_EmitsLFOnly(t *testing.T) {
	m := validManifest()
	crlfIn := strings.ReplaceAll(twoBlockDoc(), "\n", "\r\n")
	got, err := Render(m, crlfIn)
	if err != nil {
		t.Fatalf("Render CRLF input: %v", err)
	}
	if !strings.Contains(got, "\n2\n") {
		t.Fatalf("render did not process the CRLF input into canonical form: %q", got)
	}
	if strings.Contains(got, "\r") {
		t.Errorf("Render output contains CR bytes; want LF-only newlines")
	}
}

func TestRender_FailsClosedOnUnknownBlockID(t *testing.T) {
	m := validManifest()
	in := "<!-- cap-release:begin:bogus-id -->\nx\n<!-- cap-release:end -->"
	if _, err := Render(m, in); err == nil {
		t.Fatal("Render(unknown block id) = nil error, want fail-closed error")
	}
}

func TestCheckDrift_IgnoresNewlineStyleButCatchesContent(t *testing.T) {
	m := validManifest()
	clean, err := Render(m, twoBlockDoc())
	if err != nil {
		t.Fatalf("Render clean: %v", err)
	}
	// A rendered doc is not drifted.
	if _, drifted, err := CheckDrift(m, clean); err != nil || drifted {
		t.Errorf("CheckDrift(clean) = drifted %v err %v, want false/nil", drifted, err)
	}
	// The same doc with CRLF newlines is still not drifted (newline style ignored).
	crlf := strings.ReplaceAll(clean, "\n", "\r\n")
	if _, drifted, _ := CheckDrift(m, crlf); drifted {
		t.Errorf("CheckDrift(CRLF of clean) = drifted true, want false (newline style must be ignored)")
	}
	// A one-byte content change inside a block is drift.
	stale := strings.Replace(clean, "\n2\n", "\n7\n", 1)
	if _, drifted, _ := CheckDrift(m, stale); !drifted {
		t.Errorf("CheckDrift(stale) = drifted false, want true")
	}
}

func TestRenderBlock_GoldenSpecCountIsTwenty(t *testing.T) {
	m, _ := loadGolden(t)
	got, err := RenderBlock(m, "spec-count")
	if err != nil {
		t.Fatalf("RenderBlock(spec-count) error: %v", err)
	}
	if got != "20" {
		t.Errorf("golden spec-count = %q, want 20", got)
	}
}
