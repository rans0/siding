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

Requires git. Linux, macOS and Windows.

```
brew install rans0/tap/siding                 # macOS, Linux
scoop bucket add rans0 https://github.com/rans0/scoop-bucket && scoop install siding   # Windows
go install github.com/rans0/siding/cmd/siding@latest                               # Go 1.25+
```

## Quick start

```
siding init ~/work && cd ~/work
siding repo add git@github.com:acme/backend.git
siding repo add git@github.com:acme/web.git
siding adapter install claude              # or: codex, cursor, opencode, agents

siding new add-promo backend,web           # tracks/add-promo/{backend,web}
siding open add-promo                      # agent in this terminal, also live on the board
siding ui                                  # board at http://127.0.0.1:4747
siding ls                                  # agent submits at each gate
siding approve add-promo                   # you pass Gate 1, later Gate 2
siding pr add-promo                        # after Gate 2: commit, push, one PR per repo
siding done add-promo                      # after log.md is written
```

## A task, start to finish

1. **Start.** `siding new add-promo backend,web`, then `siding open add-promo`. The agent reads `AGENTS.md` in the track: rules, repos, repo facts, and the skills for its stage.
2. **PRD.** The agent writes `docs/<date>_add-promo/prd.md`, runs `siding submit add-promo` and stops at Gate 1.
3. **Gate 1 (you).** Read the PRD on the board or in your editor. `siding approve add-promo` to continue, or `siding approve add-promo --back` and tell the agent what to change.
4. **Build.** The agent plans, writes tests and code in `tracks/add-promo/<repo>`, leaves the changes uncommitted, runs `siding submit add-promo` and stops at Gate 2.
5. **Gate 2 (you).** Review the diff on the board or with `siding report add-promo`. Approve, or send it back.
6. **Ship.** `siding pr add-promo` commits, pushes and opens one PR per repo.
7. **Retro.** The agent writes `log.md` and lesson proposals to `knowledge/inbox/`. Review them with `siding learn` or on the board's Inbox.
8. **Clean up.** After the PRs are merged, `siding done add-promo` removes the worktrees. Docs stay.

One branch for a whole epic (feature A ships, then B and C on the same branch): after A's PR, `siding next add-promo` starts the next cycle in the same track. A's `prd.md`, `plan.md` and `testing.md` move to `docs/<date>_add-promo/1/`, other docs (task lists, contracts) stay, and the base commit moves to the branch tip so `report` and the next PR cover only B. Run `done` once, when the epic is finished. If merging into the target branch conflicts, resolve it in a separate track instead of on the feature branch: `siding new rc-add-promo backend --from origin/develop --branch rc/add-promo`, `git merge feature/add-promo` in its worktree, then Gate 2 and `siding pr` as usual.

Later, to deep-review a finished task: `siding review add-promo` creates track `review-add-promo` on the latest base of the same repos, with the task's commit range and docs in `AGENTS.md`. The agent writes `review.md` (likely issues, repo patterns, PRD still met, performance, tech debt; each finding with severity, `file:line` and a fix), turns the findings into `prd.md` and stops at Gate 1. You pick what gets fixed; the fixes then follow the normal build, Gate 2 and PR flow. `done` keeps a `track.json` in the task docs so this works after the worktrees are gone.

Several tracks can run at once, each in its own terminal, on its own ports. If something looks wrong, run `siding doctor`.

### Adopting an existing workspace

Already have clones side by side and a `docs/` folder with work in progress? Start a fresh workspace and point it at the old one. The old workspace is only read, never changed.

```
siding init ~/work && cd ~/work
siding adopt ~/code/old-ws                   # copies every clone into repos/, with local branches and unpushed commits
siding new add-promo backend,web \
  --branch backend=feature/promo,web=wip/promo-ui \
  --docs ~/code/old-ws/docs/PROMO            # copied into docs/<date>_add-promo/ as is
```

`adopt` prints the branch each clone was on, and warns about uncommitted changes, which are not copied: commit them in the old clone first. Each copy's `origin` points at the real remote. A track with `--docs` starts at `prd` with the adopt skill: the agent reads the old docs and the commits already on the branches, writes `prd.md` for the remaining work, and stops at Gate 1. Copy finished docs anywhere under `docs/` (for example `docs/archive/`); siding ignores folders not named `<date>_<task>`.

Production hotfix from a tag, alongside other tracks on the same repo:

```
siding new fix-crash backend --hotfix --from v1.4.2
```

## Commands

