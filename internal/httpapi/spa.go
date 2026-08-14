package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// indexFile is the app's entry point, and the answer to every path the server
// does not otherwise recognise: the router the SPA runs is in the browser, so
// /engagements/018f… is a real page even though no file of that name exists.
const indexFile = "index.html"

// apiPrefix is everything the SPA must never answer for. It is deliberately
// wider than [BasePath]: /api/v2/engagements is a client built against a
// version this server does not have, and telling it so in JSON is far more use
// than 200 and a page of HTML. TestTheAPIPrefixCoversTheBasePath keeps the two
// in step.
const apiPrefix = "/api/"

// Cache-Control for the two kinds of file a Vite build produces.
const (
	// immutableCache is for content-addressed assets. The name changes whenever
	// the bytes do, so a year is safe and anything shorter is a request that
	// could not have found anything new.
	immutableCache = "public, max-age=31536000, immutable"

	// revalidateCache is for everything else — index.html above all. "no-cache"
	// does not mean "do not store": the browser keeps the file and asks whether
	// it is still current, which the ETag answers with a 304. Without it, a
	// deploy is invisible to every tab that already has the old index.html, and
	// the asset names in it are the ones that no longer exist.
	revalidateCache = "no-cache"
)

// hashedName matches the content hash Vite puts in an asset's file name
// (index-lNm06wpq.js, geist-latin-wght-normal-BgDaEnEv.woff2). The hash is
// base64url, so it can itself contain "-": the pattern is loose on purpose and
// only has to distinguish "there is a hash in here" from "there is not".
var hashedName = regexp.MustCompile(`-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+$`)

// contentTypes is this application's answer for the extensions it ships, and it
// is consulted before mime.TypeByExtension — which reads /etc/mime.types and
// the Windows registry, and so gives an answer that depends on the machine the
// binary is running on. A stylesheet served as text/plain is ignored by every
// browser, and X-Content-Type-Options: nosniff means it stays ignored.
var contentTypes = map[string]string{
	".css":         "text/css; charset=utf-8",
	".html":        "text/html; charset=utf-8",
	".ico":         "image/vnd.microsoft.icon",
	".jpeg":        "image/jpeg",
	".jpg":         "image/jpeg",
	".js":          "text/javascript; charset=utf-8",
	".json":        "application/json",
	".map":         "application/json",
	".mjs":         "text/javascript; charset=utf-8",
	".otf":         "font/otf",
	".png":         "image/png",
	".svg":         "image/svg+xml",
	".ttf":         "font/ttf",
	".txt":         "text/plain; charset=utf-8",
	".wasm":        "application/wasm",
	".webmanifest": "application/manifest+json",
	".webp":        "image/webp",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
}

// staticFile is what the server needs to know about one file in the UI
// filesystem. Everything here is computed once, at startup: the filesystem is
// embedded in the binary and cannot change while the process runs, so hashing a
// file per request would be work with a known answer.
//
// The bytes are deliberately not here. They are already in the binary's
// read-only data, and copying them into a map would be a second megabyte of
// heap that never shrinks.
type staticFile struct {
	name         string
	contentType  string
	cacheControl string
	etag         string
}

// spa serves the built single-page app: its files where they exist, and
// index.html for the client-side routes that only exist in the browser.
type spa struct {
	fsys      fs.FS
	files     map[string]staticFile
	index     staticFile
	responder *apierr.Responder
}

// newSPA reads fsys once and builds the table [spa] serves from.
//
// A filesystem without an index.html is a build that went wrong — an empty
// web/dist, or an embed pattern that matched nothing — and it fails here, at
// startup, rather than as a 404 for every page once the process is live.
func newSPA(fsys fs.FS, responder *apierr.Responder) (*spa, error) {
	files := make(map[string]staticFile)
	err := fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		file, err := describe(fsys, name)
		if err != nil {
			return err
		}
		files[name] = file
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("httpapi: read the user interface files: %w", err)
	}

	index, ok := files[indexFile]
	if !ok {
		return nil, fmt.Errorf("httpapi: the user interface has no %s; "+
			"the frontend build did not run, or produced nothing", indexFile)
	}
	return &spa{fsys: fsys, files: files, index: index, responder: responder}, nil
}

// describe computes what is served with one file, including its ETag — the
// SHA-256 of the contents, which makes it strong in the HTTP sense: two
// responses with this ETag are byte-for-byte identical.
func describe(fsys fs.FS, name string) (staticFile, error) {
	file, err := fsys.Open(name)
	if err != nil {
		return staticFile{}, err
	}
	defer file.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return staticFile{}, fmt.Errorf("hash %s: %w", name, err)
	}
	// Half of the digest: 128 bits is far past any chance of two builds of this
	// app colliding, and the value goes out on every response.
	sum := digest.Sum(nil)[:16]

	cache := revalidateCache
	if isImmutable(name) {
		cache = immutableCache
	}
	return staticFile{
		name:         name,
		contentType:  contentType(name),
		cacheControl: cache,
		etag:         `"` + hex.EncodeToString(sum) + `"`,
	}, nil
}

