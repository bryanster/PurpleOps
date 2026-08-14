package ctid_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/ctid"
)

// The plans are read file by file rather than by downloading the repository
// archive. These tests are about what that fetch asks for — which is the whole
// point of it: an archive request would be several hundred megabytes, and the
// stub fails the test if one is made.

const (
	planURL  = "https://github.com/center-for-threat-informed-defense/adversary_emulation_library/archive/refs/heads/master.zip"
	planYAML = "- emulation_plan_details:\n" +
		"    id: 11111111-1111-1111-1111-111111111111\n" +
		"    adversary_name: Fixture Eagle\n" +
		"- id: step-0001\n" +
		"  name: Initial foothold\n" +
		"  tactic: initial-access\n" +
		"  technique:\n" +
		"    attack_id: T1566.001\n" +
		"    name: Phishing\n"
	planPath  = "fixture_eagle/Emulation_Plan/yaml/Fixture_Eagle.yaml"
	otherPath = "fixture_eagle/Emulation_Plan/yaml/planners/planner.yml"
)

func TestFetchAsksOnlyForThePlanFiles(t *testing.T) {
	t.Parallel()

	stub := &recordingHTTP{files: map[string]string{planPath: planYAML}}
	stub.tree = treeJSON(false, planPath, otherPath,
		"apt29/Emulation_Plan/yaml/README.md",
		"micro_emulation_plans/src/webshell/plan.yaml",
		"apt29/Resources/binary.exe")

	bundle, err := fetch(t, stub)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// One listing, one plan file. The planner YAML, the README, the
	// micro-emulation plan and the binary are never requested — Parse would
	// have discarded them after they had been paid for.
	want := []string{
		"https://api.github.com/repos/center-for-threat-informed-defense/adversary_emulation_library/git/trees/master?recursive=1",
		"https://raw.githubusercontent.com/center-for-threat-informed-defense/adversary_emulation_library/master/" + planPath,
	}
	if got := stub.requested; !equalStrings(got, want) {
		t.Errorf("requested\n\t%q\nwant\n\t%q", got, want)
	}

	// What comes back is a zip laid out like the archive was, so everything
	// downstream — Parse, the raw snapshot, reprocess — is unchanged.
	if bundle.MediaType != "application/zip" {
		t.Errorf("MediaType = %q, want application/zip", bundle.MediaType)
	}
	names := zipNames(t, bundle.Bytes)
	wantName := "adversary_emulation_library-master/" + planPath
	if !equalStrings(names, []string{wantName}) {
		t.Errorf("bundle entries = %q, want %q", names, []string{wantName})
	}
	if bundle.Size != int64(len(bundle.Bytes)) {
		t.Errorf("Size = %d, want %d", bundle.Size, len(bundle.Bytes))
	}
}

// The bundle must parse, or the saving is worthless.
func TestFetchedBundleParsesAndNormalizes(t *testing.T) {
	t.Parallel()

	stub := &recordingHTTP{
		tree:  treeJSON(false, planPath),
		files: map[string]string{planPath: planYAML},
	}
	bundle, err := fetch(t, stub)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	a := ctid.New()
	ast, err := a.Parse(context.Background(), bundle)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	objects, err := a.Normalize(context.Background(), ast)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("Normalize gave %d objects, want 1", len(objects))
	}
}

// Identical plans must give identical bytes, so the raw snapshot's digest moves
// when a plan moves and not when a sync runs.
func TestFetchIsReproducible(t *testing.T) {
	t.Parallel()

	newStub := func() *recordingHTTP {
		return &recordingHTTP{
			tree:  treeJSON(false, planPath, "apt29/Emulation_Plan/yaml/APT29.yaml"),
			files: map[string]string{planPath: planYAML, "apt29/Emulation_Plan/yaml/APT29.yaml": planYAML},
		}
	}

	first, err := fetch(t, newStub())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	second, err := fetch(t, newStub())
	if err != nil {
		t.Fatalf("Fetch again: %v", err)
	}
	if !bytes.Equal(first.Bytes, second.Bytes) {
		t.Error("two fetches of the same plans produced different bytes")
	}
}

