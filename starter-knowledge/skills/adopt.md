# adopt

Runs in an adopted track, before Gate 1. The task started before siding: bring it onto the rails without losing work.

1. Read every file already in the task docs folder (task lists, contracts, API notes, runbooks). Keep them; do not rename, split or rewrite them.
2. Per repo, read what the branch already holds: `git log --oneline <base>..HEAD` and `git diff --stat <base>...HEAD` in each worktree.
3. Reconcile: which items are done in code, which are only in the docs, which are on hold, which are blocked or waiting on a decision. Code wins over a stale doc; say where they disagree.
4. Write `prd.md` from the template for the **remaining** work only:
   - **Problem** and **Scope**: what is left, with links to the existing docs by file name.
   - **Out of scope**: items done already, and items on hold (keep their ids).
   - **Acceptance**: checks for the remaining work, plus pending manual steps the docs mention (migrations not yet run, config to apply).
   - **Open questions**: open decisions copied from the old docs, and every doc/code mismatch from step 3.
5. Run `siding submit <task>` and **stop**. From Gate 1 on, the usual skills apply. Keep updating the old docs where they are the contract others read (for example an API contract's changelog).