| Command | What it does |
| --- | --- |
| `siding init [dir]` | Create a workspace |
| `siding repo add <url\|dir> [--ref-only]` | Clone into `repos/` (a local clone is copied with its branches); `--ref-only` repos can't be used in tracks |
| `siding adopt <old-workspace>` | Copy every clone in an old workspace into `repos/`; the old one is untouched |
| `siding repo ls` | Repos and the tracks using each |
| `siding new <task> <repo,repo> [--from ref\|a=x,b=y] [--hotfix] [--branch b\|a=x,b=y] [--docs dir]` | Create a track: worktrees, setup commands, `AGENTS.md`; `--branch` uses a branch name as is, `--docs` adopts existing docs |
| `siding ls` | All tracks with stage, uncommitted files and ahead/behind counts |
| `siding submit <task>` | Agent: move the track to its next gate (Gate 1 needs `prd.md`) |
| `siding approve <task> [--back]` | You: pass the gate, or send the track back a stage |
| `siding open <task> [--agent cmd]` | Start the agent in the track in any terminal; if it is already running, attach to it |
| `siding report <task>` | Base, commits since base and diffstat per repo |
| `siding pr <task>` | After Gate 2: commit, push and open a PR per repo with `gh`; title and body from `prd.md` |
| `siding next <task>` | After the PR merges: start the next cycle on the same branches; the cycle's prd/plan/testing move to a numbered docs folder |
| `siding completion zsh\|bash` | Tab completion with descriptions: commands, task names with their stage, repos, flags. brew installs it (zsh needs `autoload -Uz compinit && compinit` in `~/.zshrc`); otherwise add `source <(siding completion zsh)` after `compinit`, or `eval "$(siding completion bash)"` for bash |
| `siding ui [--port N]` | Board on `127.0.0.1` (default port 4747): tracks by stage, per-track diff, docs, live terminal, gate buttons, Inbox and Health |
| `siding review <task>` | Deep-review a finished task on the latest base: report first, then fix through the gates |
| `siding done <task> [--force]` | Remove the track's worktrees once `log.md` exists; task docs are kept |
| `siding adapter install <agents\|claude\|codex\|cursor\|opencode>` | Set up required tools and skills, write harness files into tracks |
| `siding learn` | Review `knowledge/inbox/` one proposal at a time: accept, edit, skip or delete |
| `siding doctor` | Check required tools, skills and plugins, config, missing or orphaned worktrees, stale facts |
| `siding version` | Print the version |

Safety behaviors:

- An existing branch (local or on `origin`) is reused, not recreated.
- The base commit is recorded in `.track.json` when a track is created.
- `new` reports failures per repo and keeps the repos that succeeded.
- `done` checks everything before removing anything, and refuses while `log.md` is missing or any repo has uncommitted changes or unpushed commits, unless `--force` is given.

Stages: `prd` → `GATE 1` → `build` → `GATE 2` → `ship`. The agent runs `siding submit` at each gate and stops; you run `siding approve`. In Claude Code, Codex, Cursor and OpenCode a hook blocks the agent from running `siding approve` itself, and ponytail is enabled only in the build and review stages (applies from the next session).

The board keeps no state of its own and binds to `127.0.0.1` only. Gate buttons call the same code as `siding approve`, protected by a per-server token. The track page shows the agent's terminal live (xterm.js) and you can type into it; the connection is refused from other origins and without the token.

`siding open` runs the agent in a pseudo-terminal owned by siding, so it works in any terminal app (Terminal, iTerm2, a VS Code panel) with no tmux. Run `siding open` again from another terminal to attach to the same session. The session ends when the agent exits or the terminal that started it closes. On Windows the agent runs directly in the terminal and the board shows no terminal.

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
├── siding.toml            # workspace config; `git init` the workspace to version it with knowledge/
├── .siding.local.toml     # per machine: branch prefix, ref-only repos, adapters (gitignored)
├── repos/                 # main clones, never edited
├── tracks/<task>/         # AGENTS.md, .track.json, one worktree per repo
├── docs/<date>_<task>/    # prd.md, plan.md, testing.md, log.md, track.json; survives `done`
├── templates/             # starting points for those four docs
├── knowledge/
│   ├── rules/             # always loaded into AGENTS.md
│   ├── skills/            # loaded per stage: prd, workflow, tdd, review, hotfix, cross-repo, retro, deep-review, adopt
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
| Codex | `.agents/skills/siding` skill, `.codex/hooks.json` guard (edits in `repos/`, `siding approve`) |
| Cursor | `/siding` command, `.agents/skills/siding` skill, `.cursor/hooks.json` guard |
| OpenCode | `/siding` command, `.opencode/plugins/siding-guard.js` guard |
| Any other agent | Instruction-only through `AGENTS.md` (`siding adapter install agents`) |

## Roadmap

- Sessions that outlive the terminal that started them
- Live board terminal on Windows (ConPTY)

## Development

```
go test ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for layout, cross-platform testing, adapters and releases.

## License

MIT, see [LICENSE](LICENSE). Bundles [xterm.js](https://xtermjs.org) (MIT, `web/xterm/LICENSE`).
