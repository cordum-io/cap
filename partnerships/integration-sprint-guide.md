# CAP Integration Sprint Guide (DRAFT)

DRAFT — DO NOT SEND OR PUBLISH WITHOUT EXPLICIT HUMAN APPROVAL

A bounded, time-boxed plan for helping a design partner run a real workload on CAP. It is
technical scaffolding, not a commitment, and is shared only after approval.

## Shape

- **Duration:** a fixed 2-week window, agreed up front. No open-ended obligation.
- **Goal:** one real (non-demo) workload dispatched over CAP against a reference or the
  partner's own scheduler, with progress/result flowing end to end.
- **Success criteria:** defined on day 1, measurable, and owned by the partner.

## Suggested timeline

| Day | Focus |
|-----|-------|
| 1 | Scope the workload and success criteria; confirm no cloud keys or secrets are shared. |
| 2–3 | Stand up SDK + transport (NATS) in the partner's environment. |
| 4–6 | Wire the real workload; map inputs/results through safe resource pointers. |
| 7–8 | Run the conformance TCK; capture the self-test report. |
| 9 | Exercise failure paths: duplicate delivery, cancellation, reconnect. |
| 10 | Retro; the partner decides independently whether to make any public statement. |

## Guardrails

- We provide engineering time and TCK guidance — not access to partner production systems,
  credentials, or data beyond what the integration technically requires.
- No telemetry or phone-home is introduced into the partner's environment.
- Any conformance result is the partner's to publish or not; it is a self-test, not a
  certification, and confers no endorsement.
- Nothing in the sprint grants governance rights or implies a maintainer seat.
