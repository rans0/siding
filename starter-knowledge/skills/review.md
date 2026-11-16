# review

Before Gate 2, review the diff with fresh context (a subagent if your harness has one, else re-read from scratch).

1. **Tools first.** Run each repo's tests, typecheck, linters and the tools named in the `review-*` checklists listed in `AGENTS.md`. Fix what they report; don't re-review it by hand.
2. **Read the context.** The repo's own `AGENTS.md` and the Notes for it in this track's `AGENTS.md` outrank any checklist. Read code next to each change so new code matches the repo's patterns.
3. **Review**, in order:
   1. Correctness: logic errors, unhandled errors, edge cases from `plan.md`.
   2. Each `review-*` checklist in `AGENTS.md` for the stacks this diff touches, including review-security; review-database only when the diff touches schema or queries.
   3. Scope: nothing outside `prd.md`.
   4. Tests: new behavior covered, all green.
   5. Over-engineering: unused abstractions, dead code, needless dependencies.
4. **Back every finding.** Severity (critical, important, minor), `file:line`, the concrete input or state that makes it fail, the fix. A checklist item that does not apply is not a finding; a finding without a failing scenario is dropped.
5. **Verify.** For each critical or important finding, try to refute it from the code (a second fresh subagent if available). Keep only what survives.

Fix every critical finding. List remaining ones in the Gate 2 summary. A finding the user rejects at Gate 2 as wrong for this repo becomes a retro proposal for that repo, so it is not raised again.
