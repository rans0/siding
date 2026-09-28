package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const scrollback = 256 << 10

// sockDir holds session sockets: a short path (unix sockets allow ~104 bytes), mode 0700.
var sockDir = filepath.Join("/tmp", "siding-"+strconv.Itoa(os.Getuid()))

// sockPath is where a track's running session listens.
func (w *Workspace) sockPath(task string) string {
	h := sha256.Sum256([]byte(w.trackDir(task)))
	return filepath.Join(sockDir, hex.EncodeToString(h[:8])+".sock")
}

// Attach connects to a track's running session. The session first sends one line
// "cols rows", then its scrollback, then live output; bytes written are typed into it.
func (w *Workspace) Attach(task string) (net.Conn, error) {
	return net.Dial("unix", w.sockPath(task))
}

// SessionRunning reports whether `siding open` is running for the track.
func (w *Workspace) SessionRunning(task string) bool {
	c, err := w.Attach(task)
	if err == nil {
		c.Close()
	}
	return err == nil
}

// Session runs the agent for a track in a pseudo-terminal owned by siding, shown in the
// current terminal and shared with the board. If the track's session is already
// running, it attaches to it instead. Without PTY support (Windows) the agent runs
// directly in the terminal and the board shows no terminal.
func (w *Workspace) Session(task, agent string) error {
	t, err := w.LoadTrack(task)
	if err != nil {
		return err
	}
	if agent == "" {
		agent = w.Local.Agent
	}
	if agent == "" {
		agent = "claude"
	}
	if c, err := w.Attach(task); err == nil {
		fmt.Fprintf(os.Stderr, "siding: attaching to the running session of %s\r\n", task)
		return attach(c)
	}
	cmd := shell(agent)
	cmd.Dir = w.trackDir(task)
	// Ports reach services through each worktree's .env; the session only needs to know its track.
	cmd.Env = append(os.Environ(), "SIDING_TRACK="+task, "SIDING_SLOT="+strconv.Itoa(t.Slot))

	in := int(os.Stdin.Fd())
	size := &pty.Winsize{Cols: 120, Rows: 40}
	if c, r, err := term.GetSize(in); err == nil {
		size = &pty.Winsize{Cols: uint16(c), Rows: uint16(r)}
	}
	ptmx, err := pty.StartWithSize(cmd, size)
	if errors.Is(err, pty.ErrUnsupported) {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return cmd.Run()
	}
	if err != nil {
		return err
	}
	defer ptmx.Close()

	sock := w.sockPath(task)
	if err := privateDir(sockDir); err != nil {
		cmd.Process.Kill()
		return err
	}
	os.Remove(sock) // stale: nothing answered on it
	ln, err := net.Listen("unix", sock)
	if err != nil {
		cmd.Process.Kill()
		return err
	}
	defer os.Remove(sock)
	defer ln.Close()
	os.Chmod(sock, 0o600)

	if term.IsTerminal(in) {
		if old, err := term.MakeRaw(in); err == nil {
			defer term.Restore(in, old)
		}
		go followSize(in, ptmx, size)
	}
	s := &hub{ptmx: ptmx, size: size, clients: map[net.Conn]bool{}}
	go io.Copy(ptmx, os.Stdin)
	go s.serve(ln)
	s.pump(os.Stdout)
	return cmd.Wait()
}

// followSize resizes the PTY when the user's terminal changes size.
// ponytail: polls twice a second instead of SIGWINCH, which has no Windows counterpart.
func followSize(fd int, ptmx *os.File, cur *pty.Winsize) {
	for range time.Tick(500 * time.Millisecond) {
		c, r, err := term.GetSize(fd)
		if err != nil || (uint16(c) == cur.Cols && uint16(r) == cur.Rows) {
			continue
		}
		*cur = pty.Winsize{Cols: uint16(c), Rows: uint16(r)}
		pty.Setsize(ptmx, cur)
	}
}

// hub fans the agent's output out to the local terminal and attached clients.
type hub struct {
	ptmx    *os.File
	size    *pty.Winsize
	mu      sync.Mutex
	buf     []byte
	clients map[net.Conn]bool
}

func (s *hub) pump(out io.Writer) {
	b := make([]byte, 32<<10)
	for {
		n, err := s.ptmx.Read(b)
		if n > 0 {
			out.Write(b[:n])
			s.mu.Lock()
			s.buf = append(s.buf, b[:n]...)
			if len(s.buf) > scrollback {
				s.buf = s.buf[len(s.buf)-scrollback:]
			}
			for c := range s.clients {
				c.SetWriteDeadline(time.Now().Add(time.Second))
				if _, err := c.Write(b[:n]); err != nil {
					c.Close()
					delete(s.clients, c)
				}
			}
			s.mu.Unlock()
		}
		if err != nil {
			s.mu.Lock()
			for c := range s.clients {
				c.Close()
			}
			s.mu.Unlock()
			return
		}
	}
}

func (s *hub) serve(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		// ponytail: size is sent once; a client that attached before a resize keeps the old size until it reconnects.
		fmt.Fprintf(c, "%d %d\n", s.size.Cols, s.size.Rows)
		c.Write(s.buf)
		s.clients[c] = true
		s.mu.Unlock()
		go func() {
			io.Copy(s.ptmx, c)
			s.mu.Lock()
			delete(s.clients, c)
			s.mu.Unlock()
			c.Close()
		}()
	}
}

// attach shows a running session in this terminal until it ends or this terminal closes.
func attach(c net.Conn) error {
	defer c.Close()
	r := bufio.NewReader(c)
	if _, err := r.ReadString('\n'); err != nil {
		return err
	}
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		if old, err := term.MakeRaw(fd); err == nil {
			defer term.Restore(fd, old)
		}
	}
	go io.Copy(c, os.Stdin)
	_, err := io.Copy(os.Stdout, r)
	return err
}

// privateDir creates dir 0700, or checks an existing one is ours and not shared: /tmp is
// world-writable, so another user could pre-create it to intercept the agent's terminal.
func privateDir(dir string) error {
	if err := os.Mkdir(dir, 0o700); err == nil || !errors.Is(err, fs.ErrExist) {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() || fi.Mode().Perm()&0o077 != 0 || !ownedByMe(fi) {
		return fmt.Errorf("%s is not a private directory owned by you; remove it and retry", dir)
	}
	return nil
}
