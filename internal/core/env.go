package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// freeSlot returns the lowest port slot no existing track uses. Slot 0 keeps the
// configured ports, so the first track runs on the same ports as main.
func (w *Workspace) freeSlot() (int, error) {
	tracks, err := w.Tracks()
	if err != nil {
		return 0, err
	}
	used := map[int]bool{}
	for _, t := range tracks {
		used[t.Slot] = true
	}
	slot := 0
	for used[slot] {
		slot++
	}
	return slot, nil
}

func (w *Workspace) stride() int {
	if w.Cfg.Defaults.PortStride > 0 {
		return w.Cfg.Defaults.PortStride
	}
	return 100
}

// writeEnv writes dest/.env: the main clone's .env (if any) with each env_ports key
// shifted by the track's slot, plus COMPOSE_PROJECT_NAME. Nothing is written when the
// repo has neither a .env nor env_ports.
func (w *Workspace) writeEnv(task, repo, dest string, slot int) error {
	src, err := os.ReadFile(w.path("repos", repo, ".env"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	ports := w.Cfg.Repos[repo].EnvPorts
	if src == nil && len(ports) == 0 {
		return nil
	}
	set := map[string]string{"COMPOSE_PROJECT_NAME": "track-" + task}
	for _, p := range ports {
		k, v, ok := strings.Cut(p, "=")
		n, err := strconv.Atoi(v)
		if !ok || err != nil {
			return fmt.Errorf("env_ports %q: want KEY=port", p)
		}
		set[k] = strconv.Itoa(n + slot*w.stride())
	}
	var out []string
	for _, line := range strings.Split(strings.TrimRight(string(src), "\n"), "\n") {
		k, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "export "), "=")
		if v, ok := set[strings.TrimSpace(k)]; ok {
			line = strings.TrimSpace(k) + "=" + v
			delete(set, strings.TrimSpace(k))
		}
		if line != "" || len(out) > 0 {
			out = append(out, line)
		}
	}
	for _, p := range append([]string{"COMPOSE_PROJECT_NAME"}, keys(ports)...) {
		if v, ok := set[p]; ok {
			out = append(out, p+"="+v)
		}
	}
	if err := os.WriteFile(filepath.Join(dest, ".env"), []byte(strings.Join(out, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	return excludeEnv(dest)
}

// excludeEnv keeps a generated .env out of commits and out of done's dirty check.
// A repo that tracks .env on purpose is left alone: the change then shows as modified.
func excludeEnv(dest string) error {
	if _, err := git(dest, "ls-files", "--error-unmatch", ".env"); err == nil {
		return nil
	}
	if _, err := git(dest, "check-ignore", "-q", ".env"); err == nil {
		return nil
	}
	path, err := git(dest, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dest, path)
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n# added by siding: per-track env\n/.env\n")
	return err
}

func keys(pairs []string) []string {
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i], _, _ = strings.Cut(p, "=")
	}
	return out
}
