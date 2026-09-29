// Package core is the siding library shared by the CLI (and later the board UI).
package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/rans0/siding"
)

type Config struct {
	Defaults struct {
		Base       string `toml:"base"`
		PortStride int    `toml:"port_stride"`
		RetroMax   int    `toml:"retro_max"`
	} `toml:"defaults"`
	Repos    map[string]RepoConfig `toml:"repos"`
	Required struct {
		Tools  map[string]Tool  `toml:"tools"`
		Skills map[string]Skill `toml:"skills"`
	} `toml:"required"`
}

type RepoConfig struct {
	Base     string   `toml:"base"`
	Setup    string   `toml:"setup"`
	EnvPorts []string `toml:"env_ports"`
}

type Tool struct {
	Check  string `toml:"check"`
	Expect string `toml:"expect"` // optional: check passes only if its output contains this
	Setup  string `toml:"setup"`
}

// ok runs the tool's check. No check means nothing to verify.
func (t Tool) ok(dir string) bool {
	if t.Check == "" {
		return true
	}
	cmd := shell(t.Check)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return err == nil && strings.Contains(string(out), t.Expect)
}

type Skill struct {
	Source string   `toml:"source"`
	Path   string   `toml:"path"`
	Ref    string   `toml:"ref"`
	Stages []string `toml:"stages"`
	// ClaudePlugin, e.g. "ponytail@ponytail": the vendored checkout is also a Claude Code
	// marketplace, and the plugin (with its hooks) is enabled inside tracks only.
	ClaudePlugin string `toml:"claude_plugin"`
}

// Local is .siding.local.toml: per-machine, gitignored.
type Local struct {
	BranchPrefix string   `toml:"branch_prefix"`
	RefOnly      []string `toml:"ref_only"`
	Adapters     []string `toml:"adapters"`
	Agent        string   `toml:"agent"` // command `siding open` starts; default "claude"
}

type Workspace struct {
	Root  string
	Cfg   Config
	Local Local
}

const (
	configFile = "siding.toml"
	localFile  = ".siding.local.toml"
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func validName(kind, s string) error {
	if !nameRe.MatchString(s) {
		return fmt.Errorf("invalid %s name %q: use lowercase letters, digits, '.', '_', '-'", kind, s)
	}
	return nil
}

// Init creates a workspace in dir.
func Init(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, configFile)); err == nil {
		return fmt.Errorf("%s already has a %s", dir, configFile)
	}
	for _, d := range []string{"repos", "tracks", "docs", "knowledge/repos", "knowledge/inbox"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return err
		}
	}
	starter, _ := fs.Sub(siding.Assets, "starter-knowledge")
	if err := os.CopyFS(filepath.Join(dir, "knowledge"), starter); err != nil {
		return err
	}
	docTemplates, _ := fs.Sub(siding.Assets, "templates/docs")
	if err := os.CopyFS(filepath.Join(dir, "templates"), docTemplates); err != nil {
		return err
	}
	tmpl, _ := siding.Assets.ReadFile("templates/siding.toml")
	files := map[string]string{
		configFile:                 string(tmpl),
		localFile:                  "# per machine, gitignored\nbranch_prefix = \"\"   # e.g. \"rans\" gives branches rans/<task>\nref_only = []\n",
		"knowledge/inbox/.gitkeep": "",
		"knowledge/repos/.gitkeep": "",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	// Append, not overwrite: init may adopt a folder that already has a .gitignore.
	gi, err := os.OpenFile(filepath.Join(dir, ".gitignore"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	// No git init: versioning the workspace is the user's choice; .gitignore is ready for it.
	_, err = gi.WriteString(localFile + "\nrepos/\ntracks/\n.siding/\n")
	gi.Close()
	return err
}

// Open finds the workspace containing dir by walking up to siding.toml.
func Open(dir string) (*Workspace, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, configFile)); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errors.New("not inside a siding workspace (no siding.toml found); run `siding init <dir>`")
		}
		dir = parent
	}
	w := &Workspace{Root: dir}
	if _, err := toml.DecodeFile(filepath.Join(dir, configFile), &w.Cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", configFile, err)
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, localFile), &w.Local); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", localFile, err)
	}
	if w.Cfg.Defaults.Base == "" {
		w.Cfg.Defaults.Base = "origin/main"
	}
	if w.Cfg.Defaults.RetroMax == 0 {
		w.Cfg.Defaults.RetroMax = 5
	}
	return w, nil
}

func (w *Workspace) saveLocal() error {
	f, err := os.Create(filepath.Join(w.Root, localFile))
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(w.Local)
}

func (w *Workspace) path(parts ...string) string {
	return filepath.Join(append([]string{w.Root}, parts...)...)
}

// RepoAdd clones url into repos/. A local clone (e.g. from an old workspace) is copied with
// all its local branches, including unpushed commits, and origin pointed at its real remote;
// the source is left untouched. note names the branch it was on.
func (w *Workspace) RepoAdd(url string, refOnly bool) (name, note string, err error) {
	name = strings.TrimSuffix(filepath.Base(strings.TrimRight(url, "/")), ".git")
	name = strings.ToLower(name[strings.LastIndex(name, ":")+1:])
	if err := validName("repo", name); err != nil {
		return "", "", err
	}
	dest := w.path("repos", name)
	if _, err := os.Stat(dest); err == nil {
		return "", "", fmt.Errorf("repos/%s already exists", name)
	}
	if _, err := os.Stat(filepath.Join(url, ".git")); err == nil {
		if note, err = copyClone(url, dest); err != nil {
			os.RemoveAll(dest)
			return "", "", err
		}
	} else if _, err := git(w.path("repos"), "clone", "-q", url, name); err != nil {
		return "", "", err
	}
	if refOnly && !slices.Contains(w.Local.RefOnly, name) {
		w.Local.RefOnly = append(w.Local.RefOnly, name)
		return name, note, w.saveLocal()
	}
	return name, note, nil
}

