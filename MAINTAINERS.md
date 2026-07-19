# CAP Maintainers

This file lists the people who hold roles in the Cordum Agent Protocol (CAP) project
**today**, and the rules for gaining or losing a role. It lists **only roles that are
actually filled and evidenced**. A person's GitHub permission level is not, by itself,
evidence of a role here.

See [GOVERNANCE.md](GOVERNANCE.md) for how these roles exercise authority.

## Current roster

| Person | Role | Affiliation | Areas | Since |
|--------|------|-------------|-------|-------|
| [@yaront1111](https://github.com/yaront1111) | Maintainer | Cordum | Protocol, spec, all SDKs, release | project inception |

**Reviewers:** none seated.
**Committers:** none seated.
**Emeritus:** none.

> **Independence disclosure.** There is currently **one** active maintainer, affiliated
> with Cordum. There is **no** maintainer independent of Cordum. Any readiness or
> adoption claim that asserts an independent maintainer is false until this table shows
> one with an affiliation other than Cordum and the evidence rules below are met.

## The ladder

Roles are earned through sustained, public contribution. Each rung has objective,
non-gameable evidence and requires the candidate's explicit consent before nomination.

### Contributor
Anyone who opens issues/PRs, reviews, or participates in discussions. No permission
required; see [CONTRIBUTING.md](CONTRIBUTING.md).

### Reviewer
- **Evidence:** a track record of high-signal reviews in a specific area (e.g. `sdk/go`,
  `proto`, `spec`), demonstrated by linked PRs where the reviewer's feedback changed the
  outcome.
- **Rights:** their review counts toward the required approvals for that area. No merge
  access.
- **Process:** nominated by a maintainer; 14-day public comment; confirmed by a
  conflict-free maintainer vote.

### Committer
- **Evidence:** sustained authored contributions merged over multiple releases, plus a
  demonstrated understanding of cross-SDK implications; typically promoted from Reviewer.
- **Rights:** merge non-wire PRs (SDK features, docs, tooling, examples). May **not**
  merge spec/proto/wire changes.
- **Process:** nominated by a maintainer; 14-day public comment; conflict-free vote.

### Maintainer
- **Evidence:** sustained committer-level work plus ownership of a subsystem, good design
  judgment across releases, and participation in security and release duties.
- **Rights:** full merge rights including wire-level changes; release authority; final
  say on spec direction; security response.
- **Process:** nominated by an existing maintainer; **candidate must consent in writing**
  and disclose affiliation and conflicts per [COI.md](COI.md); **14-day** public comment
  period; confirmed by a **conflict-free** vote of existing maintainers recorded in
  [DECISIONS.md](DECISIONS.md).

## Nomination record

Every nomination is a public record containing:

1. Candidate GitHub handle and **written consent**.
2. Target role and area(s).
3. **Public evidence** — links to PRs, reviews, issues, or RFCs.
4. **Affiliation** and any conflicts of interest.
5. Nominating maintainer.
6. Comment-period open/close dates (≥14 days for Committer/Maintainer).
7. Eligible (non-recused) voters, tally, and outcome.

Records live alongside decisions in [DECISIONS.md](DECISIONS.md).

## Affiliation and independence

- Every roster entry states the person's employer/affiliation. "Independent of Cordum"
  means the person is **not** employed by, contracted by, or holding a controlling
  interest in Cordum, and holds their role on their own authority.
- Independence for readiness/adoption purposes is evaluated against
  [COI.md](COI.md) and the readiness evidence rules; it is not asserted here beyond the
  affiliation column above.

## Inactivity, emeritus, and removal

- **Inactivity:** a maintainer with no substantive activity (merges, reviews, RFC
  shepherding, security work) for **6 months** may be moved to **emeritus** by a
  conflict-free vote of the other maintainers. Emeritus maintainers keep credit but lose
  voting rights and merge access. Re-activation follows a lightweight re-confirmation.
- **Removal for cause:** repeated COI violations, sustained inactivity after notice, or
  a Code of Conduct violation may lead to removal by a conflict-free vote of the other
  maintainers. The subject does not vote on their own removal and is notified in advance.
- All status changes are recorded in [DECISIONS.md](DECISIONS.md).

## Bootstrap rule

While there is only one active maintainer:

- That maintainer cannot, alone, satisfy a "conflict-free vote" for adding a maintainer
  who shares their affiliation. The **first independent maintainer** is therefore added
  by an **open nomination with a 14-day public comment period and explicit human
  approval**, recorded in [DECISIONS.md](DECISIONS.md), rather than by a same-affiliation
  vote that would merely rubber-stamp the outcome.
- No role may be granted to satisfy a readiness metric. Roles reflect real
  responsibility that the person has consented to hold.

## Code ownership

Path ownership in [.github/CODEOWNERS](.github/CODEOWNERS) is kept in sync with this
roster and lists only real, write-enabled identities. Aspirational teams are not added.
Governance documents, RFC/decision records, the readiness evidence registry, and the
governance checker are owned by maintainers.
