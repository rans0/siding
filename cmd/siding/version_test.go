package main

import "testing"

// Release builds keep their stamped version; go install builds fall back to the module version.
func TestResolveVersion(t *testing.T) {
	for _, c := range [][3]string{
		{"0.1.0", "v0.2.0", "0.1.0"},
		{"dev", "v0.1.0", "0.1.0"},
		{"dev", "v0.1.1-0.20261018-abcdef", "0.1.1-0.20261018-abcdef"},
		{"dev", "(devel)", "dev"},
		{"dev", "", "dev"},
	} {
		if got := resolveVersion(c[0], c[1]); got != c[2] {
			t.Errorf("resolveVersion(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}
