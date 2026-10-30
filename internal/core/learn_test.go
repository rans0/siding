package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearnAndDoctor(t *testing.T) {
	ws := t.TempDir()
	if err := Init(ws); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(ws, "siding.toml"), []byte("[repos.api]\nenv_ports = [\"PORT\"]\n"), 0o644)
	w, err := Open(ws)
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(ws, "repos", "backend")
	os.MkdirAll(filepath.Join(clone, "scripts"), 0o755)
	os.WriteFile(filepath.Join(clone, "scripts", "db-up.sh"), nil, 0o644)
	os.MkdirAll(filepath.Join(clone, "src", "api", "booking"), 0o755)
	os.WriteFile(filepath.Join(clone, "src", "api", "booking", "checkout.ts"), nil, 0o644)
	gitT(t, clone, "init", "-q")

	inbox := filepath.Join(ws, "knowledge", "inbox")
	write := func(name, body string) { os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644) }
	write("1.md", "---\nrepo: backend   # comment\ntrack: promo\ndate: 2026-09-28\ncommit: a1b2c3d\n---\nRun `scripts/db-up.sh` before tests.\n")
	write("2.md", "---\nrepo: backend\ntrack: promo\ndate: 2026-09-28\ncommit: a1b2c3d\n---\nOld text\n")
	write("3.md", "---\nrepo: skill:debug\ntrack: promo\n---\n# debug\nBisect first.\n")
	write("4.md", "---\nrepo: nope\n---\nx\n")
	write("6.md", "---\nrepo: skill:debug\ntrack: promo\ndate: 2026-09-28\n---\nPrint the seed.\n")
	write("5.md", "---\nrule: style\ntrack: promo\ndate: 2026-09-28\n---\nNo comments that restate code.\n") // the slip agents make

	props, err := w.Inbox()
	if err != nil || len(props) != 6 || props[0].Repo != "backend" || props[0].Commit != "a1b2c3d" {
		t.Fatalf("inbox %+v %v", props, err)
	}
	if _, err := w.Accept("1.md", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Accept("2.md", "Uses `scripts/gone.sh` for seeds."); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Accept("3.md", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Accept("6.md", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Accept("5.md", ""); err != nil {
		t.Fatal(err)
	}
	if err := w.Reject("4.md"); err != nil {
		t.Fatal(err)
	}
	write("7.md", "---\ntrack: promo\n---\nNo target.\n")
	if _, err := w.Accept("7.md", ""); err == nil || !strings.Contains(err.Error(), "no target") {
		t.Fatalf("missing target: %v", err)
	}
	w.Reject("7.md")
	if _, err := w.Accept("../siding.toml", ""); err == nil {
		t.Fatal("path traversal accepted")
	}
	if props, _ := w.Inbox(); len(props) != 0 {
		t.Fatalf("inbox not emptied: %+v", props)
	}
	facts, _ := os.ReadFile(filepath.Join(ws, "knowledge/repos/backend.md"))
	want := "# backend\n\n- Run `scripts/db-up.sh` before tests. <!-- verified a1b2c3d promo 2026-09-28 -->\n- Uses `scripts/gone.sh` for seeds. <!-- verified a1b2c3d promo 2026-09-28 -->\n"
	if string(facts) != want {
		t.Fatalf("facts:\n%s", facts)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, "knowledge/skills/debug.md")); string(b) != "# debug\nBisect first.\n\n- Print the seed. <!-- verified promo 2026-09-28 -->\n" {
		t.Fatalf("skill:\n%s", b)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, "knowledge/rules/style.md")); string(b) != "- No comments that restate code. <!-- verified promo 2026-09-28 -->\n" {
		t.Fatalf("rule:\n%s", b)
	}

	// shortened and extensionless paths, elided paths and remote refs are not stale
	f, _ := os.OpenFile(filepath.Join(ws, "knowledge/repos/backend.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("- `booking/checkout.ts`, `api/booking`, `booking/checkout`, `src/.../checkout.ts`, `origin/main`.\n")
	f.Close()

	// orphaned worktree: git knows it, no track claims it; a deleted one only needs a prune
	gitT(t, clone, "add", ".")
	gitT(t, clone, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "init")
	gitT(t, clone, "worktree", "add", "-q", filepath.Join(w.Root, "tracks", "ghost", "backend"))
	gitT(t, clone, "worktree", "add", "-q", "-b", "gone", filepath.Join(w.Root, "tracks", "gone", "backend"))
	os.RemoveAll(filepath.Join(w.Root, "tracks", "gone"))

	var got []string
	for _, p := range w.Doctor() {
		if p.Kind != "missing" {
			got = append(got, p.Kind+": "+p.Detail)
		}
	}
	all := strings.Join(got, "\n")
	for _, s := range []string{
		"config: [repos.api] in siding.toml but repos/api is missing",
		`env_ports entry "PORT" is not KEY=port`,
		"worktree: orphaned worktree",
		"stale: knowledge/repos/backend.md:4: `scripts/gone.sh` not found",
		"still registers deleted worktree",
	} {
		if !strings.Contains(all, s) {
			t.Errorf("doctor missing %q in:\n%s", s, all)
		}
	}
	if strings.Contains(all, "backend.md:5") || strings.Contains(all, "orphaned worktree "+filepath.Join(w.Root, "tracks", "gone")) {
		t.Errorf("false positive:\n%s", all)
	}
	if strings.Contains(all, "db-up.sh") {
		t.Errorf("existing path flagged stale:\n%s", all)
	}
}
