package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rans0/siding/internal/core"
)

// usageLine: "  siding <cmd> <args>   <description>", description after a run of 3+ spaces.
var usageLine = regexp.MustCompile(`^  siding (\S+)(?: .*?)?(?: {3,}(.+))?$`)

// commands returns "name:description" per command, parsed from usage so it never drifts.
func commands() []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range strings.Split(usage, "\n") {
		m := usageLine.FindStringSubmatch(l)
		if m == nil || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1]+":"+strings.ReplaceAll(m[2], ":", `\:`))
	}
	return out
}

// complete prints candidates for the shell scripts below, one per line, as "name:description".
// Outside a workspace it prints only commands and adapters: completion must never error.
func complete(kind string) {
	var out []string
	switch kind {
	case "commands":
		out = commands()
	case "adapters":
		out = core.AdapterNames()
	default:
		w, err := core.Open(".")
		if err != nil {
			return
		}
		switch kind {
		case "tasks":
			tracks, _ := w.Tracks()
			for _, t := range tracks {
				out = append(out, t.Task+":"+strings.ReplaceAll(core.StageLabel[t.Status], ":", `\:`))
			}
		case "finished": // done tasks, for review
			found, _ := filepath.Glob(filepath.Join(w.Root, "docs", "????-??-??_*", "track.json"))
			for _, f := range found {
				out = append(out, filepath.Base(filepath.Dir(f))[11:])
			}
		case "repos":
			repos, _ := w.Repos()
			for _, r := range repos {
				if !r.RefOnly {
					out = append(out, r.Name)
				}
			}
		}
	}
	for _, s := range out {
		fmt.Println(s)
	}
}

const zshCompletion = `#compdef siding
# siding completion for zsh. Add to ~/.zshrc (after compinit): source <(siding completion zsh)
_siding() {
  local -a items
  _siding_list() { items=("${(@f)$(siding __complete $1 2>/dev/null)}"); [[ -n $items ]] && _describe $2 items; }
  if (( CURRENT == 2 )); then _siding_list commands command; return; fi
  if [[ $words[CURRENT] == -* ]]; then
    case $words[2] in
      new) _values flag '--from[base ref, or repo=ref,repo=ref]' '--hotfix[hotfix track (needs --from)]' \
             '--branch[branch name as is, or repo=branch,...]' '--docs[existing docs folder to adopt]' ;;
      approve) _values flag '--back[send the track back to the stage before]' ;;
      done) _values flag '--force[remove even with uncommitted or unpushed work]' ;;
      pr) _values flag '--push-only[commit and push, open no PR]' ;;
      open) _values flag '--agent[agent command to start]' ;;
      ui) _values flag '--port[port on 127.0.0.1]' ;;
      repo) _values flag '--ref-only[read-only reference repo]' ;;
    esac
    return
  fi
  case $words[2] in
    submit|approve|open|report|pr|next|done) (( CURRENT == 3 )) && _siding_list tasks task ;;
    review) (( CURRENT == 3 )) && _siding_list finished 'finished task' ;;
    new) (( CURRENT == 4 )) && _values -s , repo ${(f)"$(siding __complete repos 2>/dev/null)"}
         [[ $words[CURRENT-1] == --docs ]] && _files -/ ;;
    repo) if (( CURRENT == 3 )); then _values subcommand 'add[clone a url, or copy a local clone]' 'ls[repos and the tracks using them]'
          else _files -/; fi ;;
    adapter) if (( CURRENT == 3 )); then _values subcommand 'install[tools, skills, harness files]'
             else _siding_list adapters harness; fi ;;
    open) [[ $words[CURRENT-1] == --agent ]] && _command_names ;;
    completion) (( CURRENT == 3 )) && _values shell zsh bash ;;
    init|adopt) _files -/ ;;
  esac
}
# Autoloaded from fpath (brew): complete now; sourced: register.
if [[ $zsh_eval_context[-1] == loadautofunc ]]; then _siding "$@"; else compdef _siding siding; fi
`

const bashCompletion = `# siding completion for bash. Add to ~/.bashrc: eval "$(siding completion bash)"
_siding() {
  local cur=${COMP_WORDS[COMP_CWORD]} prev=${COMP_WORDS[COMP_CWORD-1]} kind= words=
  if (( COMP_CWORD == 1 )); then kind=commands
  elif [[ $cur == -* ]]; then case ${COMP_WORDS[1]} in
    new) words="--from --hotfix --branch --docs" ;;
    approve) words=--back ;;
    done) words=--force ;;
    pr) words=--push-only ;;
    open) words=--agent ;;
    ui) words=--port ;;
    repo) words=--ref-only ;;
  esac
  else case ${COMP_WORDS[1]} in
    submit|approve|report|pr|next|done) (( COMP_CWORD == 2 )) && kind=tasks ;;
    open) if [[ $prev == --agent ]]; then COMPREPLY=($(compgen -c -- "$cur")); return; fi
          (( COMP_CWORD == 2 )) && kind=tasks ;;
    review) (( COMP_CWORD == 2 )) && kind=finished ;;
    new) if [[ $prev == --docs ]]; then COMPREPLY=($(compgen -d -- "$cur")); return; fi
         if (( COMP_CWORD == 3 )); then # comma list: complete the part after the last comma
           local done=${cur%${cur##*,}}
           COMPREPLY=($(compgen -P "$done" -W "$(siding __complete repos 2>/dev/null)" -- "${cur##*,}"))
           compopt -o nospace 2>/dev/null; return
         fi ;;
    adapter) if (( COMP_CWORD == 2 )); then words=install; else kind=adapters; fi ;;
    repo) if (( COMP_CWORD == 2 )); then words="add ls"; else COMPREPLY=($(compgen -d -- "$cur")); return; fi ;;
    completion) (( COMP_CWORD == 2 )) && words="zsh bash" ;;
    init|adopt) COMPREPLY=($(compgen -d -- "$cur")); return ;;
  esac; fi
  [[ -n $kind ]] && words=$(siding __complete $kind 2>/dev/null | cut -d: -f1)
  COMPREPLY=($(compgen -W "$words" -- "$cur"))
}
complete -F _siding siding
`
