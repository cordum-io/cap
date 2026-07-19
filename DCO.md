# Developer Certificate of Origin

CAP uses the **Developer Certificate of Origin (DCO) 1.1**. By signing off on your
commits you certify the statement below. The DCO is a lightweight, per-commit
attestation — **not** a Contributor License Agreement, copyright assignment, trademark
transfer, patent grant beyond the [Apache-2.0](LICENSE) license already in effect, or
any claim of independent governance or foundation affiliation.

## How to sign off

Add a `Signed-off-by` trailer to each commit using your real name and an email address
you control:

```
git commit -s -m "your message"
```

which appends:

```
Signed-off-by: Your Name <you@example.com>
```

The name and email in the trailer must match the commit author. If a change was
co-authored, **every** `Co-authored-by` person must also have a matching
`Signed-off-by` trailer.

Sign-off is required **prospectively** from the effective date stated in
[CONTRIBUTING.md](CONTRIBUTING.md). The project does not retroactively claim that older
commits were signed off, and does not rewrite history to add sign-offs.

## The certificate

The full text below is reproduced verbatim from <https://developercertificate.org/>.

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

## Enforcement

A source-controlled DCO check runs on pull requests and verifies that every
non-allowlisted commit in the PR range carries a valid `Signed-off-by` trailer matching
its author, and that co-author sign-offs are present. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the allowlist (release and dependency bots) and
the effective date. The check is advisory until a maintainer adds it to branch
protection after it first runs green; that ruleset change is a human admin action, not a
claim made in this file.
