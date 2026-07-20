# CAP Triage

Triage is the **first human response** to a new issue, pull request, or RFC — labeling
it, routing it, and (for RFCs) assigning a shepherd. Triage is **not** resolution; a
triaged item may stay open for a long time. The goal is that nothing sits unseen.

## The SLA

**Every new issue, PR, and RFC receives first human triage within 7 calendar days.**

- Triage means: remove the `needs-triage` label, apply routing labels, and — for RFCs —
  assign a shepherding maintainer. It does **not** mean the item is fixed, merged, or
  decided.
- Security reports are **out of band**: they are not filed as public issues (see below)
  and follow [../SECURITY.md](../SECURITY.md) response targets instead of this SLA.
- The SLA is measured from creation time to the first triage action, on a fixed clock so
  it can be audited deterministically (see the scheduled triage audit workflow).

## Labels

- `needs-triage` — applied automatically to every new public issue/PR; removed when
  triaged. This is the label the audit counts.
- Routing labels — area (`sdk-go`, `sdk-node`, `sdk-python`, `proto`, `spec`, `docs`,
  `governance`), type (`bug`, `enhancement`, `question`, `rfc`), and priority.

## Intake forms

Public intake uses GitHub issue forms so reports arrive structured and pre-labeled
`needs-triage`:

- **Bug report** — reproduction, versions, expected/actual.
- **Feature / proposal** — problem, proposed direction. Large or wire-affecting proposals
  are redirected to the [RFC process](../rfcs/README.md).
- **RFC intake** — pointer to open an RFC PR.
- **Maintainer nomination / adoption evidence** — structured evidence per
  [../MAINTAINERS.md](../MAINTAINERS.md) and the readiness rules.
- **Security** — the form config links to **private vulnerability reporting**; it does
  **not** accept a public security issue.

## Who triages

Any maintainer or committer may triage. While there is a single maintainer, triage is
that maintainer's responsibility; the 7-day SLA is deliberately generous to be
achievable by one person and is revisited as the roster grows.

## Auditing

A scheduled, read-only workflow lists open `needs-triage` items older than the SLA in its
job summary and fails visibly when any exist. It is a **visibility** check, not a
branch-required gate, and uses a fixed reference clock in tests so results are
reproducible.
