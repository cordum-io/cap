# CAP Decision Records

Every governance decision — accepting or rejecting an RFC, seating or removing a
maintainer, a security-emergency retrospective — is recorded as an **append-only** entry
in [../DECISIONS.md](../DECISIONS.md). Decision records are never edited to change an
outcome; a later decision that changes course is a **new** entry that supersedes the
old one via the `superseded-by` / `supersedes` fields.

See [../GOVERNANCE.md](../GOVERNANCE.md) for who may decide and under what quorum.

## Why append-only

The value of a decision log is that it cannot be quietly rewritten. Editing an accepted
decision to say something different destroys that value. The governance checker treats a
change to an existing decision's outcome fields as an error and expects new decisions to
be appended.

## What every entry must contain

The fields in [`0000-template.md`](0000-template.md), in order:

- **id** — `D-NNNN`, monotonically increasing, matching its heading anchor.
- **rfc** — the RFC number this decides, or `none` for non-RFC decisions (e.g. maintainer
  seating).
- **date** — `YYYY-MM-DD`, the day the decision was recorded. Must be **on or after** the
  RFC's `review-closes` date; a decision cannot predate the close of its own review.
- **status** — `Accepted`, `Rejected`, or `Superseded`.
- **eligible-voters** — handles with affiliations.
- **recused** — handles who recused and why, or `none`.
- **quorum** — `cast/eligible-non-recused`, and whether quorum was met.
- **tally** — for / against.
- **rationale** — why.
- **minority-view** — dissent, or `none`.
- **supersedes / superseded-by** — decision ids, or `none`.
- **links** — PR, RFC file, discussion.

## Relationship to meetings

A meeting can recommend, but a decision is only real once it is recorded here with a
quorum that satisfies [../GOVERNANCE.md](../GOVERNANCE.md). See
[../community/MEETINGS.md](../community/MEETINGS.md).
