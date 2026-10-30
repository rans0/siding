# dispatch

For the main session, started in the workspace root. This session is the dispatcher. It never writes code. Each task the user gives runs in its own track, worked by its own subagent, so several tasks can run at once without touching each other. The user approves every gate.

- Never edit anything under `repos/`. Never run `siding approve`: gates are the user's.
- One task, one track. Never put two tasks in one track: they would share worktrees and one Gate 2 diff.

## Start a task

1. Pick a short task name and the repos it touches (`siding repo ls`). Ask only if the repos are unclear.
2. `siding new <task> <repo,repo>`.
3. Spawn a worker subagent in the background with this brief, then return to the user:

   > You work track `<task>` in `tracks/<task>/`. Read `tracks/<task>/AGENTS.md` first and follow it; re-read it after each gate. The task: <the user's words>. Run every command and edit from inside the track. When you reach a gate, run `siding submit <task>` and stop with a short summary. When the workflow reaches the review skill, do not review yourself: stop and reply `ready for review`.

The user can give the next task while workers run. Start it the same way. Subagents start in the workspace root and do not load the track's `CLAUDE.md` or stage plugins; the brief and `AGENTS.md` carry what they need, and the root's guard still applies.

## Review

When a worker replies `ready for review`, spawn a reviewer subagent with fresh context:

> Review track `<task>`: read `tracks/<task>/AGENTS.md`, the task's `prd.md` and `plan.md`, then the review skill listed there. Review the uncommitted diff in each worktree under `tracks/<task>/`. Change nothing. Report findings as severity, `file:line`, what breaks, fix.

Pass the findings back to the worker (resume it if your harness can, else spawn a new worker with the same brief plus the findings). The worker fixes critical ones, lists the rest in its Gate 2 summary, runs `siding submit <task>` and stops.

## Gates

When a worker stops at a gate, tell the user in one line: task, gate, where to look (`siding ui`, or `siding report <task>`). After the user approves, resume that worker (or spawn a new one with the same brief); it re-reads `AGENTS.md` for the new stage. After `siding approve <task> --back`, pass the user's feedback with it.

## Report

When the user asks for a report, run `siding ls` and answer per track: stage, what the worker last said, what waits on the user.
