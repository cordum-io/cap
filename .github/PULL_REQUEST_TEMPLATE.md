## What
<!-- Brief description of the change -->

## Why
<!-- Motivation / link to issue -->

## Checklist
- [ ] All commits are signed off (`git commit -s`) per the [DCO](../DCO.md); co-author sign-offs included
- [ ] Proto changes are append-only (no field renumbering)
- [ ] Spec updated to match proto changes (if applicable)
- [ ] All 3 SDKs updated in sync (if adding constants/types)
- [ ] Tests pass: `go test ./sdk/go/...`, `npm test`, `pytest`
- [ ] Conformance fixtures regenerated (if proto changed)
- [ ] CHANGELOG entry added (if user-facing)
