package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type adapter struct {
	writeTrack func(w *Workspace, t *Track) error
}

// Generic AGENTS.md is always written; adapters add harness-specific files per track.
var adapters = map[string]adapter{
	"agents":   {writeTrack: func(*Workspace, *Track) error { return nil }},
	"claude":   {writeTrack: claudeTrack},
	"codex":    {writeTrack: codexTrack},
	"cursor":   {writeTrack: cursorTrack},
	"opencode": {writeTrack: opencodeTrack},
}

func AdapterNames() []string {
	names := make([]string, 0, len(adapters))
	for n := range adapters {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// AdapterInstall sets up required tools and skills, then enables the adapter for all tracks.
func (w *Workspace) AdapterInstall(name string) error {
	if _, ok := adapters[name]; !ok {
		return fmt.Errorf("unknown adapter %q (have: %s)", name, strings.Join(AdapterNames(), ", "))
	}
	var errs []error
	for n, t := range w.Cfg.Required.Tools {
		if t.ok(w.Root) {
			continue
		}
		if t.Setup == "" {
			errs = append(errs, fmt.Errorf("tool %s missing and has no setup", n))
			continue
		}
		if err := sh(w.Root, t.Setup); err != nil {
			errs = append(errs, fmt.Errorf("tool %s: %w", n, err))
		}
	}
	errs = append(errs, w.FetchSkills())
	if name == "claude" {
		errs = append(errs, w.installClaudePlugins())
	}
	if !slices.Contains(w.Local.Adapters, name) {
		w.Local.Adapters = append(w.Local.Adapters, name)
		errs = append(errs, w.saveLocal())
	}
	tracks, err := w.Tracks()
	errs = append(errs, err)
	for _, t := range tracks {
		errs = append(errs, w.WriteAgentFiles(t))
	}
	return errors.Join(errs...)
}

// commandBody is the /siding slash command shared by harnesses; args is how the harness
// passes what the user typed after the command.
func commandBody(task, args string) string {
	return "Run in the shell: `siding " + args + "`\n" +
		"If the command needs a task name and none was given, use \"" + task + "\".\n" +
		"Report the output. Never run `siding approve`: gates are the user's. Do not run `siding done` before Gate 2 is approved.\n"
}

const commandFront = "---\ndescription: Run a siding workspace command for this track\n"

// installClaudePlugins registers each vendored skill checkout as a marketplace (so the
// plugin version follows the pinned ref) and installs the plugin at project scope under
// .siding/, leaving it disabled everywhere except tracks, which enable it in their settings.
func (w *Workspace) installClaudePlugins() error {
	var errs []error
	for name, s := range w.Cfg.Required.Skills {
		if s.ClaudePlugin == "" {
			continue
		}
		dir := w.path(".siding")
		if _, err := run(dir, "claude", "plugin", "marketplace", "add", w.vendorDir(name)); err != nil {
			errs = append(errs, fmt.Errorf("claude plugin %s: %w", s.ClaudePlugin, err))
		} else if _, err := run(dir, "claude", "plugin", "install", s.ClaudePlugin, "--scope", "project"); err != nil {
			errs = append(errs, fmt.Errorf("claude plugin %s: %w", s.ClaudePlugin, err))
		}
	}
	return errors.Join(errs...)
}

// claudeTrack writes CLAUDE.md, a /siding slash command, and a PreToolUse guard for repos/.
func claudeTrack(w *Workspace, t *Track) error {
	dir := w.trackDir(t.Task)
	plugins := map[string]bool{}
	for _, s := range w.Cfg.Required.Skills {
		if s.ClaudePlugin != "" {
			// Claude Code reads this at session start: a stage change applies to the next session.
			plugins[s.ClaudePlugin] = slices.Contains(s.Stages, skillStage[t.Status])
		}
	}
	settings, _ := json.MarshalIndent(map[string]any{
		"enabledPlugins": plugins,
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": "Edit|MultiEdit|Write|NotebookEdit|Bash",
				"hooks":   []any{map[string]any{"type": "command", "command": "siding guard"}},
			}},
		},
	}, "", "  ")
	files := map[string]string{
		"CLAUDE.md":                  "@AGENTS.md\n",
		".claude/commands/siding.md": commandFront + "argument-hint: <ls|submit|report|done> [args]\nallowed-tools: Bash(siding:*)\n---\n" + commandBody(t.Task, "$ARGUMENTS"),
		".claude/settings.json":      string(settings) + "\n",
	}
	return writeFiles(dir, files)
}

func writeFiles(dir string, files map[string]string) error {
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func jsonFile(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b) + "\n"
}

