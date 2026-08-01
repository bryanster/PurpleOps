package httpapi

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bryanster/purpleops/internal/httpapi/gen"
	"github.com/bryanster/purpleops/internal/store/storetest"
)

// The UI these tests serve. A hand-built filesystem rather than the real
// web/dist: the tests then say what they depend on, run in a checkout where the
// frontend has never been built, and do not change meaning when someone adds a
// font.
const (
	testIndexHTML = `<!doctype html><html><head>` +
		`<script type="module" src="/assets/index-lNm06wpq.js"></script>` +
		`</head><body><div id="root"></div></body></html>`
	testScript = "console.log('purpleops')\n"
)

func testUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                        {Data: []byte(testIndexHTML)},
		"theme-bootstrap.js":                {Data: []byte("// sets the theme\n")},
		"favicon.svg":                       {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		"site.webmanifest":                  {Data: []byte(`{"name":"PurpleOps"}`)},
		"assets/index-lNm06wpq.js":          {Data: []byte(testScript)},
		"assets/index-By36rZtA.css":         {Data: []byte(":root{}\n")},
		"assets/geist-latin-BgDaEnEv.woff2": {Data: []byte("wOF2")},
		"assets/logo.png":                   {Data: []byte("\x89PNG")},
	}
}

// newUIServer builds the real chain with a UI attached.
func newUIServer(t *testing.T) http.Handler {
	t.Helper()
	return newUIServerWith(t, testUI())
}

func newUIServerWith(t *testing.T, ui fs.FS) http.Handler {
	t.Helper()

	logs := &logBuffer{}
	server, err := NewServer(Deps{
		Config: testConfig(t),
		Store:  storetest.New(t),
		Logger: logs.logger(),
		UI:     ui,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return server
}

func TestTheRootPathServesTheIndexPage(t *testing.T) {
	t.Parallel()

	recorder := get(newUIServer(t), "/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d\nbody: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got, want := recorder.Body.String(), testIndexHTML; got != want {
		t.Errorf("body = %q, want the index page", got)
	}
	if got, want := recorder.Header().Get("Content-Type"), "text/html; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
}

// A route that only exists in the browser's router. The server has never heard
// of it and must still answer with the app, or every deep link and every page
// refresh is a 404.
func TestAClientSideRouteServesTheIndexPage(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)
	for _, target := range []string{
		"/engagements/123",
		"/engagements/018f4c8e-0000-7000-8000-000000000000/findings",
		"/settings",
		"/some/deeply/nested/thing",
	} {
		recorder := get(server, target)
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want %d", target, recorder.Code, http.StatusOK)
		}
		if recorder.Body.String() != testIndexHTML {
			t.Errorf("GET %s: body is not the index page: %q", target, recorder.Body.String())
		}
	}
}

// The regression this ticket exists to prevent. An API path the server does not
// have must stay JSON: a client that gets HTML where it expects a problem
// document fails somewhere far away from the mistake.
func TestAnUnknownAPIPathIsAProblemAndNotTheSPA(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)
	for _, target := range []string{
		"/api/v1/nope",              // the version this server serves
		"/api/v1/engagements/1/2/3", // a plausible path that does not exist
		"/api/v2/engagements",       // a client built against a later version
		"/api/",
		"/api",
	} {
		recorder := get(server, target)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want %d\nbody: %s",
				target, recorder.Code, http.StatusNotFound, recorder.Body.String())
			continue
		}
		if strings.Contains(recorder.Body.String(), "<html") {
			t.Errorf("GET %s: answered with HTML:\n%s", target, recorder.Body.String())
			continue
		}
		problem := decodeProblem(t, recorder)
		if problem.Code != gen.ProblemCodeNotFound {
			t.Errorf("GET %s: code = %q, want %q", target, problem.Code, gen.ProblemCodeNotFound)
		}
	}
}

