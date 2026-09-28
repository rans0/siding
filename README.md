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
siding open add-promo                      # agent in a tmux session (or foreground without tmux)
siding ui                                  # board at http://127.0.0.1:4747
siding ls                                  # agent submits at each gate
siding approve add-promo                   # you pass Gate 1, later Gate 2
siding pr add-promo                        # after Gate 2: commit, push, one PR per repo
siding done add-promo                      # after log.md is written
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
| `siding ls` | All tracks with stage, uncommitted files and ahead/behind counts |
| `siding submit <task>` | Agent: move the track to its next gate (Gate 1 needs `prd.md`) |
| `siding approve <task> [--back]` | You: pass the gate, or send the track back a stage |
| `siding open <task> [--agent cmd]` | Start the agent in the track, in tmux session `track-<task>` when tmux is installed |
| `siding report <task>` | Base, commits since base and diffstat per repo |
| `siding pr <task>` | After Gate 2: commit, push and open a PR per repo with `gh`; title and body from `prd.md` |
| `siding ui [--port N]` | Board on `127.0.0.1` (default port 4747): tracks by stage, per-track diff, docs, terminal snapshot, gate buttons, Inbox and Health |
| `siding done <task> [--force]` | Remove the track's worktrees once `log.md` exists; task docs are kept |
| `siding adapter install <agents\|claude>` | Set up required tools and skills, write harness files into tracks |
| `siding learn` | Review `knowledge/inbox/` one proposal at a time: accept, edit, skip or delete |
| `siding doctor` | Check required tools, skills and plugins, config, missing or orphaned worktrees, stale facts |

Safety behaviors:

- An existing branch (local or on `origin`) is reused, not recreated.
- The base commit is recorded in `.track.json` when a track is created.
- `new` reports failures per repo and keeps the repos that succeeded.
- `done` checks everything before removing anything, and refuses while `log.md` is missing or any repo has uncommitted changes or unpushed commits, unless `--force` is given.

Stages: `prd` → `GATE 1` → `build` → `GATE 2` → `ship`. The agent runs `siding submit` at each gate and stops; you run `siding approve`. In Claude Code a hook blocks the agent from running `siding approve` itself, and ponytail is enabled only in the build and review stages (applies from the next session).

The board keeps no state of its own and binds to `127.0.0.1` only. Gate buttons call the same code as `siding approve`, protected by a per-server token. The terminal view is a snapshot of the `track-<task>` tmux session refreshed every 2 seconds, not an interactive terminal.

## Learning loop

Before `done`, the agent's retro writes up to `retro_max` lesson proposals to `knowledge/inbox/`, one Markdown file each with `repo`, `track`, `date` and `commit` frontmatter. `siding learn` (or the board's Inbox) walks them:

```
[1/2] backend: integration tests need `scripts/db-up.sh` first
      source: add-promo, commit a1b2c3d
      (a)ccept  (e)dit  (s)kip  (d)elete >
```

Accepted facts are appended to `knowledge/repos/<repo>.md` with a `<!-- verified <commit> <track> <date> -->` marker, and every later track touching that repo gets them in its `AGENTS.md`. A proposal with `repo: skill:<name>` becomes `knowledge/skills/<name>.md`. `siding doctor` flags a fact as stale when a backticked path in it (containing `/`) no longer exists in the repo's main clone.

## Workspace layout

```
~/work/
├── siding.toml            # workspace config (commit this)
├── .siding.local.toml     # per machine: branch prefix, ref-only repos, adapters (gitignored)
├── repos/                 # main clones, never edited
├── tracks/<task>/         # AGENTS.md, .track.json, one worktree per repo
├── docs/<date>_<task>/    # prd.md, plan.md, testing.md, log.md; survives `done`
├── templates/             # starting points for those four docs
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
port_stride = 100

[repos.backend]
base = "origin/develop"        # optional, overrides defaults.base
setup = "go mod download"      # runs in each new worktree
env_ports = ["PORT=8080", "DB_PORT=5432"]

[required.tools]
rtk = { check = "rtk init -g --show", expect = "Hook: rtk hook", setup = "rtk init -g --auto-patch" }

[required.skills]
brainstorm = { source = "github:obra/superpowers", path = "skills/brainstorming", ref = "v6.4.2", stages = ["plan"] }
ponytail   = { source = "github:DietrichGebert/ponytail", path = "skills/ponytail", ref = "v4.10.0", stages = ["build", "review"], claude_plugin = "ponytail@ponytail" }
```

`.siding.local.toml`:

```toml
branch_prefix = "rans"         # branches become rans/<task>, rans/hotfix/<task>
ref_only = ["docs-site"]
agent = "claude"               # what `siding open` starts
```

Each track gets a port slot (the lowest free one). Its worktrees get a `.env`: the main clone's `.env` with every `env_ports` key set to `port + slot * port_stride`, plus `COMPOSE_PROJECT_NAME=track-<task>`. The generated `.env` is excluded from git in that worktree. So three tracks of `web` run on 3000, 3100 and 3200.

Required skills are fetched at their pinned ref by `siding adapter install`. `siding new` refuses to create a track while any required tool or skill is missing.

## Harness support

| Harness | Support |
| --- | --- |
| Claude Code | `CLAUDE.md`, `/siding` slash command, hook blocking edits in `repos/`, ponytail plugin enabled inside tracks only |
| Any other agent | Instruction-only through `AGENTS.md` (`siding adapter install agents`) |

## Roadmap

- **v1.0:** tested Codex, Cursor and OpenCode adapters, native Windows, brew and scoop

## Development

```
go test ./...
```

## License

MIT, see [LICENSE](LICENSE).
