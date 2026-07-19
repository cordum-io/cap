package releasetruth

import "errors"

// Protected-block markers. Generated content lives strictly between a begin
// marker (carrying the block id) and the next end marker; everything outside
// the markers is human-authored and never rewritten by the renderer.
const (
	blockBeginPrefix = "<!-- cap-release:begin:"
	blockMarkerClose = " -->"
	blockEndMarker   = "<!-- cap-release:end -->"
)

// errNotImplemented is the sentinel returned by the render/quickstart stubs
// until step 4/6 implements them, so the step-3 contract tests fail closed.
var errNotImplemented = errors.New("releasetruth: not implemented")

// Block is one protected region discovered in a markdown document.
type Block struct {
	ID        string // identifier parsed from the begin marker
	BeginLine int    // 1-based line number of the begin marker
	EndLine   int    // 1-based line number of the end marker
	Inner     string // current content between the markers, LF-joined, no wrapping newlines
}

// FindBlocks returns every protected block in document order. It errors on a
// begin without a matching end, an end without a begin, a nested begin, or a
// duplicate block id.
//
// Step-3 stub: returns no blocks and no error so the contract tests fail for the
// right reason (missing discovery and missing malformed-input detection).
func FindBlocks(content string) ([]Block, error) {
	return nil, nil
}

// RenderBlock returns the generated content for a known block id, derived
// solely from the manifest. Unknown ids return an error naming the id.
//
// Step-3 stub: returns empty content and no error so the contract tests fail for
// the right reason (nothing derived, unknown ids not rejected).
func RenderBlock(m *Manifest, id string) (string, error) {
	return "", nil
}

// Render rewrites the inner content of every protected block in content with
// RenderBlock output for its id, preserving all text outside blocks and
// emitting LF newlines. It is idempotent: Render(Render(x)) == Render(x). An
// unknown block id fails closed with an error.
//
// Step-3 stub: returns empty content and no error so the contract tests fail for
// the right reason (nothing rendered, drift not detected, unknown ids not rejected).
func Render(m *Manifest, content string) (string, error) {
	return "", nil
}
