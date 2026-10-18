package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type RepoReport struct {
	Name, Branch, Base, BaseCommit string
	Commits                        []string // oneline, newest first
	DiffStat                       string   // committed and uncommitted changes against the base commit
	Untracked                      []string
	Err                            error
}

// Report returns read-only facts per repo: base, commits since base, diffstat.
func (w *Workspace) Report(t *Track) []RepoReport {
	var out []RepoReport
	for _, r := range t.Repos {
		rep := RepoReport{Name: r.Name, Branch: r.Branch, Base: r.Base, BaseCommit: r.BaseCommit}
		dir := filepath.Join(w.trackDir(t.Task), r.Name)
		log, err := git(dir, "log", "--oneline", r.BaseCommit+"..HEAD")
		if err != nil {
			rep.Err = err
			out = append(out, rep)
			continue
		}
		rep.Commits = lines(log)
		rep.DiffStat, _ = git(dir, "diff", "--stat", r.BaseCommit)
		u, _ := git(dir, "ls-files", "--others", "--exclude-standard")
		rep.Untracked = lines(u)
		out = append(out, rep)
	}
	return out
}

// Scopes for Files and FileDiff; any other scope is a commit sha in the track.
const (
	ScopeUncommitted = "uncommitted" // HEAD to working tree, untracked files included
	ScopeCommitted   = "committed"   // base commit to HEAD
)

// FileChange is one file in a scope. Add and Del are 0 for binary and untracked files.
type FileChange struct {
	Path     string
	Add, Del int
	New      bool // untracked
	Binary   bool
}

// Limits that keep one huge diff or a stray build dir from stalling the board. Vars for tests.
var (
	maxDiffBytes      = 1 << 20 // per file diff
	maxUntrackedFiles = 200
)

var hexSHA = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// repoDir returns the worktree of repo in t.
func (w *Workspace) repoDir(t *Track, repo string) (string, TrackRepo, error) {
	for _, r := range t.Repos {
		if r.Name == repo {
			return filepath.Join(w.trackDir(t.Task), r.Name), r, nil
		}
	}
	return "", TrackRepo{}, fmt.Errorf("track %s has no repo %s", t.Task, repo)
}

// commitIn resolves sha to a full commit in base..HEAD, so the board cannot read other history.
func commitIn(dir string, r TrackRepo, sha string) (string, error) {
	bad := fmt.Errorf("no commit %q in this track", sha)
	if !hexSHA.MatchString(sha) {
		return "", bad
	}
	full, err := git(dir, "rev-parse", "--verify", "--quiet", sha+"^{commit}")
	if err != nil || full == r.BaseCommit {
		return "", bad
	}
	if _, err := git(dir, "merge-base", "--is-ancestor", r.BaseCommit, full); err != nil {
		return "", bad
	}
	if _, err := git(dir, "merge-base", "--is-ancestor", full, "HEAD"); err != nil {
		return "", bad
	}
	return full, nil
}

// diffRange returns the git diff arguments for a scope other than uncommitted.
func diffRange(dir string, r TrackRepo, scope string) ([]string, error) {
	switch scope {
	case ScopeUncommitted:
		return []string{"HEAD"}, nil
	case ScopeCommitted:
		return []string{r.BaseCommit, "HEAD"}, nil
	}
	full, err := commitIn(dir, r, scope)
	if err != nil {
		return nil, err
	}
	return []string{full + "^", full}, nil
}

// Files lists the files changed in a scope with line counts, without their diffs.
// hidden counts untracked files past maxUntrackedFiles.
func (w *Workspace) Files(t *Track, repo, scope string) (files []FileChange, hidden int, err error) {
	dir, r, err := w.repoDir(t, repo)
	if err != nil {
		return nil, 0, err
	}
	rng, err := diffRange(dir, r, scope)
	if err != nil {
		return nil, 0, err
	}
	out, err := gitOut(dir, append([]string{"diff", "--numstat", "-z", "--no-renames"}, rng...)...)
	if err != nil {
		return nil, 0, err
	}
	for _, rec := range strings.Split(out, "\x00") {
		add, rest, _ := strings.Cut(rec, "\t")
		del, path, ok := strings.Cut(rest, "\t")
		if !ok {
			continue
		}
		f := FileChange{Path: path, Binary: add == "-"}
		f.Add, _ = strconv.Atoi(add)
		f.Del, _ = strconv.Atoi(del)
		files = append(files, f)
	}
	if scope != ScopeUncommitted {
		return files, 0, nil
	}
	u, _ := gitOut(dir, "ls-files", "-z", "--others", "--exclude-standard")
	shown := 0
	for _, path := range strings.Split(u, "\x00") {
		switch {
		case path == "":
		case shown >= maxUntrackedFiles:
			hidden++
		default:
			shown++
			files = append(files, FileChange{Path: path, New: true})
		}
	}
	return files, hidden, nil
}

