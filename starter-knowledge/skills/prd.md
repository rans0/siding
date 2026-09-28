# prd

Runs at `siding new`, before Gate 1.

Write `prd.md` in the task docs folder with:

- **Problem**: what is wrong or missing, and for whom.
- **Scope**: what this task changes. **Out of scope**: what it does not.
- **Repos**: which repos change and why. **Base ref**: branch or tag to start from.
- **Acceptance**: observable checks that prove it is done.
- **Open questions**.

Then run `siding submit <task>` and **stop**. Do not write code, plans or branches until the user approves the PRD (Gate 1).
