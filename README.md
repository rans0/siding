# siding

One task, one track, across every repo it touches, with any agent.

siding is a workspace manager for coding agents. Each task gets its own track: a folder with one git worktree per repo it touches, a shared branch name, and a generated `AGENTS.md` telling the agent the rules, the repos and which skills to load at each stage. Main clones stay untouched, so parallel tasks never need `git stash` or `git checkout`.

The name comes from a railway siding: a side track where a train works without blocking the main line.

## Principles

- **The core is the CLI.** Any agent that can run shell commands can use siding. Harness adapters only add conveniences.
- **Plain files.** Config is TOML, task docs and knowledge are Markdown.
- **Git is the only hard dependency.**
- **Two human gates.** The agent stops after the PRD (Gate 1) and again with the uncommitted diff (Gate 2).

## Install

Requires Go 1.25+ and git. Linux and macOS.

```
go install github.com/rans0/siding/cmd/siding@latest
```

## Quick start

```
siding init ~/work && cd ~/work
siding repo add git@github.com:acme/backend.git
siding repo add git@github.com:acme/web.git
siding adapter install claude              # or: agents (instruction-only)

siding new add-promo backend,web           # tracks/add-promo/{backend,web}
cd tracks/add-promo && claude              # start your agent in the track
siding ls
siding done add-promo                      # after the work is pushed
```

Production hotfix from a tag, alongside other tracks on the same repo:

```
siding new fix-crash backend --hotfix --from v1.4.2
```

## Commands

| Command | What it does |
| --- | --- |
| `siding init [dir]` | Create a workspace |
| `siding repo add <url> [--ref-only]` | Clone into `repos/`; `--ref-only` repos can't be used in tracks |
| `siding repo ls` | Repos and the tracks using each |
| `siding new <task> <repo,repo> [--from ref\|a=x,b=y] [--hotfix]` | Create a track: worktrees, setup commands, `AGENTS.md` |
| `siding ls` | All tracks with uncommitted files and ahead/behind counts |
| `siding done <task> [--force]` | Remove the track's worktrees; task docs are kept |
| `siding adapter install <agents\|claude>` | Set up required tools and skills, write harness files into tracks |
| `siding doctor` | Check required tools, skills and plugins |

Safety behaviors:

- An existing branch (local or on `origin`) is reused, not recreated.
- The base commit is recorded in `.track.json` when a track is created.
- `new` reports failures per repo and keeps the repos that succeeded.
- `done` checks every repo before removing anything, and refuses while any has uncommitted changes or unpushed commits unless `--force` is given.

## Workspace layout

```
~/work/
├── siding.toml            # workspace config (commit this)
├── .siding.local.toml     # per machine: branch prefix, ref-only repos, adapters (gitignored)
├── repos/                 # main clones, never edited
├── tracks/<task>/         # AGENTS.md, .track.json, one worktree per repo
├── docs/<date>_<task>/    # prd.md, plan.md, testing.md, log.md; survives `done`
├── knowledge/
│   ├── rules/             # always loaded into AGENTS.md
│   ├── skills/            # loaded per stage: prd, workflow, tdd, review, hotfix, cross-repo, retro
│   ├── repos/<repo>.md    # per-repo facts, included in AGENTS.md
│   └── inbox/             # lesson proposals from retros
└── .siding/vendor/        # pinned upstream skills
```

## Configuration

```toml
[defaults]
base = "origin/main"
retro_max = 5

[repos.backend]
base = "origin/develop"        # optional, overrides defaults.base
setup = "go mod download"      # runs in each new worktree

[required.tools]
rtk = { check = "rtk init -g --show | grep -q 'Hook: rtk hook'", setup = "rtk init -g --auto-patch" }

[required.skills]
brainstorm = { source = "github:obra/superpowers", path = "skills/brainstorming", ref = "v6.4.2", stages = ["plan"] }
ponytail   = { source = "github:DietrichGebert/ponytail", path = "skills/ponytail", ref = "v4.10.0", stages = ["build", "review"], claude_plugin = "ponytail@ponytail" }
```

`.siding.local.toml`:

```toml
branch_prefix = "rans"         # branches become rans/<task>, rans/hotfix/<task>
ref_only = ["docs-site"]
```

Required skills are fetched at their pinned ref by `siding adapter install`. `siding new` refuses to create a track while any required tool or skill is missing.

## Harness support

| Harness | Support |
| --- | --- |
| Claude Code | `CLAUDE.md`, `/siding` slash command, hook blocking edits in `repos/`, ponytail plugin enabled inside tracks only |
| Any other agent | Instruction-only through `AGENTS.md` (`siding adapter install agents`) |

## Roadmap

- **v0.2:** `open` with tmux, port allocation, read-only web board (`siding ui`)
- **v0.3:** document templates, gate tracking, `report`, `pr`
- **v0.4:** `learn`, full `doctor`, stale-fact markers
- **v1.0:** tested Codex, Cursor and OpenCode adapters, native Windows, brew and scoop

## Development

```
go test ./...
```

## License

MIT, see [LICENSE](LICENSE).
