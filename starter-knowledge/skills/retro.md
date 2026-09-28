# retro

Runs before `siding done`.

1. Write `log.md` in the task docs folder: what was built, decisions, anything surprising.
2. Propose at most N lessons (N is in AGENTS.md; zero is fine) as files in `knowledge/inbox/`, one per file, named `<date>_<task>_<n>.md`:

   ```
   ---
   repo: backend            # or "skill:<name>" for a new skill
   track: add-promo
   date: 2026-09-28
   commit: a1b2c3d          # commit where the fact was verified
   ---
   Integration tests need `make db-up` first, otherwise they time out.
   ```

3. Only propose facts a future task would not find quickly on its own. No task summaries.
4. Collect every `ponytail:` comment in the diff into one proposal tagged as tech debt.
