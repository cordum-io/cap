package releasetruth

// Snippet markers wrap a fenced quickstart code block so it can be extracted
// and validated independently of the surrounding prose.
const (
	snippetBeginPrefix = "<!-- cap-release:snippet:"
	snippetEndMarker   = "<!-- cap-release:snippet-end -->"
)

// Snippet is one extracted quickstart code block.
type Snippet struct {
	ID       string // identifier from the snippet marker
	Language string // go | node | python
	Code     string // fenced-block body, LF-joined, without the ``` fence lines
}

// ExtractSnippets returns all cap-release quickstart snippets in content. A
// snippet is a fenced code block wrapped by a
// "<!-- cap-release:snippet:<id>:<lang> -->" begin marker and a
// "<!-- cap-release:snippet-end -->" end marker.
func ExtractSnippets(content string) ([]Snippet, error) {
	return nil, errNotImplemented
}

// ImportProblem reports a snippet import that is not a public, installed
// package: a relative ("./", "../") path, a repository-internal path, or a
// missing published SDK import.
type ImportProblem struct {
	SnippetID string
	Language  string
	Import    string
	Reason    string
}

// CheckPublicImports verifies a snippet imports only public installed packages:
// no relative or repository-internal imports, and the language's published SDK
// import must be present. It returns the exact offending imports.
func CheckPublicImports(s Snippet) []ImportProblem {
	return []ImportProblem{{Reason: "not implemented"}}
}
