package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// Changes of one repo in a track, split for review.
type Changes struct {
	Committed   string // base commit to HEAD
	Uncommitted string // HEAD to working tree, untracked files as new-file diffs
}

// Untracked files past these limits are listed by name only: a stray build dir must not stall the board.
const (
	maxUntrackedDiffs = 50
	maxUntrackedBytes = 256 << 10
)

// Changes returns a repo's committed and uncommitted diffs against the track's base commit.
func (w *Workspace) Changes(t *Track, repo string) (Changes, error) {
	for _, r := range t.Repos {
		if r.Name != repo {
			continue
		}
		dir := filepath.Join(w.trackDir(t.Task), r.Name)
		var c Changes
		var err error
		if c.Committed, err = git(dir, "diff", r.BaseCommit, "HEAD"); err != nil {
			return c, err
		}
		if c.Uncommitted, err = git(dir, "diff", "HEAD"); err != nil {
			return c, err
		}
		u, _ := git(dir, "ls-files", "--others", "--exclude-standard")
		var names []string
		for i, f := range lines(u) {
			if fi, err := os.Stat(filepath.Join(dir, f)); i >= maxUntrackedDiffs || err != nil || fi.Size() > maxUntrackedBytes {
				names = append(names, f)
				continue
			}
			// --no-index exits 1 when files differ, which is always here: the output is the result.
			d, _ := exec.Command("git", "-C", dir, "diff", "--no-index", "--", os.DevNull, f).Output()
			c.Uncommitted = strings.TrimSpace(c.Uncommitted + "\n" + strings.TrimSpace(string(d)))
		}
		if names != nil {
			c.Uncommitted += "\n\n# untracked, too many or too big to show:\n" + strings.Join(names, "\n")
		}
		return c, nil
	}
	return Changes{}, fmt.Errorf("track %s has no repo %s", t.Task, repo)
}

var hexSHA = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// Show returns one commit of a repo in a track, message and patch. Only commits in
// base..HEAD are shown, so the board cannot be used to read other history.
func (w *Workspace) Show(t *Track, repo, sha string) (string, error) {
	for _, r := range t.Repos {
		if r.Name != repo {
			continue
		}
		if !hexSHA.MatchString(sha) {
			return "", fmt.Errorf("not a commit sha: %q", sha)
		}
		dir := filepath.Join(w.trackDir(t.Task), r.Name)
		full, err := git(dir, "rev-parse", "--verify", "--quiet", sha+"^{commit}")
		if err != nil || full == r.BaseCommit {
			return "", fmt.Errorf("no commit %s in track %s/%s", sha, t.Task, repo)
		}
		if _, err := git(dir, "merge-base", "--is-ancestor", r.BaseCommit, full); err != nil {
			return "", fmt.Errorf("no commit %s in track %s/%s", sha, t.Task, repo)
		}
		if _, err := git(dir, "merge-base", "--is-ancestor", full, "HEAD"); err != nil {
			return "", fmt.Errorf("no commit %s in track %s/%s", sha, t.Task, repo)
		}
		return git(dir, "show", "--format=fuller", full)
	}
	return "", fmt.Errorf("track %s has no repo %s", t.Task, repo)
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
