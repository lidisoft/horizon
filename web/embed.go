// Package web embeds the single-page app (docs/frontend.md). The server
// serves app/ under content-hashed URLs and renders index.html with an import
// map pointing at them (internal/server/web.go).
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html app
var files embed.FS

// FS returns the embedded files: index.html and app/.
func FS() fs.FS { return files }