// sidingSkill is an Agent Skills entry, read by Codex and Cursor from .agents/skills/.
func sidingSkill(task string) string {
	return "---\nname: siding\ndescription: Run siding workspace commands (ls, submit, report, done) for this track.\n---\n" +
		commandBody(task, "<command> [args]")
}

// codexTrack: Codex reads AGENTS.md itself. Adds the siding skill and a PreToolUse guard.
// Codex runs project hooks only after the user trusts them (/hooks).
func codexTrack(w *Workspace, t *Track) error {
	return writeFiles(w.trackDir(t.Task), map[string]string{
		".agents/skills/siding/SKILL.md": sidingSkill(t.Task),
		".codex/hooks.json": jsonFile(map[string]any{"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": "Bash|apply_patch|Edit|Write",
				"hooks":   []any{map[string]any{"type": "command", "command": "siding guard", "timeout": 30}},
			}},
		}}),
	})
}

// cursorTrack: Cursor reads AGENTS.md itself. Adds /siding and guards shell commands and
// tool calls; a hook exiting 2 denies the action.
func cursorTrack(w *Workspace, t *Track) error {
	guard := []any{map[string]any{"command": "siding guard"}}
	return writeFiles(w.trackDir(t.Task), map[string]string{
		".cursor/commands/siding.md":     commandBody(t.Task, "<what the user typed after /siding>"),
		".agents/skills/siding/SKILL.md": sidingSkill(t.Task),
		".cursor/hooks.json": jsonFile(map[string]any{"version": 1, "hooks": map[string]any{
			"beforeShellExecution": guard,
			"preToolUse":           guard,
		}}),
	})
}

// opencodePlugin passes each tool call to `siding guard`; throwing blocks the tool.
const opencodePlugin = `import { spawnSync } from "node:child_process"

// Generated by siding: blocks edits in repos/ and agent-run gate approvals.
export const SidingGuard = async () => ({
  "tool.execute.before": async (input, output) => {
    const r = spawnSync("siding", ["guard"], { input: JSON.stringify({ tool_input: output.args }) })
    if (r.status === 2) throw new Error(String(r.stderr).trim())
  },
})
`

// opencodeTrack: OpenCode reads AGENTS.md itself. Adds /siding and the guard plugin.
func opencodeTrack(w *Workspace, t *Track) error {
	return writeFiles(w.trackDir(t.Task), map[string]string{
		".opencode/commands/siding.md":      commandFront + "---\n" + commandBody(t.Task, "$ARGUMENTS"),
		".opencode/plugins/siding-guard.js": opencodePlugin,
	})
}

// Guard reads a PreToolUse payload (Claude Code, Codex, Cursor, or the OpenCode plugin) and
// returns an error if an edit targets the workspace's main clones, or a shell command tries
// to pass a gate.
func (w *Workspace) Guard(payload io.Reader) error {
	type input struct {
		FilePath     string `json:"file_path"`
		FilePathOC   string `json:"filePath"` // OpenCode
		NotebookPath string `json:"notebook_path"`
		Command      string `json:"command"` // shell command, or apply_patch text in Codex
		Input        string `json:"input"`   // Codex apply_patch, older shape
		PatchText    string `json:"patchText"`
	}
	var in struct {
		input
		ToolInput input `json:"tool_input"`
	}
	if err := json.NewDecoder(payload).Decode(&in); err != nil {
		return nil // not ours to judge
	}
	var cmds, paths []string
	for _, x := range []input{in.input, in.ToolInput} {
		cmds = append(cmds, x.Command, x.Input, x.PatchText)
		paths = append(paths, x.FilePath, x.FilePathOC, x.NotebookPath)
	}
	for _, c := range cmds {
		// ponytail: substring match, a determined agent can obfuscate; it guards against mistakes, not malice.
		if strings.Contains(c, "siding approve") {
			return fmt.Errorf("siding: gates are approved by the user, not the agent; run `siding submit` and stop")
		}
		paths = append(paths, patchPaths(c)...)
	}
	for _, p := range paths {
		if p != "" && w.inRepos(p) {
			return fmt.Errorf("siding: %s is in repos/ (main clones are read-only); edit the worktree under tracks/ instead", p)
		}
	}
	return nil
}

// patchPaths returns the files named by an apply_patch envelope ("*** Update File: x").
func patchPaths(patch string) []string {
	var out []string
	for _, l := range strings.Split(patch, "\n") {
		for _, pre := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
			if p, ok := strings.CutPrefix(strings.TrimSpace(l), pre); ok {
				out = append(out, p)
			}
		}
	}
	return out
}

func (w *Workspace) inRepos(p string) bool {
	p, _ = filepath.Abs(p)
	if d, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		p = filepath.Join(d, filepath.Base(p))
	}
	rel, err := filepath.Rel(w.path("repos"), p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
