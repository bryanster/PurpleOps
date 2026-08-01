//go:build spa

package web

import (
	"embed"
	"io/fs"
)

// The `all:` prefix embeds the files Vite writes whose names begin with "." or
// "_" as well — a manifest under .vite/, a chunk rollup named _commonjsHelpers
// — which the ordinary pattern would silently leave out of the binary.
//
//go:embed all:dist
var embedded embed.FS

var (
	distFS    fs.FS = sub(embedded, "dist")
	distIsSPA       = true
)
