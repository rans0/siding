// Command siding: one task, one track, across every repo it touches, with any agent.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rans0/siding/internal/core"
	"github.com/rans0/siding/internal/ui"
)

const usage = `siding: multi-repo, multi-task workspace manager for coding agents

  siding init [dir]                          create a workspace
  siding repo add <url|dir> [--ref-only]     clone into repos/ (a local clone is copied with its branches)
  siding adopt <old-workspace>               copy every clone in an old workspace into repos/
  siding repo ls                             repos and the tracks using them
  siding new <task> <repos> [--from ref|a=x,b=y] [--hotfix] [--branch name|a=x,b=y] [--docs dir]
                                             --branch: branch name as is; --docs: adopt work started before siding
  siding ls                                  all tracks
  siding submit <task>                       agent: move the track to its next gate
  siding approve <task> [--back]             you: pass the gate, or send the track back
  siding open <task> [--agent cmd]           start the agent in the track, or attach to it
  siding ui [--port N]                       board on 127.0.0.1
  siding report <task>                       base, commits and diffstat per repo
  siding pr <task>                           after Gate 2: commit, push, open a PR per repo (needs gh)
  siding next <task>                         after pr: start the next cycle on the same branches (feature B after A)
  siding review <task>                       deep-review a finished task: report, then fix through the gates
  siding done <task> [--force]               remove the track's worktrees (needs log.md; docs stay)
  siding adapter install <harness>           agents|claude|codex|cursor|opencode: tools, skills, harness files
  siding learn                               review knowledge/inbox: accept, edit, skip, delete
  siding doctor                              check tools, skills, config, worktrees, stale facts
  siding version
`

var version = "dev" // set by the release build

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "siding:", err)
		os.Exit(1)
	}
}

