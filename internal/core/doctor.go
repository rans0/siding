package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Problem is one finding of Doctor.
type Problem struct{ Kind, Detail string }

// Doctor checks required tools and skills, config, worktrees and stale facts.
func (w *Workspace) Doctor() []Problem {
	var out []Problem
	add := func(kind, format string, a ...any) { out = append(out, Problem{kind, fmt.Sprintf(format, a...)}) }

	for _, m := range w.MissingRequired() {
		add("missing", "%s (run `siding adapter install <harness>`)", m)
	}

	onDisk := map[string]bool{}
	if repos, err := w.Repos(); err == nil {
		for _, r := range repos {
			onDisk[r.Name] = true
		}
	}
	for name, rc := range w.Cfg.Repos {
		if !onDisk[name] {
			add("config", "[repos.%s] in siding.toml but repos/%s is missing", name, name)
		}
		for _, p := range rc.EnvPorts {
			k, v, ok := strings.Cut(p, "=")
			if _, err := strconv.Atoi(v); !ok || k == "" || err != nil {
				add("config", "[repos.%s] env_ports entry %q is not KEY=port", name, p)
			}
		}
	}
	for _, name := range w.Local.RefOnly {
		if !onDisk[name] {
			add("config", "ref_only lists %s but repos/%s is missing", name, name)
		}
	}

	// worktrees: every track worktree git knows about must belong to a track, and back
	tracks, _ := w.Tracks()
	want := map[string]bool{}
	for _, t := range tracks {
		for _, r := range t.Repos {
			dir := filepath.Join(w.trackDir(t.Task), r.Name)
			want[dir] = true
			if _, err := os.Stat(dir); err != nil {
				add("worktree", "track %s: %s worktree is missing (recreate the track or `siding done %s --force`)", t.Task, r.Name, t.Task)
			}
		}
	}
	tracksDir := w.path("tracks") + string(filepath.Separator)
	for name := range onDisk {
		list, err := git(w.path("repos", name), "worktree", "list", "--porcelain")
		if err != nil {
			continue
		}
		for _, l := range strings.Split(list, "\n") {
			dir, ok := strings.CutPrefix(l, "worktree ")
			if !ok || !strings.HasPrefix(dir, tracksDir) || want[dir] {
				continue
			}
			add("worktree", "orphaned worktree %s of repos/%s (git -C repos/%s worktree remove --force %s)", dir, name, name, dir)
		}
	}

	for _, s := range w.StaleFacts() {
		add("stale", "%s", s)
	}
	return out
}

var pathRe = regexp.MustCompile("`([A-Za-z0-9_.-]+/[A-Za-z0-9_./-]*[A-Za-z0-9_-])`")

// StaleFacts flags lines in knowledge/repos/<repo>.md that name a path (`dir/file`) no
// longer present in the repo's main clone.
// ponytail: only slash-containing backticked paths are checked; bare file names and symbols
// are not, since they can't be told apart from commands. Add symbol lookup if facts rot unseen.
func (w *Workspace) StaleFacts() []string {
	files, _ := filepath.Glob(w.path("knowledge", "repos", "*.md"))
	var out []string
	for _, f := range files {
		repo := strings.TrimSuffix(filepath.Base(f), ".md")
		clone := w.path("repos", repo)
		if _, err := os.Stat(clone); err != nil {
			continue
		}
		b, _ := os.ReadFile(f)
		for i, l := range strings.Split(string(b), "\n") {
			for _, m := range pathRe.FindAllStringSubmatch(l, -1) {
				p := m[1]
				if strings.HasPrefix(p, "../") || slices.Contains(strings.Split(p, "/"), "..") {
					continue
				}
				if _, err := os.Stat(filepath.Join(clone, p)); err != nil {
					out = append(out, fmt.Sprintf("knowledge/repos/%s.md:%d: `%s` not found in repos/%s", repo, i+1, p, repo))
				}
			}
		}
	}
	return out
}
