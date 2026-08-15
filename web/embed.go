// Package web bundles the static dashboard (HTML/CSS/JS) into the binary via
// go:embed so the whole tool ships as a single self-contained executable.
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html style.css app.js
var assets embed.FS

// FS returns the embedded static asset tree.
func FS() fs.FS {
	return assets
}
