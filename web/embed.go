// Package web embeds the built frontend. Run `make web` (or `npm --prefix web
// run build`) to populate dist/; without a build only the placeholder is
// embedded and the server says so instead of failing to compile.
package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// DistFS returns the built frontend assets rooted at dist/.
func DistFS() (fs.FS, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("embedded frontend: %w", err)
	}

	return sub, nil
}
