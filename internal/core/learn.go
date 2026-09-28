package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Proposal is one lesson in knowledge/inbox/, written by the retro skill.
type Proposal struct {
	File   string // base name in knowledge/inbox/
	Repo   string // repo name, or "skill:<name>" for a new skill
	Track  string
	Date   string
	Commit string
	Body   string
}

// Inbox lists the proposals waiting for review, oldest file name first.
func (w *Workspace) Inbox() ([]Proposal, error) {
	files, err := filepath.Glob(w.path("knowledge", "inbox", "*.md"))
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	var out []Proposal
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		p := parseProposal(string(b))
		p.File = filepath.Base(f)
		out = append(out, p)
	}
	return out, nil
}

// parseProposal reads "key: value" frontmatter between --- lines; the rest is the body.
func parseProposal(s string) Proposal {
	var p Proposal
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "---\n"); ok {
		if front, body, ok := strings.Cut(rest, "\n---"); ok {
			for _, l := range strings.Split(front, "\n") {
				k, v, _ := strings.Cut(l, ":")
				v, _, _ = strings.Cut(v, "#")
				v = strings.TrimSpace(v)
				switch strings.TrimSpace(k) {
				case "repo":
					p.Repo = v
				case "track":
					p.Track = v
				case "date":
					p.Date = v
				case "commit":
					p.Commit = v
				}
			}
			s = body
		}
	}
	p.Body = strings.TrimSpace(s)
	return p
}

// Accept moves a proposal into permanent knowledge. body replaces the proposal's text when
// non-empty (the "edit" choice). Repo facts are appended to knowledge/repos/<repo>.md with a
// verified marker; "skill:<name>" becomes knowledge/skills/<name>.md.
func (w *Workspace) Accept(file, body string) (string, error) {
	src, err := w.inboxFile(file)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	p := parseProposal(string(b))
	if body = strings.TrimSpace(body); body != "" {
		p.Body = body
	}
	if p.Body == "" {
		return "", errors.New("empty proposal")
	}
	var dest, text string
	if name, ok := strings.CutPrefix(p.Repo, "skill:"); ok {
		if err := validName("skill", name); err != nil {
			return "", err
		}
		dest = w.path("knowledge", "skills", name+".md")
		if _, err := os.Stat(dest); err == nil {
			return "", fmt.Errorf("skill %s already exists; edit %s by hand", name, dest)
		}
		text = p.Body + "\n"
	} else {
		if err := validName("repo", p.Repo); err != nil {
			return "", err
		}
		dest = w.path("knowledge", "repos", p.Repo+".md")
		text = strings.TrimSpace("- "+strings.ReplaceAll(p.Body, "\n", "\n  ")+" "+marker(p)) + "\n"
		if _, err := os.Stat(dest); err != nil {
			text = "# " + p.Repo + "\n\n" + text
		}
	}
	f, err := os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return dest, os.Remove(src)
}

// marker records where a fact came from; doctor uses the facts' paths to flag staleness.
func marker(p Proposal) string {
	f := strings.Fields(p.Commit + " " + p.Track + " " + p.Date)
	if len(f) == 0 {
		return ""
	}
	return "<!-- verified " + strings.Join(f, " ") + " -->"
}

// Reject deletes a proposal.
func (w *Workspace) Reject(file string) error {
	src, err := w.inboxFile(file)
	if err != nil {
		return err
	}
	return os.Remove(src)
}

func (w *Workspace) inboxFile(file string) (string, error) {
	if file != filepath.Base(file) || !strings.HasSuffix(file, ".md") {
		return "", fmt.Errorf("invalid inbox file %q", file)
	}
	return w.path("knowledge", "inbox", file), nil
}
