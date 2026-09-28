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

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://evil.example:4747/", nil))
	if rec.Code != 403 {
		t.Fatalf("foreign Host got %d, want 403", rec.Code)
	}
}
