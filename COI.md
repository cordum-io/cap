# Conflict of Interest Policy

This policy governs how CAP maintainers disclose and manage conflicts of interest. It
exists so that decisions are made — and are seen to be made — on the merits, especially
while the project is stewarded by a single organization (see [GOVERNANCE.md](GOVERNANCE.md)).

## Who this applies to

All maintainers and, for a given decision, any reviewer or committer whose approval is
counted toward that decision.

## What must be disclosed

Each person discloses interests that could reasonably influence, or appear to influence,
their judgment on project matters, including:

- **Employment / contracting** — current employer or clients, and any paid relationship
  related to CAP, an implementation of CAP, or a competitor.
- **Equity / financial** — ownership, options, or other financial stake in an entity
  that builds on, competes with, or would benefit from CAP decisions.
- **Competitive** — maintainership or significant contribution to a competing protocol
  or product.
- **Other** — family, close personal, or advisory relationships that create a comparable
  conflict.

## When to disclose

- **Annually** — every maintainer refreshes their disclosure at least once a year.
- **Per-decision** — before participating in any decision where a conflict applies, the
  person states the conflict on the RFC/PR/decision record.

Disclosures are public and recorded with the roster in [MAINTAINERS.md](MAINTAINERS.md)
and, per-decision, in [DECISIONS.md](DECISIONS.md).

## Mandatory recusal

A person **must recuse** from a decision that would materially benefit them or their
employer/affiliate, including:

- Promotion of their own or their employer's implementation, product, or service.
- Their own appointment, promotion, or **removal**, or that of a colleague at the same
  affiliation where the conflict is material.
- **Certification, endorsement, badge, or partnership** decisions.
- **Release timing** that benefits a commercial interest.
- **Procurement**, sponsorship, or funding arrangements.
- **Trademark** use, assignment, or enforcement.
- Any change whose primary effect is a **governance benefit** to their affiliation.

Recusal means: do not vote, do not count toward quorum, and do not privately lobby other
voters. A recused person may answer factual questions on the record if asked.

## Effect on quorum

A recused person is **removed from the quorum denominator** for that decision. If
recusals leave **no unconflicted quorum**, the decision is **deferred** — it is not made
— with one exception: a maintainer may take a **temporary, fail-closed security action**
per [GOVERNANCE.md](GOVERNANCE.md), which is then reviewed retrospectively.

There is no ombudsperson or independent ethics office; the project does not claim one.
Conflicts are handled in the open by the non-recused maintainers, and unresolved
conflicts result in deferral rather than a decision made under conflict.

## Single-affiliation reality

While all maintainers share one affiliation, decisions that would benefit that
affiliation cannot achieve an unconflicted quorum and are therefore **deferred** or
escalated to explicit human approval outside the vote. This is a deliberate fail-closed
consequence of the current single-organization stewardship, not a loophole to be waived.

## Records and review

Per-decision conflict disclosures and recusals are part of the permanent
[DECISIONS.md](DECISIONS.md) record. A maintainer who repeatedly fails to disclose or
recuse may be removed for cause per [MAINTAINERS.md](MAINTAINERS.md).
