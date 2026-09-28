package core

import (
	"fmt"
	"os"
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

func (w *Workspace) setStage(t *Track) error {
	if err := w.saveTrack(t); err != nil {
		return err
	}
	return w.WriteAgentFiles(t)
}
