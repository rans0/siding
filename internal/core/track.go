package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/rans0/siding"
)

type Track struct {
	Task    string      `json:"task"`
	Hotfix  bool        `json:"hotfix,omitempty"`
	Created time.Time   `json:"created"`
	Docs    string      `json:"docs"` // relative to workspace root
	Status  string      `json:"status"`
	Slot    int         `json:"slot"` // port slot: every configured port + Slot*port_stride
	Repos   []TrackRepo `json:"repos"`
	Review  *Track      `json:"review,omitempty"` // the finished task this track reviews
}

type TrackRepo struct {
	Name       string `json:"name"`
	Branch     string `json:"branch"`
	Base       string `json:"base"`
	BaseCommit string `json:"base_commit"`
	Head       string `json:"head,omitempty"` // branch tip, recorded by done
}

type NewOpts struct {
	Repos  []string
	From   string // "ref" for all repos, or "a=x,b=y" per repo
	Hotfix bool
	Review *Track
}

func (w *Workspace) trackDir(task string) string { return w.path("tracks", task) }

func (w *Workspace) LoadTrack(task string) (*Track, error) {
	b, err := os.ReadFile(filepath.Join(w.trackDir(task), ".track.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no track %q", task)
	}
	if err != nil {
		return nil, err
	}
	var t Track
	return &t, json.Unmarshal(b, &t)
}

func (w *Workspace) saveTrack(t *Track) error {
	b, _ := json.MarshalIndent(t, "", "  ")
	return os.WriteFile(filepath.Join(w.trackDir(t.Task), ".track.json"), append(b, '\n'), 0o644)
}

