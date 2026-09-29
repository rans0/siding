package main

import (
	"strings"
	"testing"
)

// Every command in usage completes, each with a description.
func TestCommands(t *testing.T) {
	got := commands()
	for _, want := range []string{"new:create a track", "next:after the PR merges", "approve:you\\: pass the gate"} {
		if !strings.Contains(strings.Join(got, "\n"), want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	for _, c := range got {
		if strings.HasSuffix(c, ":") {
			t.Errorf("%s has no description", c)
		}
	}
}
