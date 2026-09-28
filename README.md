# siding

One task, one track, across every repo it touches, with any agent.

See [docs/design.md](docs/design.md).

```
go install github.com/rans0/siding/cmd/siding@latest

siding init ~/work && cd ~/work
siding repo add <url>
siding adapter install claude      # rtk, pinned skills, per-track CLAUDE.md + guard hook
siding new add-promo backend,web   # worktrees in tracks/add-promo/, docs in docs/<date>_add-promo/
siding ls
siding done add-promo              # refuses while anything is uncommitted or unpushed
```

## Harness support

| Harness | Level |
| --- | --- |
| Claude Code | `CLAUDE.md`, `/siding` command, read-only guard on `repos/` |
| Anything else | Instruction-only via `AGENTS.md` (`siding adapter install agents`) |