func (w *Workspace) Tracks() ([]*Track, error) {
	entries, err := os.ReadDir(w.path("tracks"))
	if err != nil {
		return nil, err
	}
	var out []*Track
	for _, e := range entries {
		if t, err := w.LoadTrack(e.Name()); err == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

func (w *Workspace) branchFor(task string, hotfix bool) string {
	b := task
	if hotfix {
		b = "hotfix/" + task
	}
	if p := w.Local.BranchPrefix; p != "" {
		b = p + "/" + b
	}
	return b
}

func parseFrom(from string, repos []string) (map[string]string, error) {
	out := map[string]string{}
	if from == "" {
		return out, nil
	}
	if !strings.Contains(from, "=") {
		for _, r := range repos {
			out[r] = from
		}
		return out, nil
	}
	for _, pair := range strings.Split(from, ",") {
		r, ref, ok := strings.Cut(pair, "=")
		if !ok || !slices.Contains(repos, r) || ref == "" {
			return nil, fmt.Errorf("--from %q: %q is not repo=ref for a repo in this track", from, pair)
		}
		out[r] = ref
	}
	return out, nil
}

// New creates a track with one worktree per repo. Failures are reported per repo;
// repos that succeeded stay in the track.
func (w *Workspace) New(task string, o NewOpts) (*Track, error) {
	if err := validName("task", task); err != nil {
		return nil, err
	}
	if _, err := os.Stat(w.trackDir(task)); err == nil {
		return nil, fmt.Errorf("track %q already exists", task)
	}
	if len(o.Repos) == 0 {
		return nil, errors.New("name at least one repo: siding new <task> repo1,repo2")
	}
	if m := w.MissingRequired(); len(m) > 0 {
		return nil, fmt.Errorf("missing required: %s; run `siding adapter install <harness>`", strings.Join(m, ", "))
	}
	for _, r := range o.Repos {
		if _, err := os.Stat(w.path("repos", r, ".git")); err != nil {
			return nil, fmt.Errorf("repo %q not found in repos/; run `siding repo add <url>`", r)
		}
		if slices.Contains(w.Local.RefOnly, r) {
			return nil, fmt.Errorf("repo %q is reference-only", r)
		}
	}
	from, err := parseFrom(o.From, o.Repos)
	if err != nil {
		return nil, err
	}
	if o.Hotfix && o.From == "" {
		return nil, errors.New("--hotfix needs --from <production tag or ref>")
	}

	t := &Track{
		Task:    task,
		Hotfix:  o.Hotfix,
		Created: time.Now().UTC().Truncate(time.Second),
		Docs:    filepath.Join("docs", time.Now().Format("2006-01-02")+"_"+task),
		Status:  StagePRD,
		Review:  o.Review,
	}
	if t.Slot, err = w.freeSlot(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(w.trackDir(task), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(w.path(t.Docs), 0o755); err != nil {
		return nil, err
	}
	var errs []error
	for _, r := range o.Repos {
		tr, err := w.addWorktree(task, r, from[r], o.Hotfix, t.Slot)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r, err))
			continue
		}
		t.Repos = append(t.Repos, *tr)
	}
	if len(t.Repos) == 0 {
		os.RemoveAll(w.trackDir(task))
		return nil, errors.Join(errs...)
	}
	if err := w.saveTrack(t); err != nil {
		return t, err
	}
	if err := w.WriteAgentFiles(t); err != nil {
		errs = append(errs, err)
	}
	return t, errors.Join(errs...)
}

func (w *Workspace) addWorktree(task, repo, from string, hotfix bool, slot int) (*TrackRepo, error) {
	main := w.path("repos", repo)
	base := from
	if base == "" {
		base = w.Cfg.Repos[repo].Base
	}
	if base == "" {
		base = w.Cfg.Defaults.Base
	}
	branch := w.branchFor(task, hotfix)
	// Best effort: an offline fetch must not block local work.
	git(main, "fetch", "-q", "--tags", "origin")

	dest := filepath.Join(w.trackDir(task), repo)
	args := []string{"worktree", "add", "-q"}
	switch {
	case refExists(main, "refs/heads/"+branch):
		args = append(args, dest, branch) // reuse existing local branch
	case refExists(main, "refs/remotes/origin/"+branch):
		args = append(args, "--track", "-b", branch, dest, "origin/"+branch)
	default:
		args = append(args, "--no-track", "-b", branch, dest, base)
	}
	if _, err := git(main, args...); err != nil {
		return nil, err
	}
	// Base commit: where the branch forked from base (the tip, for a fresh branch).
	baseCommit, err := git(dest, "merge-base", "HEAD", base)
	if err != nil {
		baseCommit, _ = git(dest, "rev-parse", "HEAD")
	}
	if err := w.writeEnv(task, repo, dest, slot); err != nil {
		return &TrackRepo{Name: repo, Branch: branch, Base: base, BaseCommit: baseCommit}, fmt.Errorf("worktree created but .env failed: %w", err)
	}
	if setup := w.Cfg.Repos[repo].Setup; setup != "" {
		if err := sh(dest, setup); err != nil {
			return &TrackRepo{Name: repo, Branch: branch, Base: base, BaseCommit: baseCommit}, fmt.Errorf("worktree created but setup failed: %w", err)
		}
	}
	return &TrackRepo{Name: repo, Branch: branch, Base: base, BaseCommit: baseCommit}, nil
}

func refExists(dir, ref string) bool {
	_, err := git(dir, "show-ref", "--verify", "--quiet", ref)
	return err == nil
}

type RepoState struct {
	Name    string
	Changed int // uncommitted files
	Ahead   int // commits since base commit
	Behind  int // base commits not in branch
	Err     error
}

func (w *Workspace) State(t *Track) []RepoState {
	var out []RepoState
	for _, r := range t.Repos {
		s := RepoState{Name: r.Name}
		dir := filepath.Join(w.trackDir(t.Task), r.Name)
		porcelain, err := git(dir, "status", "--porcelain")
		if err != nil {
			s.Err = err
			out = append(out, s)
			continue
		}
		if porcelain != "" {
			s.Changed = strings.Count(porcelain, "\n") + 1
		}
		if lr, err := git(dir, "rev-list", "--left-right", "--count", r.Base+"...HEAD"); err == nil {
			behind, ahead, _ := strings.Cut(lr, "\t")
			s.Behind, _ = strconv.Atoi(behind)
			s.Ahead, _ = strconv.Atoi(ahead)
		}
		out = append(out, s)
	}
	return out
}

// Done removes a track's worktrees. All safety checks run before anything is removed.
func (w *Workspace) Done(task string, force bool) error {
	t, err := w.LoadTrack(task)
	if err != nil {
		return err
	}
	var problems []string
	for _, r := range t.Repos {
		dir := filepath.Join(w.trackDir(task), r.Name)
		if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if out, err := git(dir, "status", "--porcelain"); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", r.Name, err))
		} else if out != "" {
			problems = append(problems, r.Name+": uncommitted changes")
		}
		if out, err := git(dir, "log", "--oneline", "HEAD", "--not", "--remotes"); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", r.Name, err))
		} else if out != "" {
			problems = append(problems, fmt.Sprintf("%s: %d unpushed commit(s)", r.Name, strings.Count(out, "\n")+1))
		}
	}
	if _, err := os.Stat(w.path(t.Docs, "log.md")); err != nil {
		problems = append(problems, "no "+filepath.Join(t.Docs, "log.md")+" (run the retro skill first)")
	}
	if len(problems) > 0 && !force {
		return fmt.Errorf("refusing to remove track %s (use --force to discard):\n  %s", task, strings.Join(problems, "\n  "))
	}
	// Keep the track's record with its docs so `siding review` can find the commits later.
	for i, r := range t.Repos {
		t.Repos[i].Head, _ = git(filepath.Join(w.trackDir(task), r.Name), "rev-parse", "HEAD")
	}
	b, _ := json.MarshalIndent(t, "", "  ")
	if err := os.WriteFile(w.path(t.Docs, "track.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	for _, r := range t.Repos {
		dir := filepath.Join(w.trackDir(task), r.Name)
		args := []string{"worktree", "remove", dir}
		if force {
			args = []string{"worktree", "remove", "--force", dir}
		}
		if _, err := os.Stat(dir); err == nil {
			if _, err := git(w.path("repos", r.Name), args...); err != nil {
				return fmt.Errorf("%s: %w (track left in place)", r.Name, err)
			}
		}
		git(w.path("repos", r.Name), "worktree", "prune")
	}
	return os.RemoveAll(w.trackDir(task))
}

