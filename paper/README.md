# CAP paper

**Title:** CAP: A Governance-First Wire Protocol for Distributed AI Agent Control Planes

**Status:** Publication-ready technical report draft. It is not peer reviewed yet.

## Build

Requirements: a recent TeX Live installation with `latexmk`, `pdflatex`, and Biber.

```bash
cd paper
make
```

The output is `cap-paper.pdf`.

## Pinned artifacts

- CAP: `df2af4753c0c05ba032f0592ca4eaff3d62bdc8d` (2026-06-21)
- Cordum: `6232e9d5c864467b4ff051cf9aa71b2b99e379c3` (2026-06-21)
- Historical CAP release: `v0.1.0` (2025-12-11)
- Historical CAP release: `v2.0.0` (2025-12-12)

## Publication path

1. Build and inspect the PDF.
2. Confirm author name, affiliation, and email.
3. Archive the source bundle and pinned artifacts on Zenodo to obtain a DOI.
4. Submit the manuscript to arXiv, recommended primary category `cs.DC` with `cs.CR` cross-list.
5. Replace the report URL and DOI placeholders in `CITATION.cff` after publication.
6. Announce only as an arXiv preprint until peer review.

## Claims policy

The paper deliberately avoids claims that CAP invented authorization, reference monitoring, contracts, audit logging, or post-action governance. It claims an early public integration of governance, budgets, fleet capacity, pointer-separated state, and lifecycle semantics in a distributed agent wire protocol.

The first version intentionally omits the historical benchmark numbers in `BENCHMARKS.md`; performance claims should be added only with a pinned harness, raw traces, complete hardware metadata, and uncertainty reporting.

## License

Manuscript text: CC BY 4.0. Protocol and SDK source: Apache-2.0. Cordum reference implementation: governed by its repository license.
