# CAP Strategy — Epics & Tasks

Source: CAP-as-neutral-standard plan (2026-04-18). Structured for Moe intake.

**Bet:** CAP is a neutral standard with Cordum as the best implementation — not "Cordum's protocol."

**Global rails for every task below:**
- Normative CAP behavior MUST NOT require Cordum-specific code paths.
- No spec/adapter/packs change lands without a conformance-fixture delta plan.
- Perception changes (naming, docs, site copy) do not wait for legal foundation transfer.
- Every public claim (README, site, marketing) must be true at merge time, not aspirationally.
- CAP public-facing content (site, papers, talks, social) never co-brands with Cordum beyond a single "founding implementer" attribution line.
- CAP's publishing cadence, community events, and launch motion run on the CAP calendar, not the Cordum marketing calendar.
- Custody of CAP public assets (domain, GitHub org, trademark, social handles) sits with a non-Cordum entity before any public launch.
- No paid advertising for CAP in the first 180 days — credibility is the asset; paid media undermines the neutrality perception.

---

## EPIC 1 — Company Decision & Staffing (Days 0–14)

**Goal:** Binary decision — CAP as primary moat vs integration surface — with staffing, owner, and roadmap reallocation visible by Day 14.

**Owner:** Founder
**Success metric:** Every team member can answer "control plane vs governance standard + best implementation" in one sentence by Day 14.

- **T1.1 — CAP-vs-feature decision memo.** One-page, founder-signed. States bet, what gets deprioritized in the current Cordum roadmap (enterprise hardening, packs, scale) to fund CAP, and the 1-sentence positioning test. DoD: memo published to internal wiki; linked from Cordum + CAP READMEs.
- **T1.2 — Staff CAP as a product line.** Identify and announce: founder-level owner, protocol/spec editor, ecosystem engineer (adapters/conformance), DX engineer (SDKs/docs/examples), part-time counsel (IP/trademark/foundation). DoD: named humans in an internal `CAP_TEAM.md` with allocation percentages.
- **T1.3 — Freeze CAP public-messaging edits under a single owner.** Every change to README, site, docs, decks, repo metadata routes through one reviewer until Epic 2 ships. DoD: CODEOWNERS entry + site repo protection + documented review SLA.
- **T1.4 — Audit all CAP public surfaces.** Enumerate: `cordum.io/protocol`, `github.com/cordum-io/cap`, SDK package metadata (npm/PyPI/Go module path), spec repo, release notes, CHANGELOG, adapter claims, pack claims, governance docs, marketing assets. Produce a delta list of every surface that still reads "vendor-owned." DoD: audit spreadsheet with URL, current text, target state, owner.

---

## EPIC 2 — Neutral Branding & Separation (Days 0–30)

**Goal:** Perception layer matches the bet. CAP does not feel Cordum-owned at the URL, repo, site, or community level.

**Dependency:** T1.4 audit complete.

- **T2.1 — Pick neutral public name and domain.** Keep acronym "CAP"; decide neutral expansion (or drop expansion entirely), reserve domain, secure trademark-ready naming. DoD: ADR in `spec/` recording name + domain + rationale; domain registered.
- **T2.2 — Stop expanding CAP as "Cordum Agent Protocol" publicly.** Sweep repo, site, spec, SDK metadata, package manifests. DoD: grep for "Cordum Agent Protocol" returns zero public-facing hits; internal docs may retain historical usage.
- **T2.3 — Migrate docs to neutral docs site.** Stand up `docs.<neutral-domain>` with the full `spec/` + getting-started + sdk-comparison + troubleshooting. Redirect `cordum.io/protocol` → neutral docs. DoD: 301 redirects live, docs site has feature parity with current, canonical URLs updated.
- **T2.4 — Create neutral GitHub org for spec and governance.** Move `cap` repo (or fork + redirect) to the neutral org. Move governance repo, trademark repo, conformance repo. DoD: org created, repos moved, old `cordum-io/cap` redirects, CI green on new paths, Go module path migration plan documented.
- **T2.5 — Reframe Cordum site from "our protocol" to "Cordum implements CAP."** Copy rewrite on `cordum.io`, pricing page, homepage, docs cross-links. DoD: site diff reviewed; no "our protocol" / "Cordum's protocol" strings remain.
- **T2.6 — Move community surfaces to neutral branding.** Discord/Slack channel, forum, social accounts, demo videos. DoD: rename list executed; handles secured.

