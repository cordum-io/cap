---
rfc: 0000
title: RFC template
status: Draft
class: editorial
created: 2026-07-19
review-opens: none
review-closes: none
authors: @yaront1111
supersedes: none
superseded-by: none
decision: none
---

# RFC 0000: RFC template

> Copy this file to `NNNN-short-title.md` and replace the frontmatter and sections
> below. Keep the frontmatter keys exactly as named; the governance checker parses them.

## Summary

One paragraph: what is being proposed and why, in plain language.

## Motivation

What problem does this solve? Who is affected? What happens if we do nothing?

## Proposal

The concrete change. Be specific enough that a reviewer can evaluate it and an
implementer can build it. For wire changes, name the exact fields/messages and their
tags, and confirm they are **additive** (no renumber/remove/repurpose).

## Compatibility

State the compatibility class (`editorial`, `additive-wire`, `governance`, `security`)
and justify it. For `additive-wire`, list the required deltas:

- spec change(s)
- proto change(s) (append-only)
- TCK fixture/case change(s)
- codegen impact across SDKs

## Alternatives considered

What else was considered, and why this proposal was chosen over them.

## Drawbacks and risks

Honest costs, migration burden, and failure modes.

## Security and privacy impact

Any effect on the trust model, admission, identity, or data exposure. If none, say so
and explain why.

## Unresolved questions

Open points to settle during review.

## Decision

Left blank until decided. The shepherding maintainer records the outcome in
[../DECISIONS.md](../DECISIONS.md) after the review window closes, sets `status:` to
`Accepted`/`Rejected`, and links the decision anchor in `decision:`.