// FileDiff returns the diff of one file listed by Files, cut at maxDiffBytes (cut reports it).
// Only listed paths are served, so a request cannot reach files outside the change.
func (w *Workspace) FileDiff(t *Track, repo, scope, path string) (diff string, cut bool, err error) {
	files, _, err := w.Files(t, repo, scope)
	if err != nil {
		return "", false, err
	}
	dir, r, _ := w.repoDir(t, repo)
	rng, _ := diffRange(dir, r, scope)
	for _, f := range files {
		if f.Path != path {
			continue
		}
		args := append(append([]string{"diff"}, rng...), "--", ":(literal)"+path)
		if f.New {
			args = []string{"diff", "--no-index", "--", os.DevNull, path}
		}
		return gitCapped(dir, maxDiffBytes, args...)
	}
	return "", false, fmt.Errorf("%s is not changed in %s", path, scope)
}

// gitOut runs git for output that gets parsed: stdout only, so warnings such as
// autocrlf's "LF will be replaced by CRLF" cannot land inside a record.
func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// gitCapped streams git's stdout and stops at max bytes, cut at a line end, so a huge
// diff is never held in memory. The exit status is ignored: diff --no-index exits 1 on
// any difference and a git stopped early exits nonzero; callers validate args first.
func gitCapped(dir string, max int, args ...string) (string, bool, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", false, err
	}
	if err := cmd.Start(); err != nil {
		return "", false, err
	}
	b, _ := io.ReadAll(io.LimitReader(out, int64(max)+1))
	cut := len(b) > max
	if cut {
		b = b[:max]
		if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
			b = b[:i]
		}
		cmd.Process.Kill()
	}
	cmd.Wait()
	return strings.TrimRight(string(b), "\n"), cut, nil
}

// Show returns the header and message of one commit in the track.
func (w *Workspace) Show(t *Track, repo, sha string) (string, error) {
	dir, r, err := w.repoDir(t, repo)
	if err != nil {
		return "", err
	}
	full, err := commitIn(dir, r, sha)
	if err != nil {
		return "", err
	}
	return git(dir, "show", "-s", "--format=fuller", full)
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

type PRResult struct {
	Repo, URL, Note string
	Err             error
}

// PR commits, pushes and opens a pull request per repo; pushOnly stops after the push.
// Allowed only after Gate 2. Title is the first heading of prd.md; the body is prd.md itself.
func (w *Workspace) PR(task string, pushOnly bool) ([]PRResult, error) {
	t, err := w.LoadTrack(task)
	if err != nil {
		return nil, err
	}
	if t.Status != StageShip {
		return nil, fmt.Errorf("track %s is at %q; `siding pr` needs Gate 2 approved", task, StageLabel[t.Status])
	}
	if _, err := exec.LookPath("gh"); err != nil && !pushOnly {
		return nil, errors.New("gh not found; install GitHub CLI and run `gh auth login`")
	}
	prd := w.path(t.Docs, "prd.md")
	title := prTitle(prd, task)
	var out []PRResult
	for _, r := range t.Repos {
		res := PRResult{Repo: r.Name}
		res.URL, res.Note, res.Err = w.prRepo(t, r, title, prd, pushOnly)
		out = append(out, res)
	}
	return out, nil
}

func (w *Workspace) prRepo(t *Track, r TrackRepo, title, body string, pushOnly bool) (url, note string, err error) {
	dir := filepath.Join(w.trackDir(t.Task), r.Name)
	if st, err := git(dir, "status", "--porcelain"); err != nil {
		return "", "", err
	} else if st != "" {
		if _, err := git(dir, "add", "-A"); err != nil {
			return "", "", err
		}
		if _, err := git(dir, "commit", "-q", "-m", title); err != nil {
			return "", "", err
		}
		note = "committed; "
	}
	if n, _ := git(dir, "rev-list", "--count", r.BaseCommit+"..HEAD"); n == "0" {
		return "", "no changes, skipped", nil
	}
	if _, err := git(dir, "push", "-q", "-u", "origin", r.Branch); err != nil {
		return "", note, err
	}
	note += "pushed"
	if pushOnly {
		return "", note, nil
	}
	// Only an open PR counts: on a long-lived branch the earlier cycle's PR is already merged.
	if u, err := run(dir, "gh", "pr", "list", "--head", r.Branch, "--state", "open", "--json", "url", "--jq", ".[].url"); err == nil && u != "" {
		return lines(u)[0], note + "; PR already open", nil
	}
	args := []string{"pr", "create", "--head", r.Branch, "--title", title, "--body-file", body}
	if b, ok := strings.CutPrefix(r.Base, "origin/"); ok {
		args = append(args, "--base", b)
	}
	u, err := run(dir, "gh", args...)
	if err != nil {
		return "", note, err
	}
	return lastLine(u), note + "; PR opened", nil
}

func prTitle(prd, task string) string {
	b, err := os.ReadFile(prd)
	if err != nil {
		return task
	}
	for _, l := range lines(string(b)) {
		if h, ok := strings.CutPrefix(l, "# "); ok && !strings.HasPrefix(h, "<") {
			return strings.TrimSpace(h)
		}
	}
	return task
}

func lastLine(s string) string {
	l := lines(strings.TrimSpace(s))
	if len(l) == 0 {
		return ""
	}
	return l[len(l)-1]
}

func run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("%s %s: %s", name, args[0], s)
	}
	return s, nil
}
