//go:build !windows

package core

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSession(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // linux
	t.Setenv("HOME", t.TempDir())           // darwin UserCacheDir
	ws, _ := filepath.EvalSymlinks(t.TempDir())
	w := &Workspace{Root: ws}
	os.MkdirAll(w.trackDir("s"), 0o755)
	w.saveTrack(&Track{Task: "s", Slot: 2})

	// agent: prints its track, echoes one typed line, then exits
	done := make(chan error, 1)
	go func() { done <- w.Session("s", `echo "track=$SIDING_TRACK slot=$SIDING_SLOT"; read l; echo "got:$l"`) }()

	var c interface {
		Write([]byte) (int, error)
		Read([]byte) (int, error)
		Close() error
	}
	for i := 0; i < 50; i++ {
		if conn, err := w.Attach("s"); err == nil {
			c = conn
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if c == nil {
		select {
		case err := <-done:
			t.Fatal("session ended:", err)
		default:
		}
		t.Fatal("session socket never came up")
	}
	defer c.Close()
	r := bufio.NewReader(c)
	if size, _ := r.ReadString('\n'); len(strings.Fields(size)) != 2 {
		t.Fatalf("size line %q", size)
	}
	if !w.SessionRunning("s") {
		t.Fatal("SessionRunning false while running")
	}
	c.Write([]byte("hello\r"))
	var out strings.Builder
	deadline := time.After(3 * time.Second)
	for !strings.Contains(out.String(), "got:hello") {
		select {
		case <-deadline:
			t.Fatalf("output so far: %q", out.String())
		default:
		}
		b := make([]byte, 256)
		n, err := r.Read(b)
		out.Write(b[:n])
		if err != nil {
			break
		}
	}
	if !strings.Contains(out.String(), "track=s slot=2") || !strings.Contains(out.String(), "got:hello") {
		t.Fatalf("output %q", out.String())
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("session did not end with the agent")
	}
	if w.SessionRunning("s") {
		t.Fatal("socket left behind")
	}
}
