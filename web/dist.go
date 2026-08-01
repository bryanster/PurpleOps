// Package web carries the built single-page app into the server binary.
//
// The sources next to this file are the app itself (see README.md); `npm run
// build` compiles them into web/dist, and [Dist] is how the Go half reaches it.
//
// # Why a build tag
//
// web/dist is build output: it is in .gitignore and absent from a fresh
// checkout, but //go:embed fails to compile when its pattern matches nothing.
// So the real embed is behind the `spa` build tag, and the default build gets a
// one-page placeholder instead. That way `go build ./...` and `go test ./...`
// work in a checkout where the frontend has never been built, which is most of
// them — and `make build`, which runs `npm run build` first, passes `-tags spa`
// and produces the single binary a release is.
//
// The committed-placeholder alternative does not survive contact with Vite:
// `npm run build` empties dist before writing to it, so a placeholder file
// there would be deleted by every build and leave the working tree dirty.
package web

import "io/fs"

// Dist returns the user interface this binary carries, and whether it is the
// real one.
//
// false means the binary was built without the `spa` build tag: the filesystem
// is a single page explaining that, rather than the app. It is never nil and
// always has an index.html, so a caller serves it the same way either way — and
// the difference is something to log at startup, not to branch on.
func Dist() (fs.FS, bool) { return distFS, distIsSPA }

// sub roots fsys at dir.
//
// The error is unreachable: dir is a literal in this package and satisfies
// fs.ValidPath. Returning the unrooted filesystem rather than panicking keeps
// an impossible case from being the one thing that can kill the process at
// init — the caller then fails to find index.html and says so.
func sub(fsys fs.FS, dir string) fs.FS {
	rooted, err := fs.Sub(fsys, dir)
	if err != nil {
		return fsys
	}
	return rooted
}
