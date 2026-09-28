// Package ui serves the read-only board for `siding ui`.
package ui

import (
	"fmt"
	"html/template"
	"net"
	"net/http"

	"github.com/rans0/siding"
	"github.com/rans0/siding/internal/core"
)

var board = template.Must(template.ParseFS(siding.Assets, "web/board.html"))

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

// Handler renders the board from the workspace files on every request; it keeps no state.
func Handler(w *core.Workspace) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(rw http.ResponseWriter, r *http.Request) {
		tracks, err := w.Tracks()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		cols := []column{{Name: "PRD"}, {Name: "Build"}, {Name: "Review"}, {Name: "Done"}}
		for _, t := range tracks {
			c := card{Task: t.Task, Stage: core.StageLabel[t.Status], Hotfix: t.Hotfix,
				Gate: t.Status == core.StageGate1 || t.Status == core.StageGate2}
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
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := board.Execute(rw, map[string]any{"Root": w.Root, "Columns": cols}); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
	})
	return localOnly(mux)
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