---

## EPIC 3 — Governance & Legal Package (Days 0–45)

**Goal:** Governance package is drafted and public before foundation outreach, so adopters can evaluate CAP's independence.

**Dependency:** T1.2 (counsel staffed), T2.1 (name).

- **T3.1 — Draft governance charter.** Scope, non-goals, membership, decision rules, conflict resolution. DoD: charter in governance repo at `/CHARTER.md`; reviewed by counsel.
- **T3.2 — Maintainer ladder.** Contributor → committer → maintainer → TSC; criteria, nomination process, removal process. DoD: `/MAINTAINERS.md` + `/GOVERNANCE.md`.
- **T3.3 — Spec change process.** RFC template, review SLA, compatibility class (wire-breaking vs additive vs editorial), required conformance-fixture delta. DoD: `spec/PROCESS.md`; first RFC PR goes through it end-to-end as a dogfooding exercise.
- **T3.4 — TSC trigger and composition rules.** When TSC forms, seat-allocation rules, voting thresholds, tiebreaker. DoD: section in `GOVERNANCE.md`.
- **T3.5 — Patent non-assert covenant.** Draft and publish. DoD: `/PATENTS.md` signed by Cordum as founding contributor.
- **T3.6 — "CAP-compatible" trademark policy.** Permitted uses, badge usage, conformance requirement, enforcement posture. DoD: `/TRADEMARK.md` + badge assets + usage examples.

---

## EPIC 4 — Apache Reference Kit (Days 15–60)

**Goal:** An Apache-2.0-only implementation proves interop without Cordum. Removes the BUSL licensing trap as an adoption blocker.

**Dependency:** T5 profile split (below) started; T3.6 badge drafted.

- **T4.1 — Reference kit scope doc.** Define what ships: golden packets, test vectors, minimal reference worker, minimal gateway/scheduler sample, conformance CLI, badge. List what it deliberately does NOT ship (auth, RBAC, workflow engine, Safety Kernel, Redis/NATS prod hardening). DoD: scope doc merged in neutral repo.
- **T4.2 — Golden packets + test vectors.** Extend existing `tools/conformance/generate_fixtures.go` output into a published fixture bundle with versioning, checksums, and per-profile coverage map. DoD: fixture bundle v1 tagged; all four SDKs decode it in CI.
- **T4.3 — Minimal reference worker runtime (Apache-2.0).** Standalone binary in neutral repo. Accepts JobRequest, emits heartbeat/progress/result. No Cordum imports. DoD: `go run` works against public NATS fixture; conformance CLI passes.
- **T4.4 — Minimal gateway/scheduler sample (Apache-2.0).** Standalone binary. Submits jobs, routes results. DoD: round-trip test against T4.3 worker passes in CI.
- **T4.5 — Conformance CLI + badge emitter.** `cap conformance run --impl <url>` produces a pass/fail report + signed badge JSON. DoD: CLI binary released; Cordum runs it in its CI and publishes the badge.
- **T4.6 — Normative vs product-behavior line.** Sweep `spec/` and annotate every clause with `[NORMATIVE]` or `[IMPLEMENTATION-NOTE]`. Move Cordum-specific behavior into a separate `docs/cordum-implementation-notes.md`. DoD: spec sweep merged; conformance CLI reports only against `[NORMATIVE]` clauses.

---

## EPIC 5 — Four-Profile Split (Days 15–60)

**Goal:** Implementation boundaries become obvious. Cordum's stack looks like one implementation, not the definition.

**Can run in parallel with Epic 4.**