// copyClone clones a local clone into dest with its local and origin branches, then points
// origin at the source's remote. HEAD is detached so every branch is free for a worktree.
func copyClone(src, dest string) (string, error) {
	src, _ = filepath.Abs(src)
	remote, err := git(src, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("%s has no origin remote", src)
	}
	if _, err := git(filepath.Dir(dest), "clone", "-q", "--no-checkout", src, dest); err != nil {
		return "", err
	}
	if _, err := git(dest, "fetch", "-q", "--update-head-ok", "--prune", src,
		"+refs/heads/*:refs/heads/*", "+refs/remotes/origin/*:refs/remotes/origin/*", "+refs/tags/*:refs/tags/*"); err != nil {
		return "", err
	}
	if _, err := git(dest, "remote", "set-url", "origin", remote); err != nil {
		return "", err
	}
	if _, err := git(dest, "checkout", "-q", "--detach", "HEAD"); err != nil {
		return "", err
	}
	branch, _ := git(src, "branch", "--show-current")
	note := "copied"
	if branch != "" {
		note += ", was on branch " + branch
	}
	if out, _ := git(src, "status", "--porcelain"); out != "" {
		note += "; WARNING: uncommitted changes in " + src + " were not copied"
	}
	return note, nil
}

// Adopt copies every git clone directly under old (an existing workspace) into repos/,
// and returns one line per repo. Old is only read.
func (w *Workspace) Adopt(old string) ([]string, error) {
	entries, err := os.ReadDir(old)
	if err != nil {
		return nil, err
	}
	var out []string
	var errs []error
	for _, e := range entries {
		dir := filepath.Join(old, e.Name())
		if _, err := os.Stat(filepath.Join(dir, ".git")); !e.IsDir() || err != nil {
			continue
		}
		name, note, err := w.RepoAdd(dir, false)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		out = append(out, name+": "+note)
	}
	if len(out) == 0 && len(errs) == 0 {
		return nil, fmt.Errorf("no git clones found in %s", old)
	}
	return out, errors.Join(errs...)
}

type RepoInfo struct {
	Name    string
	RefOnly bool
	Tracks  []string
}

// Repos lists repos/ from disk with the tracks using each.
func (w *Workspace) Repos() ([]RepoInfo, error) {
	entries, err := os.ReadDir(w.path("repos"))
	if err != nil {
		return nil, err
	}
	tracks, err := w.Tracks()
	if err != nil {
		return nil, err
	}
	var out []RepoInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		r := RepoInfo{Name: e.Name(), RefOnly: slices.Contains(w.Local.RefOnly, e.Name())}
		for _, t := range tracks {
			if slices.ContainsFunc(t.Repos, func(tr TrackRepo) bool { return tr.Name == r.Name }) {
				r.Tracks = append(r.Tracks, t.Task)
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// MissingRequired lists required tools and skills that are absent or not at their pinned ref.
func (w *Workspace) MissingRequired() []string {
	var missing []string
	for name, t := range w.Cfg.Required.Tools {
		if !t.ok(w.Root) {
			missing = append(missing, "tool "+name)
		}
	}
	for name, s := range w.Cfg.Required.Skills {
		if !w.skillPinned(name, s) {
			missing = append(missing, "skill "+name+"@"+s.Ref)
		}
	}
	if slices.Contains(w.Local.Adapters, "claude") {
		for name, s := range w.Cfg.Required.Skills {
			if s.ClaudePlugin != "" {
				if _, err := exec.LookPath("node"); err != nil {
					missing = append(missing, "tool node (hooks of skill "+name+")")
				}
				if out, err := run(w.Root, "claude", "plugin", "list"); err != nil || !strings.Contains(out, s.ClaudePlugin) {
					missing = append(missing, "claude plugin "+s.ClaudePlugin)
				}
			}
		}
	}
	slices.Sort(missing)
	return slices.Compact(missing)
}

func (w *Workspace) vendorDir(name string) string { return w.path(".siding", "vendor", name) }

func (w *Workspace) skillPinned(name string, s Skill) bool {
	dir := w.vendorDir(name)
	head, err := git(dir, "rev-parse", "HEAD")
	if err != nil {
		return false
	}
	want, err := git(dir, "rev-parse", s.Ref+"^{commit}")
	return err == nil && head == want
}

// FetchSkills clones each required skill at its pinned ref into .siding/vendor/.
func (w *Workspace) FetchSkills() error {
	var errs []error
	for name, s := range w.Cfg.Required.Skills {
		if w.skillPinned(name, s) {
			continue
		}
		url := s.Source
		if repo, ok := strings.CutPrefix(url, "github:"); ok {
			url = "https://github.com/" + repo
		}
		dir := w.vendorDir(name)
		os.RemoveAll(dir)
		os.MkdirAll(filepath.Dir(dir), 0o755)
		if _, err := git(w.Root, "-c", "advice.detachedHead=false", "clone", "-q", "--depth", "1", "--branch", s.Ref, url, dir); err != nil {
			errs = append(errs, fmt.Errorf("skill %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %s", strings.Join(args, " "), s)
	}
	return s, nil
}

// shell runs a user-written command line: sh -c, or cmd /c on Windows.
func shell(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", command)
	}
	return exec.Command("sh", "-c", command)
}

func sh(dir, command string) error {
	cmd := shell(command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", command, err, strings.TrimSpace(string(out)))
	}
	return nil
}
