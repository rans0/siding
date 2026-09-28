# review

Before Gate 2, review the diff with fresh context (a subagent if your harness has one, else re-read from scratch).

Check, in order:
1. Correctness: logic errors, unhandled errors, edge cases from `plan.md`, security.
2. Scope: nothing outside `prd.md`.
3. Tests: new behavior covered, all green.
4. Over-engineering: unused abstractions, dead code, needless dependencies.

Fix every critical finding. List remaining minor ones in the Gate 2 summary.
