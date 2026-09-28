// Package ui serves the board for `siding ui`. It keeps no state of its own: every page
// is rendered from the workspace files through core, and every action calls core.
package ui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rans0/siding"
	"github.com/rans0/siding/internal/core"
)

var pages = template.Must(template.ParseFS(siding.Assets, "web/*.html"))

type repoCard struct {
	Name, Summary string
	Clean         bool
}

type card struct {
	Task, Stage string
	Hotfix      bool
	Gate        bool
	Repos       []repoCard
}

type column struct {
	Name  string
	Cards []card
}

var columnOf = map[string]int{
	core.StagePRD: 0, core.StageGate1: 0,
	core.StageBuild: 1,
	core.StageGate2: 2,
	core.StageShip:  3,
}

type diffLine struct{ Class, Text string }

type repoReport struct {
	R    core.RepoReport
	Diff []diffLine
}

type doc struct{ Name, Body string }

// Handler serves the board. token guards the gate actions against cross-site posts;
// it lives only as long as the server.
func Handler(w *core.Workspace) http.Handler {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /style.css", func(rw http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(rw, r, siding.Assets, "web/style.css")
	})
	mux.HandleFunc("GET /{$}", func(rw http.ResponseWriter, r *http.Request) {
		tracks, err := w.Tracks()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		cols := []column{{Name: "PRD"}, {Name: "Build"}, {Name: "Review"}, {Name: "Done"}}
		for _, t := range tracks {
			c := card{Task: t.Task, Stage: core.StageLabel[t.Status], Hotfix: t.Hotfix, Gate: atGate(t)}
			for _, s := range w.State(t) {
				rc := repoCard{Name: s.Name, Summary: "clean", Clean: true}
				switch {
				case s.Err != nil:
					rc = repoCard{Name: s.Name, Summary: "error"}
				case s.Changed > 0 || s.Ahead > 0 || s.Behind > 0:
					rc = repoCard{Name: s.Name, Summary: fmt.Sprintf("%d changed +%d -%d", s.Changed, s.Ahead, s.Behind)}
				}
				c.Repos = append(c.Repos, rc)
			}
			cols[columnOf[t.Status]].Cards = append(cols[columnOf[t.Status]].Cards, c)
		}
		render(rw, "board.html", map[string]any{"Root": w.Root, "Columns": cols})
	})
	mux.HandleFunc("GET /t/{task}", func(rw http.ResponseWriter, r *http.Request) {
		t, err := w.LoadTrack(r.PathValue("task"))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}
		var reports []repoReport
		for _, rep := range w.Report(t) {
			d, _ := w.Diff(t, rep.Name)
			reports = append(reports, repoReport{rep, classify(d)})
		}
		var docs []doc
		for _, name := range []string{"prd.md", "plan.md", "testing.md", "log.md"} {
			if b, err := os.ReadFile(filepath.Join(w.Root, t.Docs, name)); err == nil {
				docs = append(docs, doc{name, string(b)})
			}
		}
		render(rw, "track.html", map[string]any{
			"T": t, "Stage": core.StageLabel[t.Status], "AtGate": atGate(t), "Token": token,
			"Reports": reports, "Docs": docs, "HasTerminal": w.Terminal(t.Task) != "",
			"Error": r.URL.Query().Get("error"),
		})
	})
	mux.HandleFunc("GET /t/{task}/terminal", func(rw http.ResponseWriter, r *http.Request) {
		render(rw, "terminal.html", w.Terminal(r.PathValue("task")))
	})
	mux.HandleFunc("GET /inbox", func(rw http.ResponseWriter, r *http.Request) {
		props, err := w.Inbox()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		render(rw, "inbox.html", map[string]any{"Props": props, "Token": token,
			"Error": r.URL.Query().Get("error"), "Done": r.URL.Query().Get("done")})
	})
	mux.HandleFunc("POST /inbox/{file}/{action}", func(rw http.ResponseWriter, r *http.Request) {
		if !validToken(rw, r, token) {
			return
		}
		file, dest, err := r.PathValue("file"), "", error(nil)
		switch r.PathValue("action") {
		case "accept":
			dest, err = w.Accept(file, r.PostFormValue("body"))
		case "delete":
			err = w.Reject(file)
		default:
			http.NotFound(rw, r)
			return
		}
		q := "?done=" + template.URLQueryEscaper(file+" -> "+dest)
		if dest == "" {
			q = "?done=" + template.URLQueryEscaper(file+" deleted")
		}
		if err != nil {
			q = "?error=" + template.URLQueryEscaper(err.Error())
		}
		http.Redirect(rw, r, "/inbox"+q, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /health", func(rw http.ResponseWriter, r *http.Request) {
		render(rw, "health.html", w.Doctor())
	})
	mux.HandleFunc("POST /t/{task}/{action}", func(rw http.ResponseWriter, r *http.Request) {
		if !validToken(rw, r, token) {
			return
		}
		task, action := r.PathValue("task"), r.PathValue("action")
		if action != "approve" && action != "back" {
			http.NotFound(rw, r)
			return
		}
		dest := "/t/" + task
		if _, err := w.Approve(task, action == "back"); err != nil {
			dest += "?error=" + template.URLQueryEscaper(err.Error())
		}
		http.Redirect(rw, r, dest, http.StatusSeeOther)
	})
	return localOnly(mux)
}

func validToken(rw http.ResponseWriter, r *http.Request, token string) bool {
	if subtle.ConstantTimeCompare([]byte(r.PostFormValue("token")), []byte(token)) != 1 {
		http.Error(rw, "stale page, reload and try again", http.StatusForbidden)
		return false
	}
	return true
}

func atGate(t *core.Track) bool { return t.Status == core.StageGate1 || t.Status == core.StageGate2 }

func classify(diff string) []diffLine {
	if diff == "" {
		return nil
	}
	var out []diffLine
	for _, l := range strings.Split(diff, "\n") {
		c := ""
		switch {
		case strings.HasPrefix(l, "+++ "), strings.HasPrefix(l, "--- "), strings.HasPrefix(l, "diff "), strings.HasPrefix(l, "@@"):
			c = "hunk"
		case strings.HasPrefix(l, "+"):
			c = "add"
		case strings.HasPrefix(l, "-"):
			c = "del"
		}
		out = append(out, diffLine{c, l})
	}
	return out
}

func render(rw http.ResponseWriter, name string, data any) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.ExecuteTemplate(rw, name, data); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

// localOnly rejects requests whose Host is not loopback, so a web page on another
// origin cannot read the board through DNS rebinding.
func localOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			http.Error(rw, "siding ui only answers on 127.0.0.1", http.StatusForbidden)
			return
		}
		h.ServeHTTP(rw, r)
	})
}
