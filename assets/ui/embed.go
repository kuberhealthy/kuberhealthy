package ui

import (
	"embed"
	"io/fs"
)

// dist contains the compiled Svelte assets.
//
//go:embed all:dist
var dist embed.FS

// Dist exposes the embedded UI files.
var Dist, _ = fs.Sub(dist, "dist")
