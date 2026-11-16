package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpdate: untouched files follow siding, edited ones are kept with the new version aside,
// missing ones are added, and a pre-update workspace is recognized by released hashes.
func TestUpdate(t *testing.T) {
	ws := t.TempDir()
	if err := Init(ws); err != nil {
		t.Fatal(err)
	}
	w, err := Open(ws)
	if err != nil {
		t.Fatal(err)
	}
	if res, err := w.Update(); err != nil || len(res) != 0 || len(w.StarterOutdated()) != 0 {
		t.Fatalf("fresh workspace not up to date: %v %v", res, err)
	}

	k := func(p string) string { return filepath.Join(ws, "knowledge", "skills", p) }
	current := func(p string) string { b, _ := os.ReadFile(k(p)); return string(b) }
	tdd, review := current("tdd.md"), current("review.md")

	// tdd.md: shipped an older version, untouched since. review.md: shipped, then edited.
	// diagnose.md: missing (workspace made before it existed).
	os.WriteFile(k("tdd.md"), []byte("old tdd\n"), 0o644)
	os.WriteFile(filepath.Join(ws, ".siding/starter/knowledge/skills/tdd.md"), []byte("old tdd\n"), 0o644)
	os.WriteFile(filepath.Join(ws, ".siding/starter/knowledge/skills/review.md"), []byte("old review\n"), 0o644)
	os.WriteFile(k("review.md"), []byte("old review\n- my fact\n"), 0o644)
	os.Remove(k("diagnose.md"))
	if got := len(w.StarterOutdated()); got != 2 {
		t.Fatalf("outdated = %d, want 2 (tdd, diagnose)", got)
	}

	res, err := w.Update()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range res {
		got[filepath.Base(r.Path)] = r.Action
	}
	if got["tdd.md"] != UpdateUpdated || got["review.md"] != UpdateEdited || got["diagnose.md"] != UpdateAdded || len(got) != 3 {
		t.Fatalf("actions = %v", got)
	}
	if current("tdd.md") != tdd || !strings.Contains(current("review.md"), "my fact") || current("diagnose.md") == "" {
		t.Fatal("file contents wrong after update")
	}
	if b, _ := os.ReadFile(filepath.Join(ws, ".siding/starter/new/knowledge/skills/review.md")); string(b) != review {
		t.Fatal("new version of an edited file not saved aside")
	}
	if res, _ := w.Update(); len(res) != 1 || res[0].Action != UpdateEdited {
		t.Fatalf("second update: %v", res)
	}

	// a v0.2.0 workspace has no baseline: a released file is still recognized as untouched
	os.RemoveAll(filepath.Join(ws, ".siding/starter"))
	os.WriteFile(k("tdd.md"), []byte(released(t, "starter-knowledge/skills/tdd.md")), 0o644)
	if res, _ := w.Update(); current("tdd.md") != tdd || len(res) != 2 {
		t.Fatalf("released tdd.md not updated: %v", res)
	}
}

// released returns a file as v0.2.0 shipped it, from the repo's git history.
func released(t *testing.T, path string) string {
	out, err := git("../..", "show", "v0.2.0:"+path)
	if err != nil {
		t.Skip("v0.2.0 tag not available (shallow clone)")
	}
	return out + "\n"
}
