---
rfc: 0001
title: CAP governance process
status: Draft
class: governance
created: 2026-07-19
review-opens: none
review-closes: none
authors: @yaront1111
supersedes: none
superseded-by: none
decision: none
---

# RFC 0001: CAP governance process

## Summary

Adopt the governance mechanics introduced in this change set — the roles and ladder in
[MAINTAINERS.md](../MAINTAINERS.md), the decision/voting/quorum/recusal rules in
[GOVERNANCE.md](../GOVERNANCE.md), the [DCO](../DCO.md) sign-off requirement, the
[conflict-of-interest policy](../COI.md), this RFC process, the append-only
[decision log](../DECISIONS.md), the [meeting](../community/MEETINGS.md) and
[triage](../community/TRIAGE.md) processes, and the machine-checkable
[readiness registry](../governance/readiness.json) — as the project's governance of
record.

## Motivation

CAP has been stewarded informally by a single organization. The prior GOVERNANCE.md
described a "small group" of maintainers that does not exist and framed CNCF membership
as direction. This RFC replaces aspiration with an honest, enforceable process that is
fair the moment a second, unaffiliated maintainer joins and fails safe until then.

## Proposal

Accept the files listed in the Summary as normative. In particular:

- Governance is consensus-first; votes use non-recused maintainer quorum and **defer**
  when no unconflicted quorum exists (one time-boxed fail-closed security exception).
- A future TSC is a **trigger** (three maintainers from two or more unaffiliated
  organizations), not a current body. No foundation affiliation is claimed.
- DCO 1.1 sign-off is required **prospectively** from the effective date in
  [CONTRIBUTING.md](../CONTRIBUTING.md); history is not rewritten.
- Readiness is computed from evidence and currently reports **BLOCKED**.

## Compatibility

Class: `governance`. Minimum review window: **14 days**. No wire, proto, or SDK behavior
changes. Editorial and process only.

## Alternatives considered

- *Keep the current document and add a MAINTAINERS file.* Rejected: it would leave the
  false "small group" and CNCF-as-direction framing in place.
- *Declare a TSC / foundation intent as current.* Rejected: no such body exists;
  claiming one would violate the project's honesty rules.

## Drawbacks and risks

The process imposes real overhead (sign-off, decision records) on a project that today
has one maintainer. That overhead is deliberate: it makes the fairness rules real before
they are needed, not after.

## Security and privacy impact

Adds a fail-closed security-emergency path and fixes a Code of Conduct hazard that
invited sensitive public reports. No change to the wire trust model.

## Unresolved questions

- Exact triage SLA duration (proposed 7 calendar days) — open for comment.
- Whether to add a break-glass team in place of org-admin always-bypass — noted as a
  post-adoption admin action.

## Decision

**Not yet decided.** This RFC remains `Draft`/`Review` until its 14-day governance
review window has actually elapsed and a maintainer records the outcome in
[../DECISIONS.md](../DECISIONS.md). It must not be marked Accepted before then.
