package releasetruth

import (
	"fmt"
	"regexp"
	"strings"
)

// Snippet markers wrap a fenced quickstart code block so it can be extracted and
// validated independently of the surrounding prose.
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

// parseSnippetBegin returns the id and language if trimmed is a snippet begin
// marker ("<!-- cap-release:snippet:<id>:<lang> -->").
func parseSnippetBegin(trimmed string) (id, lang string, ok bool) {
	if !strings.HasPrefix(trimmed, snippetBeginPrefix) || !strings.HasSuffix(trimmed, blockMarkerClose) {
		return "", "", false
	}
	mid := strings.TrimSpace(trimmed[len(snippetBeginPrefix) : len(trimmed)-len(blockMarkerClose)])
	idx := strings.LastIndex(mid, ":")
	if idx <= 0 || idx == len(mid)-1 {
		return "", "", false
	}
	return mid[:idx], mid[idx+1:], true
}

// ExtractSnippets returns all cap-release quickstart snippets in content, in
// document order. It errors on a snippet begin marker without a matching
// snippet-end marker.
func ExtractSnippets(content string) ([]Snippet, error) {
	lines := strings.Split(normalizeNewlines(content), "\n")
	var snips []Snippet
	for i := 0; i < len(lines); i++ {
		id, lang, ok := parseSnippetBegin(strings.TrimSpace(lines[i]))
		if !ok {
			continue
		}
		var body []string
		inFence, found, j := false, false, i+1
		for ; j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			if t == snippetEndMarker {
				found = true
				break
			}
			if isFenceLine(t) {
				inFence = !inFence
				continue
			}
			if inFence {
				body = append(body, lines[j])
			}
		}
		if !found {
			return nil, fmt.Errorf("snippet %q is missing %s", id, snippetEndMarker)
		}
		snips = append(snips, Snippet{ID: id, Language: lang, Code: strings.Join(body, "\n")})
		i = j
	}
	return snips, nil
}

// ImportProblem reports a snippet import that is not a public, installed package.
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
	switch s.Language {
	case "go":
		return checkGoImports(s)
	case "node":
		return checkNodeImports(s)
	case "python":
		return checkPythonImports(s)
	default:
		return []ImportProblem{{s.ID, s.Language, "", "unknown snippet language"}}
	}
}

func extractQuoted(s string) string {
	a := strings.IndexByte(s, '"')
	if a < 0 {
		return ""
	}
	b := strings.IndexByte(s[a+1:], '"')
	if b < 0 {
		return ""
	}
	return s[a+1 : a+1+b]
}

func checkGoImports(s Snippet) []ImportProblem {
	var ps []ImportProblem
	hasSDK := false
	inBlock := false
	for _, line := range strings.Split(s.Code, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "import (") {
			inBlock = true
			continue
		}
		if inBlock && t == ")" {
			inBlock = false
			continue
		}
		if !inBlock && !strings.HasPrefix(t, "import ") {
			continue
		}
		imp := extractQuoted(t)
		if imp == "" {
			continue
		}
		internal := strings.Contains(imp, "/internal/")
		if strings.HasPrefix(imp, "github.com/cordum-io/cap/v2") && !internal {
			hasSDK = true
		}
		if internal {
			ps = append(ps, ImportProblem{s.ID, "go", imp, "repository-internal import is not a public package"})
		} else if strings.HasPrefix(imp, ".") {
			ps = append(ps, ImportProblem{s.ID, "go", imp, "relative import is not a public package"})
		}
	}
	if !hasSDK {
		ps = append(ps, ImportProblem{s.ID, "go", "", "missing published SDK import (github.com/cordum-io/cap/v2/...)"})
	}
	return ps
}

var (
	reNodeRequire = regexp.MustCompile(`require\(\s*['"]([^'"]+)['"]\s*\)`)
	reNodeFrom    = regexp.MustCompile(`from\s+['"]([^'"]+)['"]`)
)

func checkNodeImports(s Snippet) []ImportProblem {
	var ps []ImportProblem
	hasSDK := false
	var specs []string
	for _, m := range reNodeRequire.FindAllStringSubmatch(s.Code, -1) {
		specs = append(specs, m[1])
	}
	for _, m := range reNodeFrom.FindAllStringSubmatch(s.Code, -1) {
		specs = append(specs, m[1])
	}
	for _, spec := range specs {
		if spec == "cap-sdk-node" || strings.HasPrefix(spec, "cap-sdk-node/") {
			hasSDK = true
		}
		if strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") {
			ps = append(ps, ImportProblem{s.ID, "node", spec, "relative or local import is not a public package"})
		}
	}
	if !hasSDK {
		ps = append(ps, ImportProblem{s.ID, "node", "", "missing published SDK import (cap-sdk-node)"})
	}
	return ps
}

func checkPythonImports(s Snippet) []ImportProblem {
	var ps []ImportProblem
	hasSDK := false
	for _, line := range strings.Split(s.Code, "\n") {
		t := strings.TrimSpace(line)
		var mod string
		switch {
		case strings.HasPrefix(t, "from "):
			mod = strings.Fields(strings.TrimPrefix(t, "from "))[0]
		case strings.HasPrefix(t, "import "):
			mod = strings.TrimSuffix(strings.Fields(strings.TrimPrefix(t, "import "))[0], ",")
		default:
			continue
		}
		if mod == "cap_sdk_python" || strings.HasPrefix(mod, "cap_sdk_python.") {
			hasSDK = true
		}
		if strings.HasPrefix(mod, ".") {
			ps = append(ps, ImportProblem{s.ID, "python", mod, "relative import is not a public package"})
		}
	}
	if !hasSDK {
		ps = append(ps, ImportProblem{s.ID, "python", "", "missing published SDK import (cap_sdk_python)"})
	}
	return ps
}
