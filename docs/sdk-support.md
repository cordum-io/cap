# CAP SDK support tiers

This table is **generated** from `sdk/support-tiers.json` by `cap-support render`
and verified in CI by `cap-support check` — do not hand-edit the region between
the markers. Every tier claim is backed by a gate that names a real workflow,
test, or package file; the verifier resolves each path and never trusts a
hand-set boolean.

## Tiers

- **stable** — a protocol SDK with publish, install, compatibility, TCK, and
  mandatory real-NATS evidence. A stable entry may still list *pending* gates:
  the evidence file exists but its content is not yet earned (for example a
  real-NATS test that currently skips), with the blocking task named.
- **community** — owned and source-tested, but without the full stable gate set.
  C++ is community: it has no registry release, no production transport profile,
  and an abstract transport only.
- **experimental** — present but without CI, release, installed-artifact,
  transport, or ownership gates. No stable/install/production guarantees.

## Kinds

- **protocol-sdk** — speaks the CAP wire format; participates in the fixture
  matrix.
- **extension** — a companion library (e.g. Python Guard) that is *not* a wire
  SDK and is never placed in the cross-language fixture matrix.

<!-- BEGIN GENERATED SDK SUPPORT TABLE -->
| SDK | Kind | Tier | Owner | Pending evidence |
|-----|------|------|-------|------------------|
| go | protocol-sdk | stable | @yaront1111 | — |
| node | protocol-sdk | stable | @yaront1111 | compat (task-de2ef8fb), real-nats (task-de2ef8fb) |
| python | protocol-sdk | stable | @yaront1111 | compat (task-83785a91), real-nats (task-83785a91) |
| cpp | protocol-sdk | community | @yaront1111 | — |
| python-guard | extension | community | @yaront1111 | — |
| dotnet | protocol-sdk | experimental | @yaront1111 | — |
| java | protocol-sdk | experimental | @yaront1111 | — |
| php | protocol-sdk | experimental | @yaront1111 | — |
| ruby | protocol-sdk | experimental | @yaront1111 | — |
| rust | protocol-sdk | experimental | @yaront1111 | — |
<!-- END GENERATED SDK SUPPORT TABLE -->

_Pending evidence names the task that will complete a gate; see
`spec/tck/MATRIX-STATUS.md` for the cross-language matrix status._

## Promotion & demotion

Tier is a function of resolvable evidence, not intent. `cap-support check`
verifies every gate against the tree, so a claim can only be made once its
evidence exists.

- **experimental → community**: add explicit CODEOWNERS ownership, CI source
  tests, and conformance decode/verify; for a compiled SDK, an install/export
  and a clean external consumer. Label absent transport/release honestly.
- **community → stable**: add a publish workflow and package coordinate, an
  installed-artifact consumer, the bidirectional fixture/signature matrix edge,
  a role-applicable TCK run, and mandatory (no-skip) real NATS. A stable entry
  may temporarily list a gate as `pending` with the blocking task while its
  evidence lands; a fully-earned stable entry has no pending gates.
- **demotion**: if a required gate's evidence is removed or a workflow/owner/
  package path stops resolving, `cap-support check` fails — the entry must be
  demoted or the evidence restored. Tiers never silently degrade.

External publication, registry pushes, and adopter claims are **out of scope**
of this manifest and require separate evidence and human approval.