- **T5.1 — Core wire profile.** Envelope (BusPacket), JobRequest, JobResult, JobProgress, JobCancel, JobStatus, Heartbeat. Minimum viable CAP. DoD: `spec/profiles/core.md` + conformance fixtures tagged `core`.
- **T5.2 — Transport bindings profile.** NATS binding first-class; HTTP, gRPC, Kafka as optional appendices with their own conformance fixtures. DoD: `spec/profiles/transport-nats.md` + placeholders for others.
- **T5.3 — Governance profile.** Policy-before-dispatch, approval binding, remediation hooks, constraints. DoD: `spec/profiles/governance.md` + fixtures showing a denied dispatch + approval round-trip.
- **T5.4 — Workflow profile.** Parent/child job semantics, DAG execution, compensation/rollback. DoD: `spec/profiles/workflow.md` + fixtures.
- **T5.5 — Profile conformance matrix.** Each SDK, each implementation, which profiles pass. DoD: matrix page on neutral docs site wired to CI badge output.

---

## EPIC 6 — Framework Adapters (Days 15–90)

**Goal:** Two credible framework-native adoption lanes. Each adapter is thin, does five things well, and ships with one real demo.

**Priority:** LangGraph → CrewAI → LlamaIndex (deferred) → AutoGen (compatibility lane only; maintenance-mode upstream).

**Per-adapter DoD (applies to T6.1 and T6.2):**
1. Submit governed jobs (CAP JobRequest → framework primitive).
2. Expose progress, heartbeat, cancel to the framework's event stream.
3. Surface approvals and policy denials cleanly (typed errors, not generic exceptions).
4. Emit interoperable traces and artifacts.
5. One real end-to-end demo repo with README, Docker-compose, <5-minute time-to-first-job.
6. Passes conformance CLI against the Apache reference kit.
7. No Cordum runtime required.

- **T6.1 — LangGraph adapter.** TypeScript + Python. Published as standalone package in neutral org. DoD: five capabilities above + demo repo + conformance pass.
- **T6.2 — CrewAI adapter.** Python. DoD: as above.
- **T6.3 — LlamaIndex adapter (deferred lane).** Stub + issue tracker entry; work starts only after T6.1 + T6.2 have one external adopter each.
- **T6.4 — AutoGen compatibility lane.** Minimal shim, documented as "compatibility only, not strategically prioritized due to upstream maintenance-mode status."
- **T6.5 — Adapter claims audit.** Sweep CAP README and site for adapter/pack claims that outrun reality; either upgrade asset to match claim or tighten claim to match asset. DoD: every public claim verifiable in ≤10 minutes by a new reader.

---

## EPIC 7 — Adopter Motions (Days 15–90)

**Goal:** One framework-native integration signed + one real product adopter signed. Both with founder-level engagement.

**Two parallel motions.**

- **T7.1 — Framework wedge: LangGraph or CrewAI design partnership.** Founder-owned outreach. Offer: engineering help, joint launch, governance seat. DoD: signed MOU or public joint announcement.
- **T7.2 — Product wedge: open-source agent/workflow product adopter.** Target shortlist of 5, outreach, land one. Offer: free implementation support, co-authored case study, roadmap input, founding-implementer recognition. DoD: signed design-partner agreement.
- **T7.3 — Standing offer template.** Productize the offer. DoD: `partnerships/FOUNDING_IMPLEMENTER.md` on neutral docs site with clear "apply here" path.
- **T7.4 — Founder calendar allocation.** Block founder time for first adopter conversations — not just engineering triage. DoD: standing weekly slot on founder's calendar labeled "CAP adopter motion."

---

## EPIC 8 — Proof Assets (Days 90–180)

**Goal:** Three assets that make adoption easy to justify.

- **T8.1 — Interoperability report v1.** Two or three independent implementations passing conformance. Report includes: implementations tested, profiles covered, failure modes found and fixed, gaps known. DoD: published on neutral docs site; referenced from all adapter READMEs.
- **T8.2 — Governance benchmark / demo.** Live, reproducible demo of: policy-before-dispatch, approval binding, rollback, audit. Not a pitch deck — a running system with a scripted scenario. DoD: public repo + hosted demo URL + 3-minute video walkthrough.
- **T8.3 — First adopter case study.** Co-authored with T7.1 or T7.2 partner. Focus on "CAP gave us enterprise governance without rewriting our runtime." DoD: case study published; partner-quoted; linked from neutral site + Cordum site.

---

## EPIC 9 — AAIF Foundation Path (Days 46–180)

**Goal:** AAIF as the primary neutral-home path. CNCF deferred as a later option for a runtime/reference implementation if one is ever opened.

