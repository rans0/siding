package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestLifecycle: init, repo add, new (with branch reuse), guard, done safety, done.
func TestLifecycle(t *testing.T) {
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")

	// upstream bare repo with one commit on main
	src := filepath.Join(tmp, "src")
	os.MkdirAll(src, 0o755)
	run(t, src, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644)
	run(t, src, "add", ".")
	run(t, src, "commit", "-qm", "init")
	run(t, tmp, "clone", "-q", "--bare", src, "app.git")

	ws := filepath.Join(tmp, "ws")
	if err := Init(ws); err != nil {
		t.Fatal(err)
	}
	if Init(ws) == nil {
		t.Fatal("second init should fail")
	}
	// no network in tests: drop required skills/tools
	cfg := "[defaults]\nbase = \"origin/main\"\n[repos.app]\nsetup = \"touch .setup-ran\"\n"
	os.WriteFile(filepath.Join(ws, "siding.toml"), []byte(cfg), 0o644)
	os.WriteFile(filepath.Join(ws, ".siding.local.toml"), []byte("branch_prefix = \"me\"\nadapters = [\"claude\"]\n"), 0o644)
	os.WriteFile(filepath.Join(ws, "knowledge/repos/app.md"), []byte("run `make test`"), 0o644)

	w, err := Open(filepath.Join(ws, "docs")) // walks up
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.RepoAdd(filepath.Join(tmp, "app.git"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.New("Bad Name", NewOpts{Repos: []string{"app"}}); err == nil {
		t.Fatal("invalid task name accepted")
	}
	if _, err := w.New("x", NewOpts{Repos: []string{"nope"}}); err == nil {
		t.Fatal("unknown repo accepted")
	}

	tr, err := w.New("feat", NewOpts{Repos: []string{"app"}})
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(ws, "tracks/feat/app")
	if tr.Repos[0].Branch != "me/feat" || run(t, wt, "branch", "--show-current") != "me/feat" {
		t.Fatalf("branch = %q", tr.Repos[0].Branch)
	}
	if tr.Repos[0].BaseCommit != run(t, src, "rev-parse", "HEAD") {
		t.Fatal("base commit not recorded")
	}
	if _, err := os.Stat(filepath.Join(wt, ".setup-ran")); err != nil {
		t.Fatal("setup did not run in worktree")
	}
	os.Remove(filepath.Join(wt, ".setup-ran"))
	agents, _ := os.ReadFile(filepath.Join(ws, "tracks/feat/AGENTS.md"))
	for _, want := range []string{"me/feat", "run `make test`", "Two gates", "knowledge/skills/prd.md"} {
		if !strings.Contains(string(agents), want) {
			t.Errorf("AGENTS.md missing %q:\n%s", want, agents)
		}
	}
	if strings.Contains(string(agents), "cross-repo") {
		t.Error("single-repo track should not load cross-repo")
	}
	if _, err := os.Stat(filepath.Join(ws, "tracks/feat/.claude/settings.json")); err != nil {
		t.Error("claude adapter files missing")
	}

	// guard blocks repos/, allows tracks/
	if w.Guard(strings.NewReader(`{"tool_input":{"file_path":"`+filepath.Join(ws, "repos/app/a.txt")+`"}}`)) == nil {
		t.Error("guard allowed edit in repos/")
	}
	if err := w.Guard(strings.NewReader(`{"tool_input":{"file_path":"` + filepath.Join(wt, "a.txt") + `"}}`)); err != nil {
		t.Error(err)
	}

	// done refuses dirty, then unpushed; succeeds once pushed
	os.WriteFile(filepath.Join(wt, "b.txt"), []byte("b"), 0o644)
	if err := w.Done("feat", false); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("want uncommitted refusal, got %v", err)
	}
	run(t, wt, "add", "b.txt")
	run(t, wt, "commit", "-qm", "b")
	if err := w.Done("feat", false); err == nil || !strings.Contains(err.Error(), "unpushed") {
		t.Fatalf("want unpushed refusal, got %v", err)
	}
	run(t, wt, "push", "-q", "origin", "me/feat")
	if err := w.Done("feat", false); err == nil || !strings.Contains(err.Error(), "log.md") {
		t.Fatalf("want missing log.md refusal, got %v", err)
	}
	os.WriteFile(filepath.Join(ws, tr.Docs, "log.md"), []byte("log"), 0o644)
	if err := w.Done("feat", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, "tracks/feat")); !os.IsNotExist(err) {
		t.Fatal("track not removed")
	}
	if _, err := os.Stat(filepath.Join(ws, tr.Docs)); err != nil {
		t.Fatal("docs removed")
	}

	// re-creating reuses the existing branch with its commit
	if _, err := w.New("feat", NewOpts{Repos: []string{"app"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt, "b.txt")); err != nil {
		t.Fatal("existing branch not reused")
	}
	if s := w.State(&Track{Task: "feat", Repos: []TrackRepo{{Name: "app", Base: "origin/main"}}}); s[0].Ahead != 1 {
		t.Fatalf("ahead = %d, want 1", s[0].Ahead)
	}
}

func TestParseFrom(t *testing.T) {
	m, err := parseFrom("a=v1,b=v2", []string{"a", "b"})
	if err != nil || m["a"] != "v1" || m["b"] != "v2" {
		t.Fatal(m, err)
	}
	if _, err := parseFrom("c=v1", []string{"a"}); err == nil {
		t.Fatal("unknown repo accepted")
	}
	if m, _ := parseFrom("v1.4.2", []string{"a", "b"}); m["b"] != "v1.4.2" {
		t.Fatal(m)
	}
}

// TestStages: submit/approve walk the gates; ponytail follows the stage; guard blocks approve.
func TestStages(t *testing.T) {
	ws := t.TempDir()
	w := &Workspace{Root: ws, Local: Local{Adapters: []string{"claude"}}}
	w.Cfg.Required.Skills = map[string]Skill{"ponytail": {Stages: []string{"build", "review"}, ClaudePlugin: "ponytail@ponytail"}}
	w.Cfg.Defaults.RetroMax = 5
	tr := &Track{Task: "s", Docs: "docs/s", Status: StagePRD}
	os.MkdirAll(filepath.Join(ws, "tracks/s"), 0o755)
	os.MkdirAll(filepath.Join(ws, "docs/s"), 0o755)
	if err := w.setStage(tr); err != nil {
		t.Fatal(err)
	}
	plugin := func() bool {
		var s struct{ EnabledPlugins map[string]bool }
		b, _ := os.ReadFile(filepath.Join(ws, "tracks/s/.claude/settings.json"))
		json.Unmarshal(b, &s)
		return s.EnabledPlugins["ponytail@ponytail"]
	}
	step := func(want string, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := w.LoadTrack("s"); got.Status != want {
			t.Fatalf("stage = %s, want %s", got.Status, want)
		}
	}
	if _, err := w.Submit("s"); err == nil {
		t.Fatal("submit without prd.md accepted")
	}
	if _, err := w.Approve("s", false); err == nil {
		t.Fatal("approve outside a gate accepted")
	}
	os.WriteFile(filepath.Join(ws, "docs/s/prd.md"), []byte("prd"), 0o644)
	_, err := w.Submit("s")
	step(StageGate1, err)
	if plugin() {
		t.Error("ponytail enabled before build")
	}
	_, err = w.Approve("s", true)
	step(StagePRD, err)
	w.Submit("s")
	_, err = w.Approve("s", false)
	step(StageBuild, err)
	if !plugin() {
		t.Error("ponytail disabled in build")
	}
	_, err = w.Submit("s")
	step(StageGate2, err)
	_, err = w.Approve("s", false)
	step(StageShip, err)
	agents, _ := os.ReadFile(filepath.Join(ws, "tracks/s/AGENTS.md"))
	if !strings.Contains(string(agents), "Stage: ship") {
		t.Error("AGENTS.md missing stage")
	}
	if w.Guard(strings.NewReader(`{"tool_input":{"command":"cd x && siding approve s"}}`)) == nil {
		t.Error("guard allowed agent approve")
	}
	if err := w.Guard(strings.NewReader(`{"tool_input":{"command":"siding submit s"}}`)); err != nil {
		t.Error(err)
	}
}