// parse lets flags appear after positional args (siding new task a,b --hotfix).
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func need(args []string, n int, form string) error {
	if len(args) != n {
		return fmt.Errorf("usage: siding %s", form)
	}
	return nil
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(usage)
		return nil
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Println("siding", version)
		return nil
	}
	cmd, args := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)

	if cmd == "init" {
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		dir := "."
		if len(pos) > 0 {
			dir = pos[0]
		}
		if err := core.Init(dir); err != nil {
			return err
		}
		fmt.Printf("workspace created in %s\nnext: siding repo add <url>, then siding adapter install claude\n", dir)
		return nil
	}

	w, err := core.Open(".")
	if err != nil {
		if cmd == "guard" {
			return nil // outside a workspace nothing to guard
		}
		return err
	}

	switch cmd {
	case "repo":
		refOnly := fs.Bool("ref-only", false, "read-only reference repo")
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		switch {
		case len(pos) == 2 && pos[0] == "add":
			name, note, err := w.RepoAdd(pos[1], *refOnly)
			if err == nil {
				fmt.Println("added repos/"+name, note)
			}
			return err
		case len(pos) == 1 && pos[0] == "ls":
			repos, err := w.Repos()
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(tw, "REPO\tMODE\tTRACKS")
			for _, r := range repos {
				mode := "work"
				if r.RefOnly {
					mode = "ref-only"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, mode, strings.Join(r.Tracks, ","))
			}
			return tw.Flush()
		}
		return fmt.Errorf("usage: siding repo add <url> [--ref-only] | siding repo ls")

	case "adopt":
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 1, "adopt <old-workspace>"); err != nil {
			return err
		}
		lines, err := w.Adopt(pos[0])
		for _, l := range lines {
			fmt.Println("  " + l)
		}
		if len(lines) > 0 {
			fmt.Println("next, per task in progress: siding new <task> <repos> --branch repo=branch,... --docs <old-workspace>/docs/<folder>")
		}
		return err

	case "new":
		from := fs.String("from", "", "base ref for all repos, or repo=ref,repo=ref")
		hotfix := fs.Bool("hotfix", false, "hotfix track (needs --from)")
		branch := fs.String("branch", "", "existing branch for all repos, or repo=branch,repo=branch")
		docs := fs.String("docs", "", "existing docs folder to copy into the task docs; marks the track adopted")
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 2, "new <task> <repo,repo> [--from ref] [--hotfix] [--branch b] [--docs dir]"); err != nil {
			return err
		}
		t, err := w.New(pos[0], core.NewOpts{Repos: strings.Split(pos[1], ","), From: *from, Hotfix: *hotfix, Branch: *branch, Docs: *docs})
		if t != nil {
			for _, r := range t.Repos {
				fmt.Printf("  %-12s %s from %s (%.7s)\n", r.Name, r.Branch, r.Base, r.BaseCommit)
			}
			fmt.Printf("track ready: %s/tracks/%s\ndocs: %s/%s\n", w.Root, t.Task, w.Root, t.Docs)
		}
		return err

	case "ls":
		if _, err := parse(fs, args); err != nil {
			return err
		}
		tracks, err := w.Tracks()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "TRACK\tREPOS\tBRANCH\tSTAGE\tCHANGES")
		for _, t := range tracks {
			var names, status []string
			for _, s := range w.State(t) {
				names = append(names, s.Name)
				switch {
				case s.Err != nil:
					status = append(status, s.Name+": error")
				case s.Changed > 0 || s.Ahead > 0 || s.Behind > 0:
					status = append(status, fmt.Sprintf("%s: %d changed +%d -%d", s.Name, s.Changed, s.Ahead, s.Behind))
				}
			}
			st := "clean"
			if len(status) > 0 {
				st = strings.Join(status, "; ")
			}
			branch := ""
			if len(t.Repos) > 0 {
				branch = t.Repos[0].Branch
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", t.Task, strings.Join(names, ","), branch, core.StageLabel[t.Status], st)
		}
		return tw.Flush()

	case "submit", "approve":
		back := fs.Bool("back", false, "send the track back instead of approving")
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 1, cmd+" <task>"); err != nil {
			return err
		}
		var t *core.Track
		if cmd == "submit" {
			t, err = w.Submit(pos[0])
		} else {
			t, err = w.Approve(pos[0], *back)
		}
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", t.Task, core.StageLabel[t.Status])
		return nil

	case "open":
		agent := fs.String("agent", "", "agent command (default: agent in .siding.local.toml, else claude)")
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 1, "open <task> [--agent cmd]"); err != nil {
			return err
		}
		return w.Session(pos[0], *agent)

	case "ui":
		port := fs.Int("port", 4747, "port on 127.0.0.1")
		if _, err := parse(fs, args); err != nil {
			return err
		}
		addr := fmt.Sprintf("127.0.0.1:%d", *port)
		fmt.Printf("siding board: http://%s (ctrl-c to stop)\n", addr)
		return http.ListenAndServe(addr, ui.Handler(w))

	case "report", "pr":
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 1, cmd+" <task>"); err != nil {
			return err
		}
		if cmd == "pr" {
			res, err := w.PR(pos[0])
			if err != nil {
				return err
			}
			failed := 0
			for _, r := range res {
				if r.Err != nil {
					failed++
					fmt.Printf("%-12s FAILED %s: %v\n", r.Repo, r.Note, r.Err)
					continue
				}
				fmt.Printf("%-12s %s %s\n", r.Repo, r.Note, r.URL)
			}
			if failed > 0 {
				return fmt.Errorf("%d repo(s) failed", failed)
			}
			return nil
		}
		t, err := w.LoadTrack(pos[0])
		if err != nil {
			return err
		}
		fmt.Printf("track %s · %s\n", t.Task, core.StageLabel[t.Status])
		for _, r := range w.Report(t) {
			fmt.Printf("\n## %s  %s  base %s (%.7s)\n", r.Name, r.Branch, r.Base, r.BaseCommit)
			if r.Err != nil {
				fmt.Println("error:", r.Err)
				continue
			}
			fmt.Printf("%d commit(s)\n", len(r.Commits))
			for _, c := range r.Commits {
				fmt.Println("  " + c)
			}
			if r.DiffStat != "" {
				fmt.Println(r.DiffStat)
			}
			for _, u := range r.Untracked {
				fmt.Println("  untracked:", u)
			}
		}
		return nil

	case "review":
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 1, "review <task>"); err != nil {
			return err
		}
		t, err := w.Review(pos[0])
		if t != nil {
			fmt.Printf("review track ready: %s/tracks/%s\nnext: siding open %s\n", w.Root, t.Task, t.Task)
		}
		return err

	case "next":
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 1, "next <task>"); err != nil {
			return err
		}
		t, err := w.Next(pos[0])
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s; previous prd/plan/testing moved to a numbered folder in %s\n", t.Task, core.StageLabel[t.Status], t.Docs)
		return nil

	case "done":
		force := fs.Bool("force", false, "remove even with uncommitted or unpushed work")
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 1, "done <task> [--force]"); err != nil {
			return err
		}
		if err := w.Done(pos[0], *force); err != nil {
			return err
		}
		fmt.Println("track removed; docs kept")
		return nil

	case "adapter":
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if len(pos) != 2 || pos[0] != "install" {
			return fmt.Errorf("usage: siding adapter install <%s>", strings.Join(core.AdapterNames(), "|"))
		}
		if err := w.AdapterInstall(pos[1]); err != nil {
			return err
		}
		fmt.Println("adapter installed:", pos[1])
		return nil

	case "learn":
		return learn(w)

	case "doctor":
		problems := w.Doctor()
		for _, p := range problems {
			fmt.Printf("%s: %s\n", p.Kind, p.Detail)
		}
		if len(problems) > 0 {
			return fmt.Errorf("%d problem(s)", len(problems))
		}
		fmt.Println("ok")
		return nil

	case "guard": // Claude Code PreToolUse hook: exit 2 blocks the tool call
		if err := w.Guard(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return nil
	}
	return fmt.Errorf("unknown command %q\n\n%s", cmd, usage)
}

// learn walks the inbox one proposal at a time.
func learn(w *core.Workspace) error {
	props, err := w.Inbox()
	if err != nil {
		return err
	}
	if len(props) == 0 {
		fmt.Println("inbox empty")
		return nil
	}
	in := bufio.NewReader(os.Stdin)
	for i, p := range props {
		fmt.Printf("[%d/%d] %s: %s\n      source: %s, commit %s\n", i+1, len(props), p.Repo, p.Body, p.Track, p.Commit)
		for {
			fmt.Print("      (a)ccept  (e)dit  (s)kip  (d)elete > ")
			line, err := in.ReadString('\n')
			if err != nil && line == "" {
				return nil // EOF: stop, remaining proposals stay in the inbox
			}
			var dest string
			switch strings.TrimSpace(line) {
			case "a":
				dest, err = w.Accept(p.File, "")
			case "e":
				fmt.Print("      new text > ")
				text, _ := in.ReadString('\n')
				if strings.TrimSpace(text) == "" {
					continue
				}
				dest, err = w.Accept(p.File, text)
			case "s":
			case "d":
				err = w.Reject(p.File)
			default:
				continue
			}
			if err != nil {
				fmt.Println("      error:", err)
			} else if dest != "" {
				fmt.Println("      ->", dest)
			}
			break
		}
	}
	return nil
}