**Dependency:** Epic 3 governance package drafted.

- **T9.1 — AAIF pre-brief.** Informal intro call with AAIF TSC contact. Share charter draft + interop status. Collect feedback. DoD: meeting notes + follow-up action items logged in governance repo.
- **T9.2 — Monthly public maintainer call.** Kick off a recurring public call; minutes published. DoD: first two calls executed; attendance and recording archive live.
- **T9.3 — AAIF proposal submission.** Formal proposal per AAIF project proposal process. DoD: proposal submitted; tracking issue linked in governance repo.
- **T9.4 — CNCF sandbox pre-assessment (optional, deferred).** Only start after T9.3 feedback; only pursue for a runtime/reference implementation if one exists as a standalone project.

---

## EPIC 10 — Kill-Criteria Review (Day 180 checkpoint)

**Goal:** Judge the bet hard at 6 months. If any kill criterion triggers, downgrade CAP from "standard" to "Cordum integration protocol" — still valuable, but not the same company bet.

- **T10.1 — 6-month review doc.** Scored against three kill criteria: (a) zero external implementers, (b) every spec change still effectively a Cordum product decision, (c) adopters keep needing Cordum-only behavior to pass "conformance." DoD: review memo + go/downgrade decision + communication plan.
- **T10.2 — Downgrade plan (contingent).** If any criterion triggers: roadmap rewrite, messaging revert, staffing reallocation, foundation conversation pause. DoD: plan ready-to-execute but unpublished; revisited at T10.1 checkpoint.

---

## EPIC 11 — Cordum Moat Above the Protocol Line (Ongoing)

**Goal:** Cordum wins by being the best CAP-native control plane, not by owning CAP. This epic tracks the moat work that already exists on Cordum's roadmap — listed here so it is visibly *separate* from CAP work and clearly positioned above the protocol line.

**Not newly scoped here — this is the existing Cordum roadmap, labeled for positioning clarity.**

- Enterprise auth + RBAC (cordum-enterprise)
- Policy authoring + change control
- Compliance packs
- Managed operations
- Deployment hardening
- Support, case studies, trust artifacts

DoD for labeling: each of these areas gets a one-line "above the CAP line" positioning note in its README.

---

## EPIC 12 — CAP Independent Web Presence (Days 0–60)

**Goal:** CAP has a standalone web presence that does not live as a tenant of `cordum.io`. Site, blog, and docs are visibly operated by the CAP project, not by Cordum.

**Dependency:** T2.1 (neutral name/domain chosen).

