//go:build !spa

package web

import (
	"embed"
	"io/fs"
)

// The placeholder is a committed source file rather than build output, so this
// half always compiles. See the package comment.
//
//go:embed placeholder
var embedded embed.FS

var (
	distFS    fs.FS = sub(embedded, "placeholder")
	distIsSPA       = false
)
