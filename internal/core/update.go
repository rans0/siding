package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/rans0/siding"
)

// starterSources maps an embedded asset folder to where `siding init` copies it.
var starterSources = []struct{ asset, dest string }{
	{"starter-knowledge", "knowledge"},
	{"templates/docs", "templates"},
}

// releasedStarter holds SHA-256 of every starter file shipped by v0.1.0 and v0.2.0, so a
// workspace made before `siding update` existed is updated where its files are untouched.
var releasedStarter = []string{
	"1e6bd282a66b5572ab5bce46583c2a41a77a06cb1be5700e5f6ae27152f0b443", // knowledge/skills/deep-review.md
	"1fe1cb5e42f7ad49cf7d7739edf436cc26efac3dfbf719b84c80c7bcd592ff21", // knowledge/skills/adopt.md
	"2bf753141ba8cac8fa450273e8e91a89c10fe738a265f2c5a04130f9709c538b", // templates/prd.md
	"3f9f4a1c092fdb38a8b453b51bf1117f459ee35179e8406dd8330c7883cf8ae2", // templates/testing.md
	"4b817c8ec63dcec37860ce871aeddd0b545e69ef969ffa3808f694a17d4ef805", // knowledge/skills/cross-repo.md
	"4e3415c890b6ab446e1ac57ca4dfdb658252798f0620c96e2a43fdcd47bf9fa3", // knowledge/skills/review.md
	"5c0f35efd0da6cf5ea5f63274f703adbfbabe0d6135940705915cfdb919d47a1", // knowledge/skills/tdd.md
	"6bf114228f0fbc84d98564e41bdb0e241647027789f399898263187245edc57d", // knowledge/skills/retro.md
	"71fbb39dfcc4cc0c842d366d9afe3c18ab52f19e9641c442fc32bbdab7186d9d", // knowledge/rules/core.md
	"a14c6f4fe432260f9aa8c29b5fd2cb0dbcc368aaa779b40920f10962302a24be", // knowledge/skills/prd.md
	"c36cfe4bf522ae540dea5f38a63260771db43bc98e64bc77949ef30c49241104", // knowledge/skills/workflow.md
	"db1bfb3f0556b1a52aa128b665e9110d24f231a54233f51820bfaf32843a1778", // knowledge/skills/hotfix.md
	"e3cb3015ada8c673c2df3ceacda1151703b99b54fb094e668b0c958c095acd1a", // knowledge/skills/dispatch.md
	"e88054691365864c6adf6d04675d9f98e420bc0b3c6fca00a207164d23772f24", // templates/log.md
	"fe66b22d32c347f3c5cb8811602411fa3ec35815b9417b73ef346269ed68e080", // templates/plan.md
}

// UpdateResult says what Update did to one file, relative to the workspace root.
type UpdateResult struct{ Path, Action string }

const (
	UpdateAdded    = "added"
	UpdateUpdated  = "updated"
	UpdateEdited   = "kept (edited)" // yours differs from what siding shipped: new version saved aside
	UpdateUnchange = "up to date"
)

// Update brings starter skills, rules and doc templates up to this binary's version and
// regenerates every track's AGENTS.md. A file is replaced only when it still matches what
// siding last shipped (the baseline in .siding/starter/, or a released version); a file you
// edited is left alone and the new version is written to .siding/starter/new/ to merge by hand.
func (w *Workspace) Update() ([]UpdateResult, error) {
	var out []UpdateResult
	for _, src := range starterSources {
		sub, _ := fs.Sub(siding.Assets, src.asset)
		err := fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			next, _ := fs.ReadFile(sub, p)
			rel := filepath.Join(src.dest, filepath.FromSlash(p))
			action, err := w.updateFile(rel, next)
			if action != UpdateUnchange {
				out = append(out, UpdateResult{rel, action})
			}
			return err
		})
		if err != nil {
			return out, err
		}
	}
	tracks, _ := w.Tracks()
	for _, t := range tracks {
		if err := w.WriteAgentFiles(t); err != nil {
			return out, fmt.Errorf("track %s: %w", t.Task, err)
		}
	}
	return out, nil
}

func (w *Workspace) updateFile(rel string, next []byte) (string, error) {
	dest, base := w.path(rel), w.path(".siding", "starter", rel)
	cur, err := os.ReadFile(dest)
	action := UpdateAdded
	switch {
	case err == nil && bytes.Equal(cur, next):
		action = UpdateUnchange
	case err == nil && !w.shipped(base, cur):
		action = UpdateEdited
		if err := writeFile(w.path(".siding", "starter", "new", rel), next); err != nil {
			return action, err
		}
	case err == nil:
		action = UpdateUpdated
	}
	if action == UpdateAdded || action == UpdateUpdated {
		if err := writeFile(dest, next); err != nil {
			return action, err
		}
	}
	if action != UpdateEdited { // the baseline is what siding shipped and you have as is
		return action, writeFile(base, next)
	}
	return action, nil
}

// shipped reports whether cur is a version siding shipped: the recorded baseline or a release.
func (w *Workspace) shipped(base string, cur []byte) bool {
	if b, err := os.ReadFile(base); err == nil && bytes.Equal(b, cur) {
		return true
	}
	sum := sha256.Sum256(bytes.ReplaceAll(cur, []byte("\r\n"), []byte("\n"))) // hashes are of LF files
	return slices.Contains(releasedStarter, hex.EncodeToString(sum[:]))
}

// StarterOutdated lists starter files whose embedded version differs from the workspace copy
// and that Update would add or replace (edited files are not counted).
func (w *Workspace) StarterOutdated() []string {
	var out []string
	for _, src := range starterSources {
		sub, _ := fs.Sub(siding.Assets, src.asset)
		fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			next, _ := fs.ReadFile(sub, p)
			rel := filepath.Join(src.dest, filepath.FromSlash(p))
			cur, err := os.ReadFile(w.path(rel))
			if err != nil || (!bytes.Equal(cur, next) && w.shipped(w.path(".siding", "starter", rel), cur)) {
				out = append(out, rel)
			}
			return nil
		})
	}
	return out
}

func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