// Review starts track review-<task> on the latest base of the repos a finished task touched.
// The agent reports first (review.md), turns the findings into prd.md, then fixes through the usual gates.
func (w *Workspace) Review(task string) (*Track, error) {
	if err := validName("task", task); err != nil {
		return nil, err
	}
	found, _ := filepath.Glob(w.path("docs", "????-??-??_"+task, "track.json"))
	if len(found) == 0 {
		return nil, fmt.Errorf("no finished task %q (docs/*_%s/track.json is written by `siding done`)", task, task)
	}
	b, err := os.ReadFile(found[len(found)-1]) // latest date wins
	if err != nil {
		return nil, err
	}
	var orig Track
	if err := json.Unmarshal(b, &orig); err != nil {
		return nil, err
	}
	var repos []string
	for _, r := range orig.Repos {
		repos = append(repos, r.Name)
	}
	return w.New("review-"+task, NewOpts{Repos: repos, Review: &orig})
}

type skillRef struct{ Stage, Name, Path string }

// WriteAgentFiles generates AGENTS.md (and harness files for installed adapters) in the track.
func (w *Workspace) WriteAgentFiles(t *Track) error {
	type repoView struct {
		TrackRepo
		Path, Notes string
	}
	data := struct {
		Task, Root, Docs, Stage string
		Review                  *Track
		Hotfix                  bool
		RetroMax                int
		Repos                   []repoView
		Rules                   []string
		Skills                  []skillRef
	}{Task: t.Task, Stage: StageLabel[t.Status], Root: w.Root, Docs: w.path(t.Docs), Hotfix: t.Hotfix, Review: t.Review, RetroMax: w.Cfg.Defaults.RetroMax}

	for _, r := range t.Repos {
		notes, _ := os.ReadFile(w.path("knowledge", "repos", r.Name+".md"))
		data.Repos = append(data.Repos, repoView{r, filepath.Join(w.trackDir(t.Task), r.Name), strings.TrimSpace(string(notes))})
	}
	rules, _ := filepath.Glob(w.path("knowledge", "rules", "*.md"))
	for _, f := range rules {
		b, _ := os.ReadFile(f)
		data.Rules = append(data.Rules, strings.TrimSpace(string(b)))
	}
	data.Skills = w.skillsFor(t)

	src, _ := siding.Assets.ReadFile("templates/AGENTS.md.tmpl")
	tmpl := template.Must(template.New("agents").Parse(string(src)))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(w.trackDir(t.Task), "AGENTS.md"), buf.Bytes(), 0o644); err != nil {
		return err
	}
	for _, a := range w.Local.Adapters {
		if ad, ok := adapters[a]; ok {
			if err := ad.writeTrack(w, t); err != nil {
				return fmt.Errorf("adapter %s: %w", a, err)
			}
		}
	}
	return nil
}

func (w *Workspace) skillsFor(t *Track) []skillRef {
	k := func(name string) string { return w.path("knowledge", "skills", name+".md") }
	var out []skillRef
	add := func(stage, name, path string) {
		if _, err := os.Stat(path); err == nil {
			out = append(out, skillRef{stage, name, path})
		}
	}
	required := func(stage string) {
		names := make([]string, 0, len(w.Cfg.Required.Skills))
		for n := range w.Cfg.Required.Skills {
			names = append(names, n)
		}
		slices.Sort(names)
		for _, n := range names {
			s := w.Cfg.Required.Skills[n]
			if slices.Contains(s.Stages, stage) {
				add(stage, n, filepath.Join(w.vendorDir(n), s.Path, "SKILL.md"))
			}
		}
	}
	required("plan")
	if t.Review != nil {
		add("plan", "deep-review", k("deep-review"))
	}
	add("plan", "prd", k("prd"))
	add("build", "workflow", k("workflow"))
	add("build", "tdd", k("tdd"))
	if len(t.Repos) > 1 {
		add("build", "cross-repo", k("cross-repo"))
	}
	if t.Hotfix {
		add("build", "hotfix", k("hotfix"))
	}
	required("build")
	add("review", "review", k("review"))
	required("review")
	add("done", "retro", k("retro"))
	return out
}
