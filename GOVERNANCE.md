# CAP Governance

This document describes how the Cordum Agent Protocol (CAP) is governed **today**,
and the rules by which that governance may change. It describes the actual current
state, not an aspiration. Where a capability does not yet exist, this document says so.

## Current status (read this first)

- CAP is an open-source project published under [Apache-2.0](LICENSE).
- **Stewardship is currently held by Cordum.** The project is hosted in a
  Cordum-controlled GitHub organization and, at the time of writing, has a
  **single active maintainer** (see [MAINTAINERS.md](MAINTAINERS.md)).
- **There is no independent governance body.** No Technical Steering Committee
  (TSC), no neutral legal entity, no foundation affiliation, and no independent
  maintainer currently exists. Statements elsewhere describing a TSC, a foundation,
  or "neutral" custody as *current* facts are incorrect; this document is the source
  of truth for governance status.
- GitHub collaborator permissions (for example, an account with `maintain` or
  `admin` rights on the repository) are an operational access control. They are
  **not** evidence of an independent maintainer or of independent governance.

Because a single organization currently stewards the project, every rule below that
concerns fairness (voting, quorum, recusal, appeal) is written to be enforceable the
moment a second, unaffiliated maintainer joins — and to fail safe until then.

## Roles

Roles and the promotion ladder (Contributor → Reviewer → Committer → Maintainer) are
defined in [MAINTAINERS.md](MAINTAINERS.md), which lists only the roles that are
actually filled today. This section summarizes authority; MAINTAINERS.md is
authoritative for who holds it.

- **Maintainer** — Final authority over spec, proto, and release direction; security
  response; may merge wire-level changes. Bound by the recusal and conflict-of-interest
  rules below and in [COI.md](COI.md).
- **Committer** — Merge rights for non-wire changes (SDK features, docs, tooling,
  examples). May not merge spec/proto/wire changes.
- **Reviewer** — Trusted reviewer whose approval counts toward merge requirements for
  their area; no merge rights.
- **Contributor** — Anyone who opens issues/PRs, reviews, or joins discussions. No
  special permission required.

## Decision making

CAP is **consensus-first**. Most changes are decided in the open on the pull request
or RFC, and merged when there are no unresolved objections from eligible reviewers.

When consensus cannot be reached, an eligible maintainer may call a vote:

- **Eligible voters** are the active maintainers listed in
  [MAINTAINERS.md](MAINTAINERS.md), minus anyone recused (see below).
- **Quorum** is a majority of *non-recused* active maintainers. If recusals leave no
  unconflicted quorum, the decision is **deferred** — it is not made — except for the
  time-boxed security exception below.
- A proposal passes on a simple majority of cast votes at quorum. Ties fail (the
  status quo wins).
- Every vote is recorded in [DECISIONS.md](DECISIONS.md) with the eligible voters,
  their affiliations, recusals, tally, and rationale.

> **Bootstrap reality:** with one active maintainer there is no meaningful vote today.
> During this period the maintainer decides in the open, records the decision, and any
> change to governance, security posture, or licensing additionally requires the
> comment period below and explicit human sign-off. This is stewardship, not neutral
> governance, and is labeled as such.

## RFC process

Substantive changes use a durable, in-repo RFC record under [`rfcs/`](rfcs/README.md)
— not an ephemeral discussion thread. GitHub Discussions and Issues may gather input,
but the canonical, reviewable decision lives in the repository.

Compatibility classes and their minimum review windows:

| Class | Examples | Review window |
|-------|----------|---------------|
| Editorial / non-normative | Docs, comments, examples | Normal PR review |
| Additive wire | New optional field/message, new profile capability | **14 days** + spec/proto/TCK/codegen deltas |
| Governance / security | This document, COI, security posture, roles | **14 days** |
| Wire-breaking | Renumber/remove/repurpose a field | **Forbidden inside v1** (see stability pledge) |

Each RFC records problem, proposal, alternatives, and compatibility impact, and is
shepherded to an Accepted/Rejected decision recorded in
[DECISIONS.md](DECISIONS.md). See [rfcs/README.md](rfcs/README.md) for the full
process and [rfcs/0000-template.md](rfcs/0000-template.md) for the template.

## Conflict of interest and recusal

