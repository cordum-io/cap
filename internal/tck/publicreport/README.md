# publicreport

`publicreport` turns a full CAP TCK JSON report (the bytes `tck.Report.JSON`
emits) into a small, deterministic, privacy-safe **public report**. A third
party can publish it, and anyone can check it offline with `Validate`. Both
functions use only the standard library: no network, no subprocesses and no
telemetry.

A public report is self-hosted evidence of one TCK run. It is not
certification or endorsement, and CAP-owned implementations never count toward
any readiness threshold.

- Schema: [`spec/tck/schema/public-report-v1.schema.json`](../../../spec/tck/schema/public-report-v1.schema.json)
  (draft 2020-12, `additionalProperties: false` at every level).
- `schemaVersion`: `cap-public-report/v1`.

## API

```go
type Subject struct {
    TestedCommit           string   // 40 lowercase hex
    ArtifactName           string   // ^[a-z0-9][a-z0-9._-]{0,127}$
    ArtifactSha256         string   // 64 lowercase hex
    ScenarioManifestSha256 string   // 64 lowercase hex: digest over the sorted scenario files used
    RequiredCaseIDs        []string // role/profile-applicable required ids from that manifest
}

func Project(source []byte, s Subject) (Report, []byte, error)
func Validate(b []byte) (Report, error)
```

- `Project` returns the public report and its canonical bytes.
- `Validate` strictly decodes public bytes. It checks every field and the case
  order, recomputes the verdict, and requires the bytes to be exactly the
  canonical encoding.
- On any error, both return the zero `Report` (and `Project` returns nil bytes).
  They never return a partial report.

The package never imports `internal/tck`, which pulls in nats-server. It decodes
into a private mirror of `tck.Report`, `tck.Summary` and `tck.CaseResult`, and a
test keeps the mirror's json tags and kinds in parity with the originals.

### Errors

Every input rejection wraps one sentinel, so match with `errors.Is`:

| Sentinel | Cause |
|---|---|
| `ErrMalformedJSON` | not exactly one well-typed JSON value |
| `ErrUnknownField` | a field the source or public schema does not define |
| `ErrSchemaVersion` | `schemaVersion` is not `cap-public-report/v1` |
| `ErrBadCommit`, `ErrBadDigest`, `ErrBadArtifactName` | malformed subject field |
| `ErrBadToken` | role, profile, transport or tool version fails its pattern |
| `ErrBadCaseID`, `ErrBadStatus` | case id or status fails its pattern or enum |
| `ErrDuplicateCase` | a case id repeats, in the source, the subject or the public cases |
| `ErrMissingRequiredCase` | a required id is absent from the source run |
| `ErrNoRequiredCases` | an empty `RequiredCaseIDs`, or a public report without a required case |
| `ErrInconsistentSource` | the source summary does not match its own cases |
| `ErrUnsortedCases` | public cases are not strictly sorted by id |
| `ErrVerdictMismatch` | the stored verdict differs from the one the cases give |
| `ErrNotCanonical` | the public bytes are not the canonical encoding |

## Rules

**Required set (a union).** A public case is `required` if the subject lists it
in `RequiredCaseIDs`, OR the source marks it `required && applicable`. A caller
can add requirements but never remove them. A required id that the source does
not contain is a rejection: a public report never silently omits required
coverage.

**Verdict.** The verdict is a pure function of the public cases, so `Validate`
can recompute it:

1. `FAIL` if any case is `FAIL` or `ERROR`;
2. otherwise `INCOMPLETE` if any required case is not `PASS` (`UNSUPPORTED` or `N/A`);
3. otherwise `PASS`.

`Project` also cross-checks the source summary: the counts must match the cases,
and `conformant` must equal "no applicable required case is non-PASS".

**What is stripped, and why.** Only the caller's `Subject` and pattern-validated
tokens are copied. A value that fails its pattern is rejected, never sanitized.

- **Copied:** `toolVersion`, `profile`, `transport`, `role`, and per case
  `id` and `status`.
- **Never copied:** `detail`, `evidence`, `durationMs`, the adapter label, the
  sdk string, and any error text.

Those free-text fields can carry hostnames, usernames, absolute paths, env
values, tokens, tenant or principal ids and timestamps. A test plants each of
these in a source report and asserts that none of them reaches the output.

**Canonical bytes.** The only accepted byte form is `encoding/json` of the fixed
`Report` struct, which means:
- fields in declaration order;
- cases sorted by id;
- `SetEscapeHTML(false)`;
- exactly one trailing newline.

Signatures over a public report (the cap-tck report CLI) are computed over
these exact bytes. That is why `Validate` rejects any re-indented, reordered or
re-encoded copy.

## Caller obligations

- **Bound the input size before calling `Project` or `Validate`.** Neither
  function limits its input, and decoding is linear in it.
- **Derive `RequiredCaseIDs` from the scenario files** whose digest is
  `ScenarioManifestSha256`.

## Signing, verification and badges (cap-tck report)

```go
func Sign(report []byte, key ed25519.PrivateKey) ([]byte, error)
func Verify(report, sig []byte, pub ed25519.PublicKey) (Report, error)
func Badge(report []byte) ([]byte, error)
```

`Sign` validates before signing. The detached signature is the raw 64-byte
Ed25519 (RFC 8032, pure) signature over the exact canonical bytes, with no
envelope or prehash. `Verify` checks the signature before validating the report.
Errors match `ErrBadKey`, `ErrBadSignature`, `ErrSignatureMismatch`, or the
validation sentinels above; failures return no signature or the zero report.

The CLI takes caller-supplied PKCS#8 `PRIVATE KEY` and PKIX `PUBLIC KEY` PEM
files. It never creates, uploads or stores keys, and clears the key buffers it
holds after use (Go and its parsers cannot guarantee erasure of internal copies).
OpenSSL 3 can verify the same signature offline:

```sh
openssl pkeyutl -verify -pubin -inkey pub.pem -rawin -in report.json -sigfile report.sig
```

`Badge` returns a fixed SVG only for a valid PASS report with profile `core` or
`cap-production`; otherwise it returns nil and `ErrBadgeVerdict`,
`ErrBadgeProfile`, or a validation error. Only constant allowlist values reach
the SVG, never report text. A signature or badge is not an endorsement.

`cap-tck report project` derives required ids from scenarios whose `required`
is true and whose profile **and role** match the source run. Its case ids must
equal all scenario ids for that profile (other roles appear as N/A). Its
manifest digest is SHA-256 over sorted sha256sum-format lines for every direct
`*.json` file, hashing file bytes after CRLF-to-LF folding. For LF files:

```sh
cd DIR && export LC_ALL=C && sha256sum -- *.json | sha256sum
```

All four subcommands run offline, without telemetry. See `cap-tck report --help`
for flags. Output paths are relative to `--out-dir` (default `.`); final
symlinks, directories and escaping paths are refused. Outputs use mode 0600
through a same-directory temporary file and rename, cleaned up on failure.
