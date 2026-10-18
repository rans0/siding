package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trackT returns a workspace with track "feat" on repo "app" and the worktree path.
func trackT(t *testing.T) (*Workspace, *Track, string) {
	t.Helper()
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "t")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "t@t")
	}
	src := filepath.Join(tmp, "src")
	os.MkdirAll(src, 0o755)
	gitT(t, src, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a\n"), 0o644)
	gitT(t, src, "add", ".")
	gitT(t, src, "commit", "-qm", "init")
	gitT(t, tmp, "clone", "-q", "--bare", src, "app.git")
	ws := filepath.Join(tmp, "ws")
	if err := Init(ws); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(ws, "siding.toml"), []byte("[defaults]\nbase = \"origin/main\"\n"), 0o644)
	w, err := Open(ws)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.RepoAdd(filepath.Join(tmp, "app.git"), false); err != nil {
		t.Fatal(err)
	}
	tr, err := w.New("feat", NewOpts{Repos: []string{"app"}})
	if err != nil {
		t.Fatal(err)
	}
	return w, tr, filepath.Join(ws, "tracks/feat/app")
}

// Committed and uncommitted work come apart; untracked files show their content.
func TestChanges(t *testing.T) {
	w, tr, wt := trackT(t)
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("committed\n"), 0o644)
	gitT(t, wt, "commit", "-qam", "c1")
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("committed\nwip\n"), 0o644)
	os.WriteFile(filepath.Join(wt, "new.txt"), []byte("fresh\n"), 0o644)

	c, err := w.Changes(tr, "app")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Committed, "+committed") || strings.Contains(c.Committed, "wip") {
		t.Errorf("committed:\n%s", c.Committed)
	}
	if !strings.Contains(c.Uncommitted, "+wip") || strings.Contains(c.Uncommitted, "+committed") {
		t.Errorf("uncommitted:\n%s", c.Uncommitted)
	}
	if !strings.Contains(c.Uncommitted, "+++ b/new.txt") || !strings.Contains(c.Uncommitted, "+fresh") {
		t.Errorf("untracked content missing:\n%s", c.Uncommitted)
	}
	if _, err := w.Changes(tr, "nope"); err == nil {
		t.Error("unknown repo accepted")
	}
}

// Show returns one commit of the track; anything outside base..HEAD is refused.
func TestShow(t *testing.T) {
	w, tr, wt := trackT(t)
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("one\n"), 0o644)
	gitT(t, wt, "commit", "-qam", "first change")
	sha := gitT(t, wt, "rev-parse", "HEAD")

	d, err := w.Show(tr, "app", sha[:7])
	if err != nil || !strings.Contains(d, "first change") || !strings.Contains(d, "+one") {
		t.Fatalf("show: %v\n%s", err, d)
	}
	for _, bad := range []string{tr.Repos[0].BaseCommit, "--output=/tmp/x", "HEAD", "zzzzzzz"} {
		if _, err := w.Show(tr, "app", bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
