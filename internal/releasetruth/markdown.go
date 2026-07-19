package releasetruth

// GitHubSlug converts one heading's text to its GitHub anchor slug: lowercased,
// spaces collapsed to single hyphens, and characters other than [a-z0-9-_]
// dropped. It does not apply duplicate suffixes.
func GitHubSlug(text string) string {
	return ""
}

// DocumentAnchors returns the ordered anchor slugs for all ATX headings in the
// document, applying GitHub duplicate-slug suffixes (foo, foo-1, foo-2) and
// ignoring fenced code blocks.
func DocumentAnchors(content string) []string {
	return nil
}

// RenderRoot selects how repo-relative links in a file resolve on GitHub.
type RenderRoot string

const (
	// RootNormal: relative links resolve from the file's own directory.
	RootNormal RenderRoot = "normal"
	// RootDocs: docs-tree file; relative links resolve from the file's directory.
	RootDocs RenderRoot = "docs"
	// RootWiki: wiki page; repo-relative file links do not resolve.
	RootWiki RenderRoot = "wiki"
	// RootIssueTemplate: issue-template file; relative links resolve from repo root.
	RootIssueTemplate RenderRoot = "issue-template"
)

// LinkProblem is one broken markdown link or anchor with file:line context.
type LinkProblem struct {
	File   string
	Line   int
	Target string
	Reason string
}

// CheckLinks validates every markdown link in content (the file `file` relative
// to repoRoot, rendered under `root`): same-file "#anchor" links against the
// document's headings, and repo-relative file targets against disk. Links inside
// fenced code blocks are ignored. Problems are returned in ascending line order.
func CheckLinks(repoRoot, file, content string, root RenderRoot) []LinkProblem {
	return []LinkProblem{{Reason: "not implemented"}}
}
