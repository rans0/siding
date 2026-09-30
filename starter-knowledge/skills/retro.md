# retro

Runs once per cycle, after Gate 2: before `siding next` (which refuses until `log.md` changed this cycle) and before `siding done` (which refuses until `log.md` exists).

1. Write `log.md` in the task docs folder: what was built, decisions, anything surprising. On a track with several cycles, append a section per cycle (`## <feature>`); never rewrite earlier sections.
2. Propose at most N lessons (N is in AGENTS.md; zero is fine) as files in `knowledge/inbox/`, one per file, named `<date>_<task>_<n>.md`:

   ```
   ---
   repo: backend            # or "skill:<name>" to add to or create a skill, "rule:<name>" for a rule every task follows
   track: add-promo
   date: 2026-09-28
   commit: a1b2c3d          # commit where the fact was verified
   ---
   Integration tests need `make db-up` first, otherwise they time out.
   ```

3. Only propose facts a future task would not find quickly on its own. No task summaries.
   Pick the target: `rule:<name>` for a preference that applies to all code, `skill:<name>` for a stage-specific procedure, a repo name for a fact about that repo.
4. Collect every `ponytail:` comment in the diff into one proposal tagged as tech debt.
