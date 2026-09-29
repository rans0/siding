package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
)

// Track stages in order. Agents move a track to a gate with Submit; only a human passes it with Approve.
const (
	StagePRD   = "prd"
	StageGate1 = "gate1"
	StageBuild = "build"
	StageGate2 = "gate2"
	StageShip  = "ship"
)

var StageLabel = map[string]string{
	StagePRD:   "prd: drafting",
	StageGate1: "GATE 1: awaiting PRD approval",
	StageBuild: "build",
	StageGate2: "GATE 2: awaiting diff review",
	StageShip:  "ship: approved",
}

// skillStage maps a track stage to the skill stage whose skills and plugins are active.
var skillStage = map[string]string{
	StagePRD: "plan", StageGate1: "plan",
	StageBuild: "build", StageGate2: "review", StageShip: "review",
}

// Submit is called by the agent when it reaches a gate.
func (w *Workspace) Submit(task string) (*Track, error) {
	t, err := w.LoadTrack(task)
	if err != nil {
		return nil, err
	}
	switch t.Status {
	case StagePRD:
		need := []string{"prd.md"}
		if t.Review != nil {
			need = []string{"review.md", "prd.md"}
		}
		for _, f := range need {
			if _, err := os.Stat(w.path(t.Docs, f)); err != nil {
				return nil, fmt.Errorf("write %s first", w.path(t.Docs, f))
			}
		}
		t.Status = StageGate1
	case StageBuild:
		t.Status = StageGate2
	default:
		return nil, fmt.Errorf("track %s is at %q; nothing to submit", task, StageLabel[t.Status])
	}
	return t, w.setStage(t)
}

// Approve passes the gate a track waits at; back sends it back to the stage before.
func (w *Workspace) Approve(task string, back bool) (*Track, error) {
	t, err := w.LoadTrack(task)
	if err != nil {
		return nil, err
	}
	next := map[string][2]string{ // gate: {approve, back}
		StageGate1: {StageBuild, StagePRD},
		StageGate2: {StageShip, StageBuild},
	}[t.Status]
	if next[0] == "" {
		return nil, fmt.Errorf("track %s is not at a gate (%s)", task, StageLabel[t.Status])
	}
	t.Status = next[0]
	if back {
		t.Status = next[1]
	}
	return t, w.setStage(t)
}

// Next starts the next cycle of a shipped track on the same branches (feature B after A):
// the cycle's prd.md, plan.md and testing.md move to docs/<n>/, other docs stay, and the
// base commit moves to the branch tip so report and pr cover only the new work.
func (w *Workspace) Next(task string) (*Track, error) {
	t, err := w.LoadTrack(task)
	if err != nil {
		return nil, err
	}
	if t.Status != StageShip {
		return nil, fmt.Errorf("track %s is at %q; `siding next` needs a shipped cycle", task, StageLabel[t.Status])
	}
	heads := make([]string, len(t.Repos))
	for i, r := range t.Repos { // check every repo before moving anything
		dir := w.path("tracks", task, r.Name)
		if st, err := git(dir, "status", "--porcelain"); err != nil || st != "" {
			return nil, fmt.Errorf("%s has uncommitted changes; commit first (yourself or `siding pr %s`)", r.Name, task)
		}
		if heads[i], err = git(dir, "rev-parse", "HEAD"); err != nil {
			return nil, fmt.Errorf("%s: %w", r.Name, err)
		}
	}
	sum := fileSum(w.path(t.Docs, "log.md"))
	if sum == "" || sum == t.LogSum {
		return nil, fmt.Errorf("log.md has no retro for this cycle; run the retro skill first (%s)", w.path(t.Docs, "log.md"))
	}
	n := 1
	for ; ; n++ {
		if _, err := os.Stat(w.path(t.Docs, strconv.Itoa(n))); err != nil {
			break
		}
	}
	dir := w.path(t.Docs, strconv.Itoa(n))
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil, err
	}
	for _, f := range []string{"review.md", "prd.md", "plan.md", "testing.md"} {
		if err := os.Rename(w.path(t.Docs, f), w.path(t.Docs, strconv.Itoa(n), f)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	for i := range t.Repos {
		t.Repos[i].BaseCommit = heads[i]
	}
	t.Status, t.Adopt, t.Review, t.LogSum = StagePRD, false, nil, sum
	return t, w.setStage(t)
}

// fileSum is the sha256 of a file, or "" when it cannot be read.
func fileSum(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func (w *Workspace) setStage(t *Track) error {
	if err := w.saveTrack(t); err != nil {
		return err
	}
	return w.WriteAgentFiles(t)
}