// The API's own 404 must not change shape because a UI is attached. Without a
// UI the router's NotFound answers it; with one, the sub-router's does.
func TestTheAPIAnswersTheSameWithAndWithoutAUI(t *testing.T) {
	t.Parallel()

	withUI := get(newUIServer(t), "/api/v1/nope")
	withoutUI, _ := newTestServer(t)

	if got, want := withUI.Code, get(withoutUI, "/api/v1/nope").Code; got != want {
		t.Errorf("status with a UI = %d, without = %d", got, want)
	}
}

func TestTheAPIPrefixCoversTheBasePath(t *testing.T) {
	t.Parallel()

	if !isAPIPath(BasePath + "/health") {
		t.Fatalf("isAPIPath(%q) is false; the SPA would answer for the API's own paths",
			BasePath+"/health")
	}
}

func TestAHashedAssetIsCachedForeverAndTheIndexIsNot(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)

	asset := get(server, "/assets/index-lNm06wpq.js")
	if asset.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", asset.Code, http.StatusOK)
	}
	if got, want := asset.Body.String(), testScript; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got, want := asset.Header().Get("Cache-Control"), immutableCache; got != want {
		t.Errorf("asset Cache-Control = %q, want %q", got, want)
	}

	index := get(server, "/")
	if got, want := index.Header().Get("Cache-Control"), revalidateCache; got != want {
		t.Errorf("index.html Cache-Control = %q, want %q", got, want)
	}

	// Copied from web/public/, so its name never changes and a year of caching
	// would make it impossible to fix.
	unhashed := get(server, "/theme-bootstrap.js")
	if got, want := unhashed.Header().Get("Cache-Control"), revalidateCache; got != want {
		t.Errorf("theme-bootstrap.js Cache-Control = %q, want %q", got, want)
	}

	// In assets/ but with no hash in its name — it came from public/assets/.
	stray := get(server, "/assets/logo.png")
	if got, want := stray.Header().Get("Cache-Control"), revalidateCache; got != want {
		t.Errorf("assets/logo.png Cache-Control = %q, want %q", got, want)
	}
}

func TestAMatchingIfNoneMatchIsNotModified(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)
	for _, target := range []string{"/assets/index-lNm06wpq.js", "/", "/engagements/123"} {
		first := get(server, target)
		etag := first.Header().Get("ETag")
		if etag == "" {
			t.Fatalf("GET %s: no ETag on the first response", target)
		}

		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.Header.Set("If-None-Match", etag)
		second := do(server, request)

		if second.Code != http.StatusNotModified {
			t.Errorf("GET %s with If-None-Match: status = %d, want %d",
				target, second.Code, http.StatusNotModified)
		}
		if second.Body.Len() != 0 {
			t.Errorf("GET %s with If-None-Match: body is %d bytes, want none", target, second.Body.Len())
		}
		// The caching rules still apply to the response that says "unchanged",
		// or the next request has nothing to go on.
		if got, want := second.Header().Get("Cache-Control"), first.Header().Get("Cache-Control"); got != want {
			t.Errorf("GET %s: 304 Cache-Control = %q, want %q", target, got, want)
		}
	}
}

func TestADifferentIfNoneMatchSendsTheFile(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/assets/index-lNm06wpq.js", nil)
	request.Header.Set("If-None-Match", `"0000000000000000"`)
	recorder := do(newUIServer(t), request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != testScript {
		t.Errorf("body = %q, want the asset", recorder.Body.String())
	}
}

// Two files with different contents must not share an ETag, or a deploy leaves
// browsers holding the wrong one.
func TestETagsFollowTheContents(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)
	script := get(server, "/assets/index-lNm06wpq.js").Header().Get("ETag")
	style := get(server, "/assets/index-By36rZtA.css").Header().Get("ETag")

	if script == style {
		t.Errorf("two different files share the ETag %s", script)
	}
	if !strings.HasPrefix(script, `"`) || !strings.HasSuffix(script, `"`) {
		t.Errorf("ETag = %s, want it quoted", script)
	}
}

