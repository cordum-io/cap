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
