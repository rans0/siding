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

// Files lists each scope's files with counts; FileDiff serves one listed file, cut at maxDiffBytes.
func TestFiles(t *testing.T) {
	w, tr, wt := trackT(t)
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("committed\n"), 0o644)
	gitT(t, wt, "commit", "-qam", "first change")
	sha := gitT(t, wt, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("committed\nwip\n"), 0o644)
	os.WriteFile(filepath.Join(wt, "new.txt"), []byte("fresh\n"), 0o644)

	files, hidden, err := w.Files(tr, "app", ScopeUncommitted)
	want := []FileChange{{Path: "a.txt", Add: 1}, {Path: "new.txt", New: true}}
	if err != nil || hidden != 0 || len(files) != 2 || files[0] != want[0] || files[1] != want[1] {
		t.Fatalf("uncommitted: %+v %d %v", files, hidden, err)
	}
	for _, c := range []struct{ scope, path, has, not string }{
		{ScopeUncommitted, "a.txt", "+wip", "+committed"},
		{ScopeUncommitted, "new.txt", "+fresh", ""},
		{ScopeCommitted, "a.txt", "+committed", "wip"},
		{sha[:7], "a.txt", "+committed", "wip"},
	} {
		d, cut, err := w.FileDiff(tr, "app", c.scope, c.path)
		if err != nil || cut || !strings.Contains(d, c.has) || (c.not != "" && strings.Contains(d, c.not)) {
			t.Errorf("%s %s: %v cut=%v\n%s", c.scope, c.path, err, cut, d)
		}
	}
	for _, c := range [][2]string{
		{ScopeCommitted, "new.txt"}, // not in that scope
		{ScopeUncommitted, "../../siding.toml"},
		{tr.Repos[0].BaseCommit, "a.txt"}, // outside base..HEAD
		{"--output=/tmp/x", "a.txt"},
	} {
		if _, _, err := w.FileDiff(tr, "app", c[0], c[1]); err == nil {
			t.Errorf("%q accepted", c)
		}
	}
	if _, _, err := w.Files(tr, "nope", ScopeCommitted); err == nil {
		t.Error("unknown repo accepted")
	}

	defer func(b, n int) { maxDiffBytes, maxUntrackedFiles = b, n }(maxDiffBytes, maxUntrackedFiles)
	maxDiffBytes, maxUntrackedFiles = 64, 1
	os.WriteFile(filepath.Join(wt, "z.txt"), []byte(strings.Repeat("long line\n", 100)), 0o644)
	if files, hidden, _ := w.Files(tr, "app", ScopeUncommitted); len(files) != 2 || hidden != 1 {
		t.Errorf("cap: %+v hidden %d", files, hidden)
	}
	if d, cut, _ := w.FileDiff(tr, "app", ScopeUncommitted, "new.txt"); !cut || len(d) > 64 {
		t.Errorf("not cut: %v %d bytes", cut, len(d))
	}
}

// Show returns the message of a commit in base..HEAD; other commits are refused.
func TestShow(t *testing.T) {
	w, tr, wt := trackT(t)
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("one\n"), 0o644)
	gitT(t, wt, "commit", "-qam", "first change")
	sha := gitT(t, wt, "rev-parse", "HEAD")

	if m, err := w.Show(tr, "app", sha[:7]); err != nil || !strings.Contains(m, "first change") || strings.Contains(m, "+one") {
		t.Fatalf("show: %v\n%s", err, m)
	}
	for _, bad := range []string{tr.Repos[0].BaseCommit, "--output=/tmp/x", "HEAD", "zzzzzzz"} {
		if _, err := w.Show(tr, "app", bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
