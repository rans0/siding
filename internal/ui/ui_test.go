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
	os.WriteFile(filepath.Join(wt, "x & y.txt"), []byte("odd name\n"), 0o644)

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

// src returns the lazy-load url of the first file block after marker in page.
func src(t *testing.T, page, marker string) string {
	t.Helper()
	i := strings.Index(page, marker)
	j := strings.Index(page[max(i, 0):], `data-src="`)
	if i < 0 || j < 0 {
		t.Fatalf("no %s file block:\n%s", marker, page)
	}
	rest := page[i+j+len(`data-src="`):]
	return rest[:strings.IndexByte(rest, '"')]
}

// The track page lists uncommitted and committed files; each diff loads on its own.
func TestTrackChanges(t *testing.T) {
	h, _ := gitTrack(t)
	code, page := serve(h, "/t/feat")
	if code != 200 || strings.Contains(page, "+wip") {
		t.Fatalf("diff should load lazily, code %d:\n%s", code, page)
	}
	for marker, want := range map[string]string{">Uncommitted<": "+wip", ">Committed<": "+committed"} {
		u := src(t, page, marker)
		if code, frag := serve(h, u); code != 200 || !strings.Contains(frag, want) {
			t.Errorf("%s %s: %d\n%s", marker, u, code, frag)
		}
	}
	// the block's data-src is the last one before its name
	odd := page[:strings.Index(page, "<code>x & y.txt")]
	odd = odd[strings.LastIndex(odd, `data-src="`)+len(`data-src="`):]
	if _, frag := serve(h, odd[:strings.IndexByte(odd, '"')]); !strings.Contains(frag, "+odd name") {
		t.Errorf("odd file name not escaped in url:\n%s", frag)
	}
	if code, _ := serve(h, "/t/feat/app/diff?scope=committed&path=nope.txt"); code != 404 {
		t.Errorf("unlisted path got %d, want 404", code)
	}
}

// Commits are listed and each opens a page with its message and files.
func TestCommitPage(t *testing.T) {
	h, sha := gitTrack(t)
	if _, page := serve(h, "/t/feat"); !strings.Contains(page, `href="/t/feat/app/`+sha[:7]) || !strings.Contains(page, "add feature") {
		t.Fatalf("commit link missing:\n%s", page)
	}
	code, page := serve(h, "/t/feat/app/"+sha)
	if code != 200 || !strings.Contains(page, "add feature") {
		t.Fatalf("commit page %d:\n%s", code, page)
	}
	if _, frag := serve(h, src(t, page, "<section>")); !strings.Contains(frag, "+committed") || strings.Contains(frag, "+wip") {
		t.Errorf("commit file diff:\n%s", frag)
	}
	if code, _ := serve(h, "/t/feat/app/HEAD"); code != 404 {
		t.Errorf("non-sha got %d, want 404", code)
	}
}
