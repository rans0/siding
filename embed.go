// Package siding holds the files embedded into the binary: templates and starter knowledge.
package siding

import "embed"

//go:embed all:starter-knowledge templates web
var Assets embed.FS
