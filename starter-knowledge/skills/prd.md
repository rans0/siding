# prd

Runs at `siding new`, before Gate 1.

Grill first. Before writing, interview the user until every branch of the design is decided:

- One question per turn, each with your recommended answer and why.
- Don't ask what the code, the docs or `knowledge/` can answer: read them instead.
- Use the terms in the glossary (in `AGENTS.md`, if any). When the user names a term differently, ask which word to keep.
- Stop when the remaining doubts would not change the scope or the acceptance checks.

Then write `prd.md` in the task docs folder with:

- **Problem**: what is wrong or missing, and for whom.
- **Scope**: what this task changes. **Out of scope**: what it does not.
- **Repos**: which repos change and why. **Base ref**: branch or tag to start from.
- **Acceptance**: observable checks that prove it is done.
- **Open questions**: only what waits on someone other than the user.

Then run `siding submit <task>` and **stop**. Do not write code, plans or branches until the user approves the PRD (Gate 1).
