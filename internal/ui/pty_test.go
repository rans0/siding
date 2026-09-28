//go:build !windows

package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rans0/siding/internal/core"
	"golang.org/x/net/websocket"
)

func TestLiveTerminal(t *testing.T) {
	ws, _ := filepath.EvalSymlinks(t.TempDir())
	os.WriteFile(filepath.Join(ws, "siding.toml"), nil, 0o644)
	os.MkdirAll(filepath.Join(ws, "tracks/p"), 0o755)
	os.WriteFile(filepath.Join(ws, "tracks/p/.track.json"), []byte(`{"task":"p","status":"build"}`), 0o644)
	w, _ := core.Open(ws)
	go w.Session("p", `read l; echo "got:$l"; sleep 1`)
	for i := 0; i < 50 && !w.SessionRunning("p"); i++ {
		time.Sleep(20 * time.Millisecond)
	}

	srv := httptest.NewServer(Handler(w))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	page := get(t, srv.URL+"/t/p")
	i := strings.Index(page, "?token=")
	if i < 0 {
		t.Fatalf("no live terminal on page:\n%s", page)
	}
	token := page[i+7 : i+39]

	if _, err := websocket.Dial("ws://"+host+"/t/p/pty?token="+token, "", "http://evil.example"); err == nil {
		t.Fatal("foreign origin accepted")
	}
	if _, err := websocket.Dial("ws://"+host+"/t/p/pty?token=bad", "", srv.URL); err == nil {
		t.Fatal("bad token accepted")
	}
	c, err := websocket.Dial("ws://"+host+"/t/p/pty?token="+token, "", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var size string
	if websocket.Message.Receive(c, &size) != nil || len(strings.Fields(size)) != 2 {
		t.Fatalf("size %q", size)
	}
	websocket.Message.Send(c, "hi\r")
	var out string
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for !strings.Contains(out, "got:hi") {
		var b []byte
		if err := websocket.Message.Receive(c, &b); err != nil {
			t.Fatalf("%v; output %q", err, out)
		}
		out += string(b)
	}
}

func get(t *testing.T, url string) string {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b := new(strings.Builder)
	io.Copy(b, res.Body)
	return b.String()
}