// isImmutable reports whether a file may be cached for a year.
//
// Both conditions matter. Vite writes content-hashed output to assets/ and
// nothing else goes there — but anything dropped into web/public/assets/ would
// be copied there verbatim, unhashed, and a year of caching on a file whose
// name never changes is a deploy that cannot take effect.
func isImmutable(name string) bool {
	return strings.HasPrefix(name, "assets/") && hashedName.MatchString(path.Base(name))
}

// contentType maps a file name onto what the browser is told it is.
func contentType(name string) string {
	ext := path.Ext(name)
	// A source map is index-lNm06wpq.js.map, whose extension by this measure is
	// ".map" — which is in the table. Nothing else here is double-extensioned.
	if known, ok := contentTypes[strings.ToLower(ext)]; ok {
		return known
	}
	if guess := mime.TypeByExtension(ext); guess != "" {
		return guess
	}
	// Deliberately not text/plain: an unrecognised file downloads rather than
	// rendering, which is the safer default for something this server did not
	// expect to be serving.
	return "application/octet-stream"
}

func (s *spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The API's paths are the API's, including the ones it does not have. A
	// mistyped or out-of-date API path that fell through to here gets the same
	// JSON problem as every other API failure; answering it with index.html
	// would hand the client 200 and a page of HTML where it expected a document
	// it can read, and its error handling would fail somewhere unrelated. This
	// is the regression M0B-010 exists to prevent — see
	// TestAnUnknownAPIPathIsAProblemAndNotTheSPA.
	if isAPIPath(r.URL.Path) {
		s.responder.Write(w, r, apierr.NotFound("endpoint", apierr.RedactPath(r.URL.Path)))
		return
	}

	// Cleaning resolves "." and ".." and collapses repeated slashes, so
	// /assets/../../go.mod becomes /go.mod. It is not what makes traversal
	// safe, though: the lookup below is a map whose keys are the files that
	// exist, so a path that escapes the filesystem matches nothing and is
	// answered with index.html like any other unknown path. There is no name
	// this can turn into an open() of something outside fsys.
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")

	if file, ok := s.files[name]; ok {
		s.serve(w, r, file)
		return
	}
	// A client-side route, or a typo. Both are pages: the app renders its own
	// "not found" for the second, which is a better answer than the server's.
	s.serve(w, r, s.index)
}

// isAPIPath reports whether a path belongs to the API rather than to the app.
func isAPIPath(p string) bool {
	return p == strings.TrimSuffix(apiPrefix, "/") || strings.HasPrefix(p, apiPrefix)
}

// serve writes one file.
func (s *spa) serve(w http.ResponseWriter, r *http.Request, file staticFile) {
	opened, err := s.fsys.Open(file.name)
	if err != nil {
		// The table said this file exists, so it did when the server started.
		// Reaching here means the filesystem changed underneath a running
		// process, which is not something a client can be told anything useful
		// about.
		s.responder.Write(w, r, apierr.Internal(fmt.Errorf("open %s: %w", file.name, err)))
		return
	}
	defer opened.Close()

	content, err := readSeeker(opened)
	if err != nil {
		s.responder.Write(w, r, apierr.Internal(fmt.Errorf("read %s: %w", file.name, err)))
		return
	}

	header := w.Header()
	header.Set("Content-Type", file.contentType)
	header.Set("Cache-Control", file.cacheControl)
	header.Set("ETag", file.etag)

	// ServeContent does conditional requests (If-None-Match against the ETag
	// above, answered with a 304 that keeps the caching headers and drops the
	// body), ranges, and HEAD. The zero modification time is deliberate: a file
	// embedded in a binary has none, and a Last-Modified invented at startup
	// would be a different answer from each replica of the same deployment.
	http.ServeContent(w, r, file.name, time.Time{}, content)
}

// readSeeker adapts an open file to what [http.ServeContent] needs.
//
// Every filesystem this is used with — embed.FS, os.DirFS, fstest.MapFS —
// returns files that already seek, so the fallback is for an fs.FS
// implementation that does not. Reading it into memory is acceptable because
// the files here are a frontend build; nothing streams.
func readSeeker(file fs.File) (io.ReadSeeker, error) {
	if seeker, ok := file.(io.ReadSeeker); ok {
		return seeker, nil
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}