All maintainers disclose employment, contracting, equity, and competitive interests
annually and per-decision, per [COI.md](COI.md). A maintainer **must recuse** from any
decision that materially benefits them or their employer — including promotion of their
own implementation, maintainer removal, certification/endorsement, partnership,
release timing that benefits a commercial interest, or trademark matters. A recused
maintainer is removed from the quorum denominator for that decision.

## Appeals

A contributor who believes a decision violated this document may appeal by opening a
governance RFC that cites the specific rule. The appeal is decided by the non-recused
maintainers under the voting rules above and recorded in
[DECISIONS.md](DECISIONS.md). Until an independent body exists, an appeal cannot be
escalated beyond the maintainers; this limitation is disclosed honestly rather than
papered over with a body that does not exist.

## Inactivity, emeritus, and removal

- A maintainer with no substantive activity for **6 months** may be moved to
  **emeritus** by the process in [MAINTAINERS.md](MAINTAINERS.md). Emeritus
  maintainers retain credit but not voting rights or merge access.
- A maintainer may be removed for sustained inactivity, repeated COI violations, or
  Code of Conduct violations, by a vote of the other non-recused maintainers. The
  subject does not vote on their own removal.

## Security emergencies

A maintainer may take a **temporary, fail-closed** action to mitigate an active
security threat (for example, disabling a vulnerable path or yanking a release) without
waiting for quorum. Such an action:

- must be the least action needed to fail safe;
- must be announced to the other maintainers immediately; and
- must be reviewed **retrospectively** and recorded in [DECISIONS.md](DECISIONS.md)
  within 7 days, with any confidential details handled per [SECURITY.md](SECURITY.md).

This is the *only* exception to the "defer when there is no unconflicted quorum" rule,
and it is bounded in scope and time.

## Meetings

The project is **async-first**. There is **no recurring public meeting currently
scheduled.** If synchronous meetings are introduced, they follow
[community/MEETINGS.md](community/MEETINGS.md): agenda published ≥7 days ahead, minutes
within 3 days, and no binding decision without a corresponding
[DECISIONS.md](DECISIONS.md) entry. Meetings do not create decisions on their own.

## Contribution sign-off (DCO)

Contributions are certified under the **Developer Certificate of Origin 1.1** (see
[DCO.md](DCO.md)). Sign-off (`git commit -s`) is required **prospectively** from the
effective date stated in [CONTRIBUTING.md](CONTRIBUTING.md). The project does **not**
retroactively assert that historical commits were DCO sign-off compliant, and does not
rewrite history to add sign-offs.

## Future neutral governance (a trigger, not a body)

CAP's maintainers intend to move toward neutral, multi-organization governance. That is
a **goal with a defined trigger**, not a present fact:

- A **Technical Steering Committee will be formed when** at least **three active
  maintainers from at least two unaffiliated organizations** are seated per
  [MAINTAINERS.md](MAINTAINERS.md).
- Any foundation submission, neutral-entity transfer, trademark assignment, or CLA is a
  **counsel-owned, human-approved** action that has **not** occurred and is not implied
  by this document.

Until those triggers are met, CAP is honestly described as a single-maintainer,
Cordum-stewarded open-source project.

## Release process

- Releases are triggered by Git tags on `main`; every user-facing change requires a
  CHANGELOG entry; wire-level changes bump the wire version and require conformance
  fixtures across all SDKs. See [spec/17-versioning-policy.md](spec/17-versioning-policy.md)
  and the canonical release manifest for the authoritative version matrix.

## Protocol stability pledge

The CAP v1 wire format is **frozen** — append-only evolution only. Existing fields are
never renumbered, removed, or repurposed, so any conformant v1 implementation keeps
interoperating with future versions. New capability arrives as new fields/messages.

## Licensing and trademark

- **Code and specification:** [Apache-2.0](LICENSE). Apache-2.0 is a license; it does
  **not** imply Apache Software Foundation affiliation, endorsement, or membership.
- **"CAP"** and **"Cordum Agent Protocol"** are trademarks of Cordum. Conformant
  implementations may use the names to describe compatibility. A conformance
  self-report or badge is self-hosted TCK evidence, not certification, endorsement,
  partnership, adopter status, or foundation acceptance.

## Changes to this document

Governance changes follow the **governance/security** RFC class above (14-day review)
and require explicit human publication approval. Propose changes via an RFC under
[`rfcs/`](rfcs/README.md); do not backdate or self-declare the review complete.
