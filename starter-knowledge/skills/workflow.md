# workflow

Core loop between Gate 1 and Gate 2.

1. **Plan.** Write `plan.md` as vertical slices: each one a thin behavior that works end to end (through every layer and repo it needs) and can be tested on its own. Per slice: the behavior, files touched per repo, which slices it depends on. Order slices so each builds on finished ones; never a layer at a time.
2. **Edge cases.** List inputs and states that could break the plan. Add them to `plan.md`.
3. **Execute.** One slice at a time, test first (tdd skill). Update `plan.md` if reality differs.
4. **Check.** Run each repo's tests, linters and build. Record commands and results in `testing.md`. A failure you can't explain at a glance: follow the diagnose skill.
5. **Edge cases again.** Re-check the list from step 2 against the finished code.
6. **Summarize.** Run `siding submit <task>` and stop at Gate 2: report what changed per repo, what was tested, what is left. Do not commit.
