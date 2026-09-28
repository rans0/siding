package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardPayloads(t *testing.T) {
	ws, _ := filepath.EvalSymlinks(t.TempDir())
	w := &Workspace{Root: ws}
	os.MkdirAll(filepath.Join(ws, "repos/app"), 0o755)
	in := filepath.Join(ws, "repos/app/a.go")
	out := filepath.Join(ws, "tracks/t/app/a.go")
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	cases := []struct {
		name, payload string
		block         bool
	}{
		{"claude edit", `{"tool_input":{"file_path":` + q(in) + `}}`, true},
		{"claude edit track", `{"tool_input":{"file_path":` + q(out) + `}}`, false},
		{"codex apply_patch", `{"tool_name":"apply_patch","tool_input":{"command":` + q("*** Begin Patch\n*** Update File: "+in+"\n@@\n-a\n+b\n*** End Patch") + `}}`, true},
		{"codex apply_patch track", `{"tool_input":{"command":` + q("*** Begin Patch\n*** Add File: "+out+"\n+x\n*** End Patch") + `}}`, false},
		{"cursor shell", `{"hook_event_name":"beforeShellExecution","command":"siding approve t"}`, true},
		{"cursor shell ok", `{"command":"siding submit t"}`, false},
		{"opencode edit", `{"tool_input":{"filePath":` + q(in) + `}}`, true},
		{"garbage", `not json`, false},
	}
	for _, c := range cases {
		if err := w.Guard(strings.NewReader(c.payload)); (err != nil) != c.block {
			t.Errorf("%s: block=%v, err=%v", c.name, c.block, err)
		}
	}
}

func TestAdapterFiles(t *testing.T) {
	ws := t.TempDir()
	w := &Workspace{Root: ws}
	tr := &Track{Task: "t", Status: StageBuild}
	os.MkdirAll(w.trackDir("t"), 0o755)
	for name, want := range map[string][]string{
		"codex":    {".codex/hooks.json", ".agents/skills/siding/SKILL.md"},
		"cursor":   {".cursor/hooks.json", ".cursor/commands/siding.md"},
		"opencode": {".opencode/commands/siding.md", ".opencode/plugins/siding-guard.js"},
	} {
		if err := adapters[name].writeTrack(w, tr); err != nil {
			t.Fatal(name, err)
		}
		for _, f := range want {
			b, err := os.ReadFile(filepath.Join(w.trackDir("t"), f))
			if err != nil {
				t.Errorf("%s: %v", name, err)
			} else if strings.HasSuffix(f, ".json") && !json.Valid(b) {
				t.Errorf("%s: invalid JSON", f)
			}
		}
	}
}