func TestFilesAreServedWithTheirOwnContentType(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)
	for target, want := range map[string]string{
		"/favicon.svg":                       "image/svg+xml",
		"/assets/geist-latin-BgDaEnEv.woff2": "font/woff2",
		"/site.webmanifest":                  "application/manifest+json",
		"/assets/index-By36rZtA.css":         "text/css; charset=utf-8",
		"/assets/index-lNm06wpq.js":          "text/javascript; charset=utf-8",
	} {
		if got := get(server, target).Header().Get("Content-Type"); got != want {
			t.Errorf("GET %s: Content-Type = %q, want %q", target, got, want)
		}
	}
}

// Only GET and HEAD fall back to the app. Anything else on an unknown path is
// the router's business, and it answers in JSON.
func TestOnlyReadsFallBackToTheSPA(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		recorder := do(server, httptest.NewRequest(method, "/engagements/123", nil))

		if recorder.Code != http.StatusMethodNotAllowed && recorder.Code != http.StatusNotFound {
			t.Errorf("%s /engagements/123: status = %d, want 405 or 404\nbody: %s",
				method, recorder.Code, recorder.Body.String())
			continue
		}
		if strings.Contains(recorder.Body.String(), "<html") {
			t.Errorf("%s /engagements/123: answered with the app:\n%s", method, recorder.Body.String())
		}
	}
}

func TestHeadServesTheHeadersWithoutTheBody(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)
	head := do(server, httptest.NewRequest(http.MethodHead, "/assets/index-lNm06wpq.js", nil))

	if head.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", head.Code, http.StatusOK)
	}
	if head.Body.Len() != 0 {
		t.Errorf("body is %d bytes, want none", head.Body.Len())
	}
	if got := head.Header().Get("ETag"); got == "" {
		t.Error("no ETag on a HEAD response")
	}
}

// The one bullet in M0B-010 that is a security property rather than a
// behaviour: nothing outside the embedded filesystem is reachable, however the
// path is written.
func TestPathTraversalCannotEscapeTheUIFilesystem(t *testing.T) {
	t.Parallel()

	server := newUIServer(t)
	for _, target := range []string{
		"/../go.mod",
		"/../../etc/passwd",
		"/assets/../../go.mod",
		"/%2e%2e/go.mod",
		"/%2e%2e%2f%2e%2e%2fgo.mod",
		"/assets/%2e%2e/%2e%2e/go.mod",
		"/..%2fgo.mod",
	} {
		recorder := get(server, target)

		if recorder.Code != http.StatusOK {
			// Rejected outright is a fine answer too; what matters is the body.
			continue
		}
		if body := recorder.Body.String(); body != testIndexHTML {
			t.Errorf("GET %s: served something other than the app:\n%s", target, body)
		}
	}
}

// An empty web/dist, or an embed pattern that matched nothing, is a broken
// build. It fails at startup rather than as a 404 on every page.
func TestAUIWithoutAnIndexPageIsAStartupError(t *testing.T) {
	t.Parallel()

	_, err := NewServer(Deps{
		Config: testConfig(t),
		Store:  storetest.New(t),
		UI:     fstest.MapFS{"assets/index-lNm06wpq.js": {Data: []byte(testScript)}},
	})
	if err == nil {
		t.Fatal("NewServer accepted a UI with no index.html")
	}
	if !strings.Contains(err.Error(), indexFile) {
		t.Errorf("error = %q, want it to name %s", err, indexFile)
	}
}

// The SPA is served through the same chain as everything else, so the headers
// that make the CSP work are on the page that has to satisfy it.
func TestTheIndexPageCarriesTheSecurityHeaders(t *testing.T) {
	t.Parallel()

	recorder := get(newUIServer(t), "/engagements/123")

	for header, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": contentSecurityPolicy,
	} {
		if got := recorder.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}
