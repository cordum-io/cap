# Decision record template

> Copy this block into [../DECISIONS.md](../DECISIONS.md) as a new `## D-NNNN:` section.
> Keep the field names exactly; the governance checker parses them. Never edit a prior
> entry's outcome — append a new one that supersedes it.

```
## D-NNNN: <short title>

- id: D-NNNN
- rfc: NNNN            # or none
- date: YYYY-MM-DD     # on or after the RFC's review-closes
- status: Accepted     # Accepted | Rejected | Superseded
- eligible-voters: @handle (Affiliation), @handle2 (Affiliation)
- recused: @handle (reason) ; or none
- quorum: 2/2 met      # cast / eligible-non-recused, and met|not-met
- tally: 2 for, 0 against
- rationale: <one or more sentences>
- minority-view: <dissent> ; or none
- supersedes: D-MMMM   # or none
- superseded-by: none  # set when a later decision replaces this
- links: PR #NN, rfcs/NNNN-title.md
```
