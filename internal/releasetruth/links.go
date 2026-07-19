package releasetruth

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

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

var reMarkdownLink = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func isExternal(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "//")
}

// CheckLinks validates every markdown link in content (the file `file` relative
// to repoRoot, rendered under `root`): same-file "#anchor" links against the
// document's headings, and repo-relative file targets against disk. Links inside
// fenced code blocks are ignored. Problems are returned in ascending line order.
func CheckLinks(repoRoot, file, content string, root RenderRoot) []LinkProblem {
	anchors := anchorSet(DocumentAnchors(content))
	var problems []LinkProblem
	inFence := false
	for i, raw := range strings.Split(normalizeNewlines(content), "\n") {
		if isFenceLine(strings.TrimSpace(raw)) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		for _, mt := range reMarkdownLink.FindAllStringSubmatch(raw, -1) {
			if p, ok := checkTarget(repoRoot, file, mt[1], root, anchors); ok {
				p.File, p.Line = file, i+1
				problems = append(problems, p)
			}
		}
	}
	return problems
}

func anchorSet(anchors []string) map[string]bool {
	set := make(map[string]bool, len(anchors))
	for _, a := range anchors {
		set[a] = true
	}
	return set
}

// checkTarget validates a single link target, returning a problem (with File and
// Line left for the caller to fill) when the link is broken.
func checkTarget(repoRoot, file, rawTarget string, root RenderRoot, anchors map[string]bool) (LinkProblem, bool) {
	target := strings.TrimSpace(rawTarget)
	urlPart := strings.Fields(target)[0] // drop optional "title"
	if isExternal(urlPart) {
		return LinkProblem{}, false
	}
	if strings.HasPrefix(urlPart, "#") {
		if !anchors[strings.TrimPrefix(urlPart, "#")] {
			return LinkProblem{Target: target, Reason: "same-file anchor not found among document headings"}, true
		}
		return LinkProblem{}, false
	}
	pathPart := urlPart
	if idx := strings.IndexByte(pathPart, '#'); idx >= 0 {
		pathPart = pathPart[:idx]
	}
	if pathPart == "" {
		return LinkProblem{}, false
	}
	return resolvePath(repoRoot, file, target, pathPart, root)
}

func resolvePath(repoRoot, file, target, pathPart string, root RenderRoot) (LinkProblem, bool) {
	if root == RootWiki && (strings.Contains(pathPart, "/") || strings.HasSuffix(pathPart, ".md")) {
		return LinkProblem{Target: target, Reason: "repo-relative link does not resolve on a wiki page; use an absolute URL"}, true
	}
	base := path.Dir(file)
	if root == RootIssueTemplate {
		base = "."
	}
	resolved := path.Clean(path.Join(base, pathPart))
	if resolved == ".." || strings.HasPrefix(resolved, "../") || path.IsAbs(pathPart) {
		return LinkProblem{Target: target, Reason: "unsafe: link escapes the repository root"}, true
	}
	if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(resolved))); err != nil {
		return LinkProblem{Target: target, Reason: "path not found: " + resolved}, true
	}
	return LinkProblem{}, false
}
