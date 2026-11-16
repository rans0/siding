# diagnose

For a bug, or a failure you can't explain at a glance. Find the cause before changing code.

1. **Loop.** Build the fastest deterministic way to see the failure: one test, one command, a script. Fix seeds, clocks and ports until it fails the same way every run. Under a second beats a full suite.
2. **Reproduce.** Run the loop; confirm it shows the reported symptom, not a different one.
3. **Hypothesize.** Write down one hypothesis and the check that would refute it. Run the check. Refuted: next hypothesis. Never change two things at once.
4. **Narrow.** Still lost: bisect (`git bisect run <loop>` across commits, or halve the input, config or code path) until the failing piece is small.
5. **Fix the cause**, not the symptom. A guard that hides a bad value is not a fix unless the value is legitimate.
6. **Regression test.** The loop from step 1 becomes a test that failed before the fix and passes after. Run the full suite.

Record in `testing.md`: symptom, cause, the regression test. If the cause was a trap a future task would hit, propose it in the retro.
