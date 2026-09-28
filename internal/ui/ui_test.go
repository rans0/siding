package ui

import (
	"net/http/httptest"
	"os"
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
