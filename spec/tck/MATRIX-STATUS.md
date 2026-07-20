# Cross-language fixture/signature matrix — status

The matrix engine (`internal/tck/matrix.go`) drives every producer SDK's signed
fixtures through every consumer SDK, comparing normalized-semantic and unsigned-
preimage digests and verifying signatures, and **computes** completeness with
`MissingEdges` rather than hard-coding a passing list.

## Edge status (3 stable SDKs → 9 edges)

| producer → consumer | status |
|---------------------|--------|
| go → go             | ✅ proven (`matrix_test.go`) |
| go → python, python → * | ⏳ gated |
| go → node, node → * | ⏳ gated |

`TestMatrixCompletenessIsComputed` asserts the eight non-go edges are reported
missing, so the gap is enforced, not hidden.

## Why the cross-language edges are gated

Verified at CAP `origin/main = ed0d8bd`:

- **Python has no `sign`/`verify` API** on this baseline — a Python producer or
  consumer cannot participate in a signature matrix. That API arrives with the
  unmerged Python P0 (task-83785a91) / CAP-PRODUCTION (task-a13f83fa).
- **Node signature enforcement** is the unmerged Node P0 (task-de2ef8fb).
- **Installed-artifact isolation** (Go module proxy with `GOWORK=off`, a clean
  Python wheel venv, an `npm pack` tgz consumer) depends on the Node/Python
  packaging fixes, both unmerged.
- The **production identity/attempt/resource/signature corpus** is defined by
  CAP-PRODUCTION (task-a13f83fa), which is on no ref.

When those land, each SDK is wrapped as a `tck.Consumer` and added to the matrix
with its own installed-artifact producer; the engine and the go→go reference
edge are already in place and unchanged.