// A truncated listing is refused: the entries it dropped could be plans, and a
// catalog quietly missing an adversary is worse than a failed sync.
func TestTruncatedListingIsRefused(t *testing.T) {
	t.Parallel()

	stub := &recordingHTTP{tree: treeJSON(true, planPath), files: map[string]string{planPath: planYAML}}
	_, err := fetch(t, stub)
	if err == nil {
		t.Fatal("Fetch succeeded on a truncated listing")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("error does not say what went wrong: %v", err)
	}
	for _, u := range stub.requested {
		if strings.Contains(u, "raw.githubusercontent.com") {
			t.Errorf("fetched %s despite refusing the listing", u)
		}
	}
}

func TestNoPlansIsAnError(t *testing.T) {
	t.Parallel()

	stub := &recordingHTTP{tree: treeJSON(false, "readme.md", otherPath)}
	_, err := fetch(t, stub)
	if err == nil {
		t.Fatal("Fetch succeeded with no plan files in the repository")
	}
	if !strings.Contains(err.Error(), "Emulation_Plan") {
		t.Errorf("error does not say what was looked for: %v", err)
	}
}

// The rate limit is a wait, not a misconfiguration, and the message has to say
// so — otherwise an operator rechecks a URL that was right all along.
func TestRateLimitSaysSo(t *testing.T) {
	t.Parallel()

	stub := &recordingHTTP{status: http.StatusForbidden}
	_, err := fetch(t, stub)
	if err == nil {
		t.Fatal("Fetch succeeded against a 403")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error does not mention the rate limit: %v", err)
	}
}

// A source pointed at a mirror still works the way it always did: one request
// for the URL as configured.
func TestNonGitHubURLIsFetchedWhole(t *testing.T) {
	t.Parallel()

	mirror := "https://mirror.example.com/ctid/plans.zip"
	stub := &recordingHTTP{files: map[string]string{"": string(fixtureZip(t))}}
	a := ctid.New()
	_, err := a.Fetch(context.Background(), content.FetchRequest{
		Source:   content.SourceInfo{URL: mirror},
		MaxBytes: 1 << 20,
		HTTP:     stub,
		Policy:   content.URLPolicy{LookupIP: publicIP},
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !equalStrings(stub.requested, []string{mirror}) {
		t.Errorf("requested %q, want just the configured URL", stub.requested)
	}
}

func fetch(t *testing.T, stub *recordingHTTP) (content.Bundle, error) {
	t.Helper()

	return ctid.New().Fetch(context.Background(), content.FetchRequest{
		Source:   content.SourceInfo{URL: planURL},
		MaxBytes: 1 << 20,
		HTTP:     stub,
		Policy:   content.URLPolicy{LookupIP: publicIP},
	})
}

// recordingHTTP answers the two shapes of request this fetch makes and records
// every URL, so a test can assert on what was *not* asked for.
type recordingHTTP struct {
	tree      string
	files     map[string]string
	status    int
	requested []string
}

func (h *recordingHTTP) Do(req *http.Request) (*http.Response, error) {
	h.requested = append(h.requested, req.URL.String())

	if h.status != 0 {
		return response(h.status, ""), nil
	}
	if strings.Contains(req.URL.Host, "api.github.com") {
		return response(http.StatusOK, h.tree), nil
	}
	if strings.Contains(req.URL.Host, "codeload") || strings.Contains(req.URL.Path, "/archive/") {
		return nil, fmt.Errorf("the repository archive was requested: %s", req.URL)
	}
	// A raw file, or the mirror zip in the whole-fetch test.
	for path, body := range h.files {
		if path == "" || strings.HasSuffix(req.URL.Path, escapeTestPath(path)) {
			return response(http.StatusOK, body), nil
		}
	}
	return response(http.StatusNotFound, ""), nil
}

func escapeTestPath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header),
	}
}

// treeJSON builds a git-trees response listing paths as blobs.
func treeJSON(truncated bool, paths ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"truncated":%t,"tree":[`, truncated)
	for i, p := range paths {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"path":%q,"type":"blob"}`, p)
	}
	b.WriteString("]}")
	return b.String()
}

func fixtureZip(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("adversary_emulation_library-master/" + planPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(planYAML)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipNames(t *testing.T, raw []byte) []string {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("reading the bundle as a zip: %v", err)
	}
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

// publicIP resolves every hostname to a public address, so the egress fence
// passes without a test touching DNS.
func publicIP(context.Context, string) ([]net.IP, error) {
	return []net.IP{net.ParseIP("8.8.8.8")}, nil
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
