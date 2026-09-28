// Command siding: one task, one track, across every repo it touches, with any agent.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rans0/siding/internal/core"
)

const usage = `siding: multi-repo, multi-task workspace manager for coding agents

  siding init [dir]                          create a workspace
  siding repo add <url> [--ref-only]         clone into repos/
  siding repo ls                             repos and the tracks using them
  siding new <task> <repos> [--from ref|a=x,b=y] [--hotfix]
  siding ls                                  all tracks
  siding submit <task>                       agent: move the track to its next gate
  siding approve <task> [--back]             you: pass the gate, or send the track back
  siding done <task> [--force]               remove the track's worktrees (needs log.md; docs stay)
  siding adapter install <agents|claude>     set up required tools, skills, harness files
  siding doctor                              check required tools and skills
`

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
			name, err := w.RepoAdd(pos[1], *refOnly)
			if err == nil {
				fmt.Println("added repos/" + name)
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

	case "new":
		from := fs.String("from", "", "base ref for all repos, or repo=ref,repo=ref")
		hotfix := fs.Bool("hotfix", false, "hotfix track (needs --from)")
		pos, err := parse(fs, args)
		if err != nil {
			return err
		}
		if err := need(pos, 2, "new <task> <repo,repo> [--from ref] [--hotfix]"); err != nil {
			return err
		}
		t, err := w.New(pos[0], core.NewOpts{Repos: strings.Split(pos[1], ","), From: *from, Hotfix: *hotfix})
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

	case "doctor":
		missing := w.MissingRequired()
		for _, m := range missing {
			fmt.Println("missing:", m)
		}
		if len(missing) > 0 {
			return fmt.Errorf("%d problem(s); run `siding adapter install <harness>`", len(missing))
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
