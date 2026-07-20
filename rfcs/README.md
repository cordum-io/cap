# CAP RFCs

Substantive changes to CAP are proposed as **RFCs** (Requests for Comments) recorded in
this directory. Unlike a chat thread, an RFC is a durable, reviewable file in the
repository. GitHub Discussions and Issues may gather early input, but the canonical
proposal and its decision live here and in [../DECISIONS.md](../DECISIONS.md).

See [../GOVERNANCE.md](../GOVERNANCE.md) for how RFCs are decided.

## When an RFC is required

| Change | RFC required? | Class | Minimum review |
|--------|---------------|-------|----------------|
| Docs, comments, examples, non-normative edits | No (normal PR) | `editorial` | normal PR review |
| New optional wire field/message, new profile capability | **Yes** | `additive-wire` | **14 days** + spec/proto/TCK/codegen deltas |
| Governance, roles, security posture, licensing | **Yes** | `governance` or `security` | **14 days** |
| Renumber / remove / repurpose a wire field | **Forbidden in v1** | — | — (see stability pledge) |

## Process

1. Copy [`0000-template.md`](0000-template.md) to `NNNN-short-title.md`, choosing the
   next free number.
2. Fill in the frontmatter and body. Open a pull request. Set `status: Draft`.
3. When ready for review, set `status: Review` and set `review-opens` to that date and
   `review-closes` to at least the minimum window later for the class.
4. Discuss on the PR. The shepherding maintainer records the outcome in
   [../DECISIONS.md](../DECISIONS.md) and sets `status:` to `Accepted` or `Rejected`
   with the `decision:` link **only after** `review-closes` has actually passed.
5. Do **not** backdate `review-opens`/`review-closes`, and do not mark an RFC Accepted
   before its review window closes. The governance checker enforces this.

## Frontmatter schema

Each RFC begins with a fenced `---` block of `key: value` lines:

```
---
rfc: 0001
title: Short human title
status: Draft            # Draft | Review | Accepted | Rejected | Withdrawn | Superseded
class: governance        # editorial | additive-wire | governance | security
created: 2026-07-19      # YYYY-MM-DD, the day the file was added
review-opens: none       # YYYY-MM-DD when status becomes Review, else none
review-closes: none      # YYYY-MM-DD, >= review-opens + class minimum window, else none
authors: @handle         # comma-separated GitHub handles
supersedes: none         # RFC number this replaces, or none
superseded-by: none      # RFC number that replaced this, or none
decision: none           # DECISIONS.md anchor once Accepted/Rejected, else none
---
```

The `class` minimum windows are: `editorial` = 0 days, `additive-wire` = 14 days,
`governance` = 14 days, `security` = 14 days.

## Status meanings

- **Draft** — being written; not yet under formal review.
- **Review** — review window open; `review-opens`/`review-closes` set.
- **Accepted** / **Rejected** — decided; requires a `decision:` link and a closed review
  window.
- **Withdrawn** — retracted by its authors before a decision.
- **Superseded** — replaced by a later RFC (see `superseded-by`).

## Index

| RFC | Title | Status | Class |
|-----|-------|--------|-------|
| [0001](0001-governance-process.md) | CAP governance process | Draft | governance |
