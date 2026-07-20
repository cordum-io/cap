# Cross-language fixture/signature matrix — status

The matrix engine (`internal/tck/matrix.go`) drives every producer SDK's signed
fixtures through every consumer SDK, comparing normalized-semantic and unsigned-
preimage digests and verifying signatures, and **computes** completeness with
`MissingEdges` rather than hard-coding a passing list.

## Edge status (3 stable SDKs → 9 edges)

| producer → consumer | status |
|---------------------|--------|
| go → go             | ✅ proven |
| go → node           | ✅ proven |
| go → python         | ✅ proven |
| node → go           | ✅ proven |
| node → node         | ✅ proven |
| node → python       | ✅ proven |
| python → go         | ✅ proven |
| python → node       | ✅ proven |
| python → python     | ✅ proven |

`TestCrossLanguageMatrixCoversAllNineEdges` requires `MissingEdges` to be empty
over `StableSDKs × StableSDKs`, so adding a fourth stable SDK to
`sdk/support-tiers.json` fails the matrix until its driver exists.

## What each edge proves

For every one of the four corpus cases in `spec/tck/matrix-corpus.json`:

1. The producer builds the case with **only its own SDK's public API** and signs
   it with `SignProductionPacket` / `sign_production_packet` /
   `signProductionPacket`.
2. The consumer decodes the exact wire bytes, runs its own
   `VerifyProductionPacket` equivalent against a trust store, and runs its own
   `validateBusPacket`.
3. The consumer independently recomputes the **unsigned-preimage digest** (over
   the received bytes) and the **normalized digest** (over a fresh deterministic
   re-encode of the decoded message); both must equal the producer's claim.
4. Every consumer additionally rejects a one-byte-flipped copy of the wire and a
   correct wire presented under a wrong public key.

Randomized ECDSA signature bytes are never compared for equality; agreement is
asserted on semantics, preimage bytes, and verification outcome.

## Installed artifacts, not repository source

`tools/tck/matrix/build_artifacts.py` builds what a third party would consume:

| SDK | artifact | isolation |
|-----|----------|-----------|
| Go | module zip served from a local file proxy built from committed git bytes | external module, `GOWORK=off` |
| Python | `python -m build --wheel` output installed into a fresh venv | `python -I`, repo not on `sys.path` |
| Node | `npm pack` tarball installed into an empty consumer package | resolves `cap-sdk-node` from `node_modules` |

## Running it

```bash
CAP_TCK_MATRIX=1 go test -run TestCrossLanguage ./internal/tck/...
```

The gate is opt-in because it needs three toolchains and takes ~25s to build the
artifacts. It is not optional in CI: the `cross-language-matrix` job in
`.github/workflows/sdk-gates.yml` sets `CAP_TCK_MATRIX=1`, and
`TestCrossLanguageMatrixIsEnforcedInCI` fails if that job or its environment
variable is removed. Once enabled, a missing toolchain is a hard failure — the
matrix never reports success because a language was absent.

## Interop defect this matrix already caught

The Node driver originally set `nanos: 0` explicitly on `created_at` and
`expires_at`. protobufjs encodes such an explicit zero on the wire; Go and
Python drop it as a proto3 default. Every signature still verified and every
preimage digest still matched — only the normalized digest diverged. Because
CAP-PRODUCTION signs exact bytes and de-duplicates on the signed-body digest,
that non-canonical encoding would have made a semantically identical packet look
like a distinct one to a replay store. The normalized-digest comparison is what
surfaced it.
