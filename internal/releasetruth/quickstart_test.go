package releasetruth

import (
	"strings"
	"testing"
)

func snippetDoc() string {
	return strings.Join([]string{
		"# Quickstart",
		"<!-- cap-release:snippet:go-echo:go -->",
		"```go",
		"package main",
		"import capgo \"github.com/cordum-io/cap/v2/sdk/go\"",
		"func main() { _ = capgo.Version }",
		"```",
		"<!-- cap-release:snippet-end -->",
		"prose",
		"<!-- cap-release:snippet:py-echo:python -->",
		"```python",
		"import cap_sdk_python",
		"```",
		"<!-- cap-release:snippet-end -->",
	}, "\n")
}

func TestExtractSnippets_ReturnsEachLanguage(t *testing.T) {
	snips, err := ExtractSnippets(snippetDoc())
	if err != nil {
		t.Fatalf("ExtractSnippets error: %v", err)
	}
	if len(snips) != 2 {
		t.Fatalf("ExtractSnippets = %d snippets, want 2: %+v", len(snips), snips)
	}
	if snips[0].ID != "go-echo" || snips[0].Language != "go" {
		t.Errorf("snippet[0] id/lang = %q/%q, want go-echo/go", snips[0].ID, snips[0].Language)
	}
	if !strings.Contains(snips[0].Code, "cordum-io/cap/v2/sdk/go") {
		t.Errorf("snippet[0] code missing the SDK import: %q", snips[0].Code)
	}
	if strings.Contains(snips[0].Code, "```") {
		t.Errorf("snippet[0] code must not include fence lines: %q", snips[0].Code)
	}
	if snips[1].ID != "py-echo" || snips[1].Language != "python" {
		t.Errorf("snippet[1] id/lang = %q/%q, want py-echo/python", snips[1].ID, snips[1].Language)
	}
}

func TestExtractSnippets_EmptyWhenNoMarkers(t *testing.T) {
	snips, err := ExtractSnippets("# Just prose\nno snippets here")
	if err != nil {
		t.Fatalf("ExtractSnippets error: %v", err)
	}
	if len(snips) != 0 {
		t.Errorf("ExtractSnippets = %d, want 0 for a doc with no snippet markers", len(snips))
	}
}

func TestCheckPublicImports_AcceptsPublicGo(t *testing.T) {
	s := Snippet{ID: "go-ok", Language: "go", Code: strings.Join([]string{
		"package main",
		"import (",
		"\t\"fmt\"",
		"\tcapgo \"github.com/cordum-io/cap/v2/sdk/go\"",
		")",
		"func main() { fmt.Println(capgo.Version) }",
	}, "\n")}
	if ps := CheckPublicImports(s); len(ps) != 0 {
		t.Errorf("CheckPublicImports(public go) = %+v, want none", ps)
	}
}

func TestCheckPublicImports_FlagsInternalGoImport(t *testing.T) {
	s := Snippet{ID: "go-bad", Language: "go", Code: strings.Join([]string{
		"package main",
		"import capgo \"github.com/cordum-io/cap/v2/sdk/go\"",
		"import bad \"github.com/cordum-io/cap/v2/internal/releasetruth\"",
		"func main() { _, _ = capgo.Version, bad.Load }",
	}, "\n")}
	ps := CheckPublicImports(s)
	if len(ps) != 1 {
		t.Fatalf("CheckPublicImports = %+v, want one problem", ps)
	}
	if ps[0].Import != "github.com/cordum-io/cap/v2/internal/releasetruth" {
		t.Errorf("offending import = %q, want the internal path", ps[0].Import)
	}
}

func TestCheckPublicImports_FlagsRelativeNodeImport(t *testing.T) {
	s := Snippet{ID: "node-bad", Language: "node", Code: strings.Join([]string{
		"const cap = require('cap-sdk-node');",
		"const local = require('../src/index');",
	}, "\n")}
	ps := CheckPublicImports(s)
	if len(ps) != 1 || ps[0].Import != "../src/index" {
		t.Fatalf("CheckPublicImports = %+v, want one problem targeting ../src/index", ps)
	}
}

func TestCheckPublicImports_FlagsRelativePythonImport(t *testing.T) {
	s := Snippet{ID: "py-bad", Language: "python", Code: strings.Join([]string{
		"from cap import client",
		"from .helpers import thing",
	}, "\n")}
	ps := CheckPublicImports(s)
	if len(ps) != 1 {
		t.Fatalf("CheckPublicImports = %+v, want one problem", ps)
	}
	if !strings.Contains(ps[0].Import, "helpers") {
		t.Errorf("offending import = %q, want the relative helpers import", ps[0].Import)
	}
}

func TestCheckPublicImports_PythonHandlesEmptyImportLine(t *testing.T) {
	// A snippet line that is exactly "from " or "import " with nothing after it
	// must not panic the checker; the malformed line is skipped.
	s := Snippet{ID: "py-empty", Language: "python", Code: strings.Join([]string{
		"import cap",
		"from ",
		"import ",
	}, "\n")}
	ps := CheckPublicImports(s) // must not panic
	if len(ps) != 0 {
		t.Errorf("CheckPublicImports = %+v, want no problems (valid cap import; empty lines skipped)", ps)
	}
}

func TestCheckPublicImports_FlagsMissingSDKImport(t *testing.T) {
	s := Snippet{ID: "go-nosdk", Language: "go", Code: strings.Join([]string{
		"package main",
		"import \"fmt\"",
		"func main() { fmt.Println(\"hi\") }",
	}, "\n")}
	ps := CheckPublicImports(s)
	if len(ps) != 1 {
		t.Fatalf("CheckPublicImports = %+v, want one problem for missing SDK import", ps)
	}
	if !strings.Contains(strings.ToLower(ps[0].Reason), "sdk") {
		t.Errorf("reason = %q, want it to mention the missing SDK import", ps[0].Reason)
	}
}
