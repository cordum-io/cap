package releasetruth

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGitHubSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"MCP vs CAP", "mcp-vs-cap"},
		{"CAP is not!", "cap-is-not"},
		{"What's New?", "whats-new"},
		{"Foo   Bar", "foo-bar"},
		{"under_score kept", "under_score-kept"},
	}
	for _, tc := range cases {
		if got := GitHubSlug(tc.in); got != tc.want {
			t.Errorf("GitHubSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDocumentAnchors_AppliesDuplicateSuffixes(t *testing.T) {
	doc := strings.Join([]string{
		"# Foo",
		"## Foo",
		"### Foo",
		"## Bar",
	}, "\n")
	got := DocumentAnchors(doc)
	want := []string{"foo", "foo-1", "foo-2", "bar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DocumentAnchors = %v, want %v", got, want)
	}
}

func TestDocumentAnchors_IgnoresFencedHeadings(t *testing.T) {
	doc := strings.Join([]string{
		"# Real Heading",
		"```",
		"# Not A Heading",
		"```",
		"## Second Real",
	}, "\n")
	got := DocumentAnchors(doc)
	want := []string{"real-heading", "second-real"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DocumentAnchors = %v, want %v (fenced heading must be ignored)", got, want)
	}
}

func TestCheckLinks_FlagsMissingRepoRelativePathWithLine(t *testing.T) {
	root := t.TempDir()
	doc := strings.Join([]string{
		"# Doc",
		"see [here](./missing.md) for details",
	}, "\n")
	ps := CheckLinks(root, "README.md", doc, RootNormal)
	if len(ps) != 1 {
		t.Fatalf("CheckLinks = %d problems, want 1: %+v", len(ps), ps)
	}
	if ps[0].Line != 2 {
		t.Errorf("problem line = %d, want 2", ps[0].Line)
	}
	if ps[0].Target != "./missing.md" {
		t.Errorf("problem target = %q, want ./missing.md", ps[0].Target)
	}
}

func TestCheckLinks_FlagsWhitespaceOnlyTargetWithoutPanicking(t *testing.T) {
	root := t.TempDir()
	doc := strings.Join([]string{
		"# Doc",
		"see [here]( ) for details",
	}, "\n")
	ps := CheckLinks(root, "README.md", doc, RootNormal)
	if len(ps) != 1 {
		t.Fatalf("CheckLinks = %d problems, want 1: %+v", len(ps), ps)
	}
	if ps[0].Reason != "empty link target" {
		t.Errorf("problem reason = %q, want %q", ps[0].Reason, "empty link target")
	}
}

func TestCheckLinks_AcceptsExistingRepoRelativePath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "spec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spec", "01-overview.md"), []byte("# Overview"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := "[spec](spec/01-overview.md)"
	if ps := CheckLinks(root, "README.md", doc, RootNormal); len(ps) != 0 {
		t.Errorf("CheckLinks = %+v, want no problems for existing path", ps)
	}
}

func TestCheckLinks_IgnoresLinksInFencedCode(t *testing.T) {
	root := t.TempDir()
	doc := strings.Join([]string{
		"```",
		"[broken](./nope.md)",
		"```",
	}, "\n")
	if ps := CheckLinks(root, "README.md", doc, RootNormal); len(ps) != 0 {
		t.Errorf("CheckLinks = %+v, want no problems for link inside code fence", ps)
	}
}

func TestCheckLinks_SameFileAnchors(t *testing.T) {
	root := t.TempDir()
	good := strings.Join([]string{
		"# Getting Started",
		"jump to [start](#getting-started)",
	}, "\n")
	if ps := CheckLinks(root, "README.md", good, RootNormal); len(ps) != 0 {
		t.Errorf("CheckLinks(good anchor) = %+v, want none", ps)
	}
	bad := strings.Join([]string{
		"# Getting Started",
		"jump to [x](#does-not-exist)",
	}, "\n")
	ps := CheckLinks(root, "README.md", bad, RootNormal)
	if len(ps) != 1 || ps[0].Target != "#does-not-exist" {
		t.Errorf("CheckLinks(bad anchor) = %+v, want one problem targeting #does-not-exist", ps)
	}
}

func TestCheckLinks_RejectsTraversalPath(t *testing.T) {
	root := t.TempDir()
	doc := "[escape](../../etc/passwd)"
	ps := CheckLinks(root, "README.md", doc, RootNormal)
	if len(ps) != 1 {
		t.Fatalf("CheckLinks = %+v, want one problem for traversal", ps)
	}
	if ps[0].Target != "../../etc/passwd" {
		t.Errorf("problem target = %q, want ../../etc/passwd", ps[0].Target)
	}
	if !strings.Contains(strings.ToLower(ps[0].Reason), "unsafe") &&
		!strings.Contains(strings.ToLower(ps[0].Reason), "escape") {
		t.Errorf("reason %q should explain the path is unsafe/escapes the repo", ps[0].Reason)
	}
}

// Render-root awareness: an issue-template link resolves from the repo root, not
// from the template's own directory.
func TestCheckLinks_IssueTemplateResolvesFromRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "spec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spec", "01-overview.md"), []byte("# Overview"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := ".github/ISSUE_TEMPLATE/bug.md"
	doc := "[spec](spec/01-overview.md)"
	if ps := CheckLinks(root, file, doc, RootIssueTemplate); len(ps) != 0 {
		t.Errorf("issue-template link = %+v, want none (resolves from repo root)", ps)
	}
	// Under normal rules the same link resolves relative to the file's dir and misses.
	if ps := CheckLinks(root, file, doc, RootNormal); len(ps) != 1 {
		t.Errorf("normal-root link = %+v, want one problem (resolves under .github/ISSUE_TEMPLATE)", ps)
	}
}

// Render-root awareness: a repo-relative file link does not resolve on a wiki page.
func TestCheckLinks_WikiFlagsRepoRelativeFileLink(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "spec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spec", "00-index.md"), []byte("# Index"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := "[index](spec/00-index.md)"
	ps := CheckLinks(root, "Home.md", doc, RootWiki)
	if len(ps) != 1 {
		t.Fatalf("wiki link = %+v, want one problem (repo-relative link cannot resolve on wiki)", ps)
	}
	// The same path DOES resolve for a normal repo page.
	if ps := CheckLinks(root, "Home.md", doc, RootNormal); len(ps) != 0 {
		t.Errorf("normal wiki-mirror link = %+v, want none", ps)
	}
}
