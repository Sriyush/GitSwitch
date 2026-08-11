// Package web holds the frontend served by `gsw ui`.
//
// The assets are embedded into the binary rather than read from disk, so an
// installed gsw is still a single file with nothing to install alongside it and
// no path to get wrong.
//
// They are also hand-written and dependency-free: no npm, no bundler, no build
// step in front of `make build`. The UI is three screens over a small JSON API,
// which a framework would not make meaningfully simpler, and requiring a node
// toolchain to compile a Go binary is a real cost paid by everyone who builds
// the project.
//
// Structure comes from native ES modules instead: app/main.js is the shell, and
// each screen is a folder under app/features/ exporting the same shape. The
// browser resolves the imports itself, so the layout is a bundler's without a
// bundler. If the UI ever outgrows that trade, this package is where a generated
// web/dist would be embedded instead.
package web

import "embed"

// Assets holds the frontend, rooted at the app/ directory.
var (
	//go:embed app
	Assets embed.FS
)
