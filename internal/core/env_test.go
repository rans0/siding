package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteEnv(t *testing.T) {
	ws := t.TempDir()
	w := &Workspace{Root: ws}
	w.Cfg.Defaults.PortStride = 100
	w.Cfg.Repos = map[string]RepoConfig{"web": {EnvPorts: []string{"PORT=3000", "DB_PORT=5432"}}}
	os.MkdirAll(filepath.Join(ws, "repos/web"), 0o755)
	os.WriteFile(filepath.Join(ws, "repos/web/.env"), []byte("# app\nexport PORT=3000\nSECRET=x\n"), 0o644)
	dest := t.TempDir()
	if _, err := git(dest, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if err := w.writeEnv("feat", "web", dest, 2); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, ".env"))
	want := "# app\nPORT=3200\nSECRET=x\nCOMPOSE_PROJECT_NAME=track-feat\nDB_PORT=5632\n"
	if string(got) != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if st, _ := git(dest, "status", "--porcelain"); st != "" {
		t.Fatalf(".env not excluded: %q", st)
	}

	// no .env and no env_ports: nothing written
	os.MkdirAll(filepath.Join(ws, "repos/api"), 0o755)
	dest2 := t.TempDir()
	if err := w.writeEnv("feat", "api", dest2, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest2, ".env")); err == nil {
		t.Fatal(".env written for repo without ports")
	}

	w.Cfg.Repos["bad"] = RepoConfig{EnvPorts: []string{"PORT"}}
	if err := w.writeEnv("feat", "bad", t.TempDir(), 1); err == nil || !strings.Contains(err.Error(), "KEY=port") {
		t.Fatalf("want env_ports error, got %v", err)
	}
}

func TestFreeSlot(t *testing.T) {
	ws := t.TempDir()
	w := &Workspace{Root: ws}
	os.MkdirAll(filepath.Join(ws, "tracks"), 0o755)
	for task, slot := range map[string]int{"a": 0, "b": 2} {
		os.MkdirAll(filepath.Join(ws, "tracks", task), 0o755)
		w.saveTrack(&Track{Task: task, Slot: slot})
	}
	if s, _ := w.freeSlot(); s != 1 {
		t.Fatalf("slot = %d, want 1", s)
	}
}
