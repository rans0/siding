package ui

import (
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rans0/siding/internal/core"
)

func TestBoard(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "siding.toml"), nil, 0o644)
	os.MkdirAll(filepath.Join(ws, "tracks/promo"), 0o755)
	os.WriteFile(filepath.Join(ws, "tracks/promo/.track.json"), []byte(`{"task":"promo","status":"gate1"}`), 0o644)
	w, err := core.Open(ws)
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(w)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://127.0.0.1:4747/", nil))
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "promo") || !strings.Contains(body, `card gate`) {
		t.Fatalf("code %d, body:\n%s", rec.Code, body)
	}

	// gate action: detail page carries the token; a post without it is refused
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://127.0.0.1:4747/t/promo", nil))
	page := rec.Body.String()
	i := strings.Index(page, `name="token" value="`)
	if rec.Code != 200 || i < 0 {
		t.Fatalf("detail code %d, body:\n%s", rec.Code, page)
	}
	token := page[i+20 : i+52]
	post := func(form string) int {
		req := httptest.NewRequest("POST", "http://127.0.0.1:4747/t/promo/approve", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if post("token=wrong") != 403 {
		t.Fatal("post without valid token accepted")
	}
	if post("token="+token) != 303 {
		t.Fatal("approve failed")
	}
	if tr, _ := w.LoadTrack("promo"); tr.Status != core.StageBuild {
		t.Fatalf("stage = %s after approve", tr.Status)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://evil.example:4747/", nil))
	if rec.Code != 403 {
		t.Fatalf("foreign Host got %d, want 403", rec.Code)
	}
}

func TestInboxAndHealth(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "siding.toml"), nil, 0o644)
	os.MkdirAll(filepath.Join(ws, "knowledge/inbox"), 0o755)
	os.MkdirAll(filepath.Join(ws, "knowledge/repos"), 0o755)
	os.WriteFile(filepath.Join(ws, "knowledge/inbox/1.md"), []byte("---\nrepo: web\ntrack: promo\ncommit: abc\n---\nOld\n"), 0o644)
	w, _ := core.Open(ws)
	h := Handler(w)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://127.0.0.1/inbox", nil))
	page := rec.Body.String()
	i := strings.Index(page, `name="token" value="`)
	if rec.Code != 200 || i < 0 || !strings.Contains(page, "Old") {
		t.Fatalf("inbox code %d:\n%s", rec.Code, page)
	}
	req := httptest.NewRequest("POST", "http://127.0.0.1/inbox/1.md/accept", strings.NewReader("token="+page[i+20:i+52]+"&body=Edited"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if b, _ := os.ReadFile(filepath.Join(ws, "knowledge/repos/web.md")); rec.Code != 303 || !strings.Contains(string(b), "- Edited <!-- verified abc promo") {
		t.Fatalf("accept code %d, web.md:\n%s", rec.Code, b)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://127.0.0.1/health", nil))
	if rec.Code != 200 {
		t.Fatalf("health code %d", rec.Code)
	}
}

// gitTrack makes workspace ws with track "feat" whose repo "app" has one commit past base,
// then leaves an uncommitted edit. It returns the handler and the commit's sha.
func gitTrack(t *testing.T) (http.Handler, string) {
	t.Helper()
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "t")
	}
	ws := t.TempDir()
	wt := filepath.Join(ws, "tracks/feat/app")
	os.MkdirAll(wt, 0o755)
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", wt}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("a\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "init")
	base := git("rev-parse", "HEAD")
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("committed\n"), 0o644)
	git("commit", "-qam", "add feature")
	sha := git("rev-parse", "HEAD")
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("committed\nwip\n"), 0o644)

	os.WriteFile(filepath.Join(ws, "siding.toml"), nil, 0o644)
	os.WriteFile(filepath.Join(ws, "tracks/feat/.track.json"),
		[]byte(`{"task":"feat","status":"gate2","repos":[{"name":"app","branch":"b","base":"main","base_commit":"`+base+`"}]}`), 0o644)
	w, err := core.Open(ws)
	if err != nil {
		t.Fatal(err)
	}
	return Handler(w), sha
}

func serve(h http.Handler, path string) (int, string) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://127.0.0.1"+path, nil))
	return rec.Code, html.UnescapeString(rec.Body.String())
}

// The track page splits uncommitted from committed work.
func TestTrackChanges(t *testing.T) {
	h, _ := gitTrack(t)
	code, page := serve(h, "/t/feat")
	u, c := strings.Index(page, "Uncommitted"), strings.Index(page, "Committed")
	if code != 200 || u < 0 || c < 0 {
		t.Fatalf("code %d:\n%s", code, page)
	}
	if wip := strings.Index(page, "+wip"); wip < u || wip > c {
		t.Errorf("+wip not under Uncommitted:\n%s", page)
	}
	if strings.LastIndex(page, "+committed") < c {
		t.Errorf("+committed not under Committed:\n%s", page)
	}
}

// Commits are listed and each opens its own diff.
func TestCommitPage(t *testing.T) {
	h, sha := gitTrack(t)
	if _, page := serve(h, "/t/feat"); !strings.Contains(page, `href="/t/feat/app/`+sha[:7]) || !strings.Contains(page, "add feature") {
		t.Fatalf("commit link missing:\n%s", page)
	}
	code, page := serve(h, "/t/feat/app/"+sha)
	if code != 200 || !strings.Contains(page, "add feature") || !strings.Contains(page, "+committed") || strings.Contains(page, "+wip") {
		t.Fatalf("commit page %d:\n%s", code, page)
	}
	if code, _ := serve(h, "/t/feat/app/HEAD"); code != 404 {
		t.Errorf("non-sha got %d, want 404", code)
	}
}

// Diffs split into one foldable block per file, with +/- counts; big files start folded.
func TestSplitFiles(t *testing.T) {
	big := strings.Repeat("+x\n", maxOpenLines+1)
	d := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n" +
		"diff --git a/big.txt b/big.txt\n--- /dev/null\n+++ b/big.txt\n@@ -0,0 +1 @@\n" + big
	files := splitFiles(d)
	if len(files) != 2 {
		t.Fatalf("got %d files", len(files))
	}
	if f := files[0]; f.Name != "a.go" || f.Add != 1 || f.Del != 1 || !f.Open || len(f.Lines) != 6 {
		t.Errorf("a.go: %+v", f)
	}
	if f := files[1]; f.Name != "big.txt" || f.Add != maxOpenLines+1 || f.Open {
		t.Errorf("big.txt: name %s add %d open %v", f.Name, f.Add, f.Open)
	}
	if splitFiles("") != nil {
		t.Error("empty diff gave files")
	}
	// commit header before the first file is kept as its own block
	if f := splitFiles("commit abc\nAuthor: t\n\n    msg\ndiff --git a/a b/a\n+x"); len(f) != 2 || f[0].Name != "" || f[1].Name != "a" {
		t.Errorf("header: %+v", f)
	}
}
