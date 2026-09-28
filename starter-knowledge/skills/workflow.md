# workflow

Core loop between Gate 1 and Gate 2.

1. **Plan.** Write `plan.md`: steps, files touched per repo, order of changes.
2. **Edge cases.** List inputs and states that could break the plan. Add them to `plan.md`.
3. **Execute.** Follow the plan. Update `plan.md` if reality differs.
4. **Check.** Run each repo's tests, linters and build. Record commands and results in `testing.md`.
5. **Edge cases again.** Re-check the list from step 2 against the finished code.
6. **Summarize.** Stop at Gate 2: report what changed per repo, what was tested, what is left. Do not commit.
