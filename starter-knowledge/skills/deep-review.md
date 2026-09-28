# deep-review

Runs in a `review-<task>` track, before Gate 1. You review a finished task as it lives now, on the latest base, with fresh eyes. Report first; fix nothing yet.

Read the task's `prd.md`, `plan.md` and `log.md`, then its commits (`git log -p <range>` in each worktree; ranges are in `AGENTS.md`). Follow each change into the current code, not just the diff.

Check, in order:

1. **Likely issues.** Nil or empty input, swallowed errors, races, leaked resources, timeouts, retries, partial failure. Callers the change breaks. Across repos: contracts still match on both sides.
2. **Repo patterns.** Compare with the surrounding code and `knowledge/repos/<repo>.md`: naming, error handling, layout, logging, tests, libraries already in use. New code should read like the code next to it.
3. **PRD.** Each acceptance item still holds in today's code: met, broken, or unverifiable (say why).
4. **Performance.** Queries or network calls in loops, nested loops over large data, allocations on hot paths, missing indexes.
5. **Tech debt.** Dead code, duplication, new TODOs, abstractions with one user, shortcuts without a noted limit.

Write `review.md` in the task docs folder. Per finding: severity (critical, important, minor), `file:line`, what goes wrong and when, the fix. Order by severity. Findings you can't back with a concrete scenario don't go in.

Then write `prd.md`: the findings to fix as scope, one acceptance check each; minor ones go out of scope unless trivial. Run `siding submit <task>` and **stop**. The user decides what gets fixed at Gate 1. From there the usual workflow and review skills apply, and fixes follow the repo's existing patterns.

If a finding is a lasting fact about a repo, propose it through the retro like any other lesson.
