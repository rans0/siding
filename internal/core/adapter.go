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
	"agents": {writeTrack: func(*Workspace, *Track) error { return nil }},
	"claude": {writeTrack: claudeTrack},
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
		if t.Check != "" && sh(w.Root, t.Check) == nil {
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

const claudeCommand = `---
description: Run a siding workspace command for this track
argument-hint: <ls|submit|done> [args]
allowed-tools: Bash(siding:*)
---
Run in the shell: ` + "`siding $ARGUMENTS`" + `
If the command needs a task name and none was given, use "%s".
Report the output. Never run ` + "`siding approve`" + `: gates are the user's. Do not run ` + "`siding done`" + ` before Gate 2 is approved.
`

// installClaudePlugins registers each vendored skill checkout as a marketplace (so the
// plugin version follows the pinned ref) and installs the plugin at project scope under
// .siding/, leaving it disabled everywhere except tracks, which enable it in their settings.
func (w *Workspace) installClaudePlugins() error {
	var errs []error
	for name, s := range w.Cfg.Required.Skills {
		if s.ClaudePlugin == "" {
			continue
		}
		cmd := fmt.Sprintf("claude plugin marketplace add %q && claude plugin install %q --scope project", w.vendorDir(name), s.ClaudePlugin)
		if err := sh(w.path(".siding"), cmd); err != nil {
			errs = append(errs, fmt.Errorf("claude plugin %s: %w", s.ClaudePlugin, err))
		}
	}
	return errors.Join(errs...)
}

// claudeTrack writes CLAUDE.md, a /siding slash command, and a PreToolUse guard for repos/.
func claudeTrack(w *Workspace, t *Track) error {
	dir := w.trackDir(t.Task)
	if err := os.MkdirAll(filepath.Join(dir, ".claude", "commands"), 0o755); err != nil {
		return err
	}
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
		".claude/commands/siding.md": fmt.Sprintf(claudeCommand, t.Task),
		".claude/settings.json":      string(settings) + "\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Guard reads a Claude Code PreToolUse payload and returns an error if the edit
// targets the workspace's main clones, or a shell command tries to pass a gate.
func (w *Workspace) Guard(payload io.Reader) error {
	var in struct {
		ToolInput struct {
			FilePath     string `json:"file_path"`
			NotebookPath string `json:"notebook_path"`
			Command      string `json:"command"`
		} `json:"tool_input"`
	}
	if err := json.NewDecoder(payload).Decode(&in); err != nil {
		return nil // not ours to judge
	}
	// ponytail: substring match, a determined agent can obfuscate; it guards against mistakes, not malice.
	if strings.Contains(in.ToolInput.Command, "siding approve") {
		return fmt.Errorf("siding: gates are approved by the user, not the agent; run `siding submit` and stop")
	}
	p := in.ToolInput.FilePath + in.ToolInput.NotebookPath
	if p == "" {
		return nil
	}
	p, _ = filepath.Abs(p)
	if d, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		p = filepath.Join(d, filepath.Base(p))
	}
	if rel, err := filepath.Rel(w.path("repos"), p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("siding: %s is in repos/ (main clones are read-only); edit the worktree under tracks/ instead", p)
	}
	return nil
}