- **T12.1 — Marketing site architecture + stack decision.** Decide stack (Astro / Next / Hugo), hosting provider (under the neutral entity, not Cordum's Vercel/Cloudflare account), analytics posture (privacy-respecting), ownership of DNS. DoD: ADR merged; hosting account provisioned under the interim legal structure from Epic 15.
- **T12.2 — Homepage + positioning pages.** Homepage with the pitch; "Why CAP"; audience landings ("For framework authors / For platform teams / For enterprise"); "Compare" (vs MCP, A2A, OpenAgents — fair and specific); "Implementations" matrix; "Adopters" with logos + case-study slots; "Governance" (charter + maintainers + TSC + working-group membership); "Download / Getting started"; "Community"; "Conformance badge" explainer. DoD: all pages live; copy reviewed by T1.3 owner; "Cordum" appears on-site only in a single "founding implementer" line.
- **T12.3 — Blog infrastructure + editorial cadence.** Blog backend, RSS, author profiles, submission process for community posts. Editorial calendar with a minimum cadence (≥2 posts/month). Posts authored by CAP maintainers (includes Cordum engineers, but also independent maintainers once they exist). DoD: first 3 posts published; RSS live; calendar in governance repo.
- **T12.4 — Brand system.** Logo, wordmark, typography, color system, badge variants (conformant / founding-implementer / partner), presentation template, social card templates. DoD: brand package in neutral repo; Cordum site updated to use the new brand for CAP references.
- **T12.5 — SEO + discoverability.** Canonical URL strategy (all CAP content under neutral domain; `cordum.io` carries outbound links only), schema.org markup, sitemap, search submission. DoD: site indexed; "CAP protocol" / "agent protocol governance" queries return the CAP site in top results within 90 days post-launch.
- **T12.6 — Public status + roadmap page.** Single page showing current spec version, wire version, profile status, conformance matrix, open RFCs, release cadence. DoD: page live; auto-updated from repo state.

---

## EPIC 13 — CAP Publishing Program (Days 30–180)

**Goal:** A series of published materials that make the technical and strategic case for CAP credibly and independently of Cordum marketing. Papers are co-authored by CAP maintainers (Cordum engineers may be primary authors early, but the masthead expands as external maintainers join).

**Dependency:** T3.1 (charter), T5.1–T5.4 (profiles), T12.1 (site).

- **T13.1 — Whitepaper 1: "CAP: A Neutral Wire Protocol for Governed Agent Workloads."** Technical overview + design rationale. 12–15 pages; PDF + HTML. DoD: peer-reviewed by ≥2 external technical readers; published on CAP site; cited from `spec/README.md`.
- **T13.2 — Whitepaper 2: "Governance-as-Wire: Policy, Approval, and Audit at the Protocol Envelope."** Makes the case for the governance profile as a wire-level concern, not an application-level bolt-on. DoD: as T13.1.
- **T13.3 — Whitepaper 3: "CAP vs MCP vs A2A: A Comparison Matrix."** Fair, specific, non-snarky. Emphasizes complementarity where real (e.g., MCP is tool-facing; CAP is job/workload-facing). DoD: as T13.1; sent to MCP + A2A maintainers for factual-accuracy review before publication.
- **T13.4 — Whitepaper 4: "Conformance and Interoperability: How CAP Avoids Standards Fragmentation."** How the reference kit + conformance CLI + badge policy + spec-change process prevent drift. DoD: as T13.1.
- **T13.5 — Whitepaper 5: "Reference Architecture for CAP-Native Control Planes."** Lets other vendors see how to build a CAP control plane. Includes the Cordum implementation as **one** worked example among a template class (gateway pattern, scheduler pattern, safety/policy engine pattern, workflow engine pattern). DoD: as T13.1; Cordum shown as an instance, not the definition.
- **T13.6 — Adopter Playbook.** Practical guide: "From zero to CAP-compatible in 90 days." DoD: live on site; linked from every adapter README.
- **T13.7 — Explainer video + animation.** 3-minute homepage video. DoD: published on neutral YouTube channel; embedded on homepage.
- **T13.8 — Technical blog series (supporting the whitepapers).** ~2 blog posts per whitepaper (teaser + deep-dive). DoD: first 4 posts published alongside T13.1–T13.2 launch.

---

## EPIC 14 — CAP Launch & Developer Evangelism (Days 60–365)

**Goal:** CAP is visible, talked about, and easy to start using. Launch motion is not a single moment but a 180-day rolling cadence. DevRel presence is distinct from Cordum's growth marketing.

**Dependency:** T12.2 (site live), T13.1 (first paper), T6.1 **or** T6.2 (at least one adapter shippable), T15.* (custody settled).

- **T14.1 — Launch-day plan.** Coordinated: HN submission, X/Twitter thread, r/programming post, blog post, press list (The New Stack, InfoQ, Changelog, Latent Space, Hacker Newsletter), podcast pre-records lined up. DoD: runbook merged in governance repo; all assets staged; launch day locked; dry-run completed.
- **T14.2 — Podcast circuit.** Target: 6 appearances in the first 90 days post-launch (TWIML, Changelog, Latent Space, MLOps Community, AI Engineer, PracticalAI). DoD: bookings confirmed; appearances completed; transcripts linked from site.
- **T14.3 — Conference talks.** Target: 3 CFP acceptances in the first 180 days (KubeCon, QCon, AAIF events, AI Engineer Summit, Agentic AI Summit). DoD: talks accepted, delivered, recordings linked from site.
- **T14.4 — Community infrastructure.** Forum (Discourse or GitHub Discussions), chat (Matrix or Discord — under neutral identity, NOT Cordum's Slack), monthly office hours, issue-triage rotation. DoD: forum live; chat live; first 3 office hours held; triage rotation documented.
- **T14.5 — DevRel lead.** Dedicated DevRel owner (the DX engineer from T1.2 with protected time, or a new hire). Weekly cadence; monthly metrics report (GitHub stars, SDK downloads, Discord members, adapter pulls, conformance runs, whitepaper downloads). DoD: named owner; first monthly metrics report published.
- **T14.6 — "CAP in Production" speaker bureau.** Program that helps adopters and implementers (not just Cordum) speak at events about their CAP deployment. DoD: program page live; 2 external speakers enrolled.
- **T14.7 — Paid-media moratorium (explicit).** Document the 180-day no-paid-ads policy. Revisit at the 6-month review (ties into Epic 10). DoD: policy in governance repo; cross-referenced from marketing budget docs.
- **T14.8 — Community content grants (small).** Small stipend program for external tutorials, integrations, translations. DoD: grant page live; first 3 grants awarded.

---

## EPIC 15 — CAP Interim Legal Structure & Asset Custody (Days 15–120)

**Goal:** Until AAIF transfer completes, CAP has a clear interim legal structure that is visibly not Cordum-owned. Contributors and adopters can see *who* governs CAP in the meantime.

**Dependency:** T1.2 (counsel staffed), T2.1 (neutral name).

- **T15.1 — Interim Working Group charter.** Written charter for the CAP Working Group: membership, decision rules, IP handling, how it relates to Cordum (Cordum = founding member, not operator). DoD: charter merged in governance repo; signed by initial members; published on site governance page.
- **T15.2 — Trademark holding.** "CAP" (or neutral full-name) trademark registered under a neutral legal structure — fiscal sponsor (e.g., Open Collective, Software Freedom Conservancy), a new non-profit, or a counsel-recommended vehicle. Holder is NOT Cordum, Inc. DoD: trademark filed; assignment chain documented.
- **T15.3 — Domain + asset custody.** Domain, GitHub org, social handles, brand assets, demo repos, forum, chat — held by the interim structure, not Cordum. DoD: ownership transferred; audit trail documented in governance repo.
- **T15.4 — Contributor License Agreement (or DCO).** Decide and implement (DCO recommended for lowest friction). DoD: DCO or CLA bot configured on all repos; first external PR passes through it.
- **T15.5 — Financial transparency statement.** If Cordum funds the interim structure, that funding is disclosed publicly and updated quarterly. DoD: funding statement on governance page; first quarterly update scheduled.
- **T15.6 — Conflict-of-interest policy.** Policy for maintainers who are also employees of implementer companies (including Cordum). DoD: policy merged; maintainers acknowledge annually.

---

## Dependency Graph (top-level)

```
Epic 1 (decide + staff) ──┬─▶ Epic 2  (neutral branding: repos/domain/handles)
                          ├─▶ Epic 3  (governance package) ──▶ Epic 9 (AAIF)
                          ├─▶ Epic 15 (interim legal + asset custody) ──▶ Epic 12 (site) ──▶ Epic 14 (launch + DevRel)
                          └─▶ Epic 5  (profile split) ──┐
                                                        ├─▶ Epic 4  (Apache reference kit) ──▶ Epic 6 (adapters) ──▶ Epic 7 (adopter motions) ──▶ Epic 8 (proof assets)
                                                        │
                                                        └─▶ Epic 13 (whitepapers) ──▶ Epic 14 (launch + DevRel)

Epic 12 (site)   needs Epic 15 custody settled and Epic 2 naming decided.
Epic 13 (papers) needs Epic 5 profiles stable and Epic 12 site to host them.
Epic 14 (launch) needs Epic 12 site live, Epic 13 first paper, Epic 6 first adapter, Epic 15 custody settled.

All of Epics 4, 6, 7, 8, 12, 13, 14 feed ──▶ Epic 10 (kill-criteria review, Day 180).

Epic 11 (Cordum moat) runs in parallel; unblocked by all.
```

**Publishing vs technical tracks:**
- **Technical track:** E1 → E2 + E3 + E5 → E4 → E6 → E7 → E8
- **Publishing track:** E1 → E15 → E12 → E13 → E14
- The two tracks converge at Epic 14 (launch) and Epic 8 (proof assets), then both feed Epic 10 (kill-criteria review).

## Monday Checklist Mapping (from the plan)

| Monday item | Epic.Task |
|---|---|
| 1. Freeze CAP messaging edits | T1.3 |
| 2. Audit every CAP surface | T1.4 |
| 3. Decide neutral public name and domain | T2.1 |
| 4. Draft governance charter + legal package | T3.1–T3.6 + T15.1–T15.6 |
| 5. Define the four CAP profiles | T5.1–T5.4 |
| 6. Scope Apache reference kit + conformance CLI | T4.1, T4.5 |
| 7. Pick LangGraph and CrewAI as first adapter lanes | T6.1, T6.2 |
| 8. Build one governed demo per lane | part of T6.1 / T6.2 DoD + T8.2 |
| 9. Start AAIF intro conversations | T9.1 |
| 10. Put a founder in charge of first adopter conversation | T7.4 |

## Publishing / Independence Checklist (added after clarification)

| Independence requirement | Epic.Task |
|---|---|
| Standalone CAP site (not on cordum.io) | T12.1–T12.2 |
| Neutral GitHub org + domain + handles held by non-Cordum entity | T2.4 + T15.3 |
| Trademark held outside Cordum | T15.2 |
| Interim working group with written charter and signed members | T15.1 |
| Contributor License / DCO | T15.4 |
| Conflict-of-interest policy for Cordum-employed maintainers | T15.6 |
| Financial transparency on Cordum funding | T15.5 |
| First whitepaper peer-reviewed by external readers | T13.1 |
| Full whitepaper series (5 papers + playbook) | T13.1–T13.6 |
| Homepage explainer video | T13.7 |
| Launch-day runbook | T14.1 |
| ≥6 podcast appearances in 90 days | T14.2 |
| ≥3 conference talks in 180 days | T14.3 |
| Community forum + chat + monthly office hours | T14.4 |
| Public implementations matrix + conformance badge | T12.2 + T4.5 |
| Public status + roadmap page | T12.6 |
| No paid advertising for 180 days | T14.7 |

## Staffing Update (Implied by Epics 12–15)

The original Epic 1 staffing list (founder owner + spec editor + ecosystem engineer + DX engineer + part-time counsel) is **insufficient** for the publishing track. Add to T1.2:

- **Marketing / content lead** (full-time after Day 14) — owns E12 (site) and E13 (papers) editorial.
- **DevRel lead** (full-time after Day 60) — owns E14. Can start as the DX engineer with protected time, should become a dedicated role by launch.
- **Community ops** (part-time, Day 30+) — owns E14.4 + E14.6 + E14.8.
- **Legal / entity ops** (counsel from T1.2 extended) — owns E15 through the interim period.

This pushes CAP's dedicated headcount from ~4.5 FTE to ~6.5–7 FTE. That increase is the real cost of the standard-vs-feature bet. Flagging explicitly so it lands in the T1.1 decision memo rather than surfacing as a surprise in Month 3.

## Notes for Moe Intake

- File Epics 1–10 and Epics 12–15 as Moe epics; file T-items as tasks under each epic.
- Epic 11 should NOT be filed as a CAP epic — it's existing Cordum roadmap, cross-referenced only.
- Most T-items need an architect plan before a worker claims them. Epic 4, Epic 5, and Epic 12 tasks in particular are cross-subsystem and should enter Claude Code plan mode (≥2 of: 3+ subsystems, new pattern, 5+ DoD items).
- Adapter tasks (Epic 6) likely touch LangGraph/CrewAI source trees — may need upstream contribution path decided before planning.
- Trademark/patent/charter tasks (Epic 3) and interim-entity tasks (Epic 15) require counsel in the loop; architect plans must explicitly flag legal review as a step.
- Whitepaper tasks (Epic 13) need an editorial review workflow, not a code-review workflow; architect plans should include peer-review cycles and factual-accuracy sign-offs from named external readers.
- Launch tasks (Epic 14) need runbook-style plans with dry-runs, not code-style plans. Treat T14.1 like an ops change with rollback and coordination.
- Epic 15 tasks block Epic 12 and Epic 14 — do not approve launch-track plans that skip the custody-transfer and COI-policy gates.
