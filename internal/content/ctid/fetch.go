package ctid

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/content"
)

// Fetching the plans without fetching the repository.
//
// The configured source URL is a GitHub archive of the whole
// adversary_emulation_library, which is around 640 MiB and grows with every
// tool, binary and packet capture the project adds. The thirteen files this
// adapter reads — the machine-readable plans under {actor}/Emulation_Plan/yaml
// — come to about 400 KiB of that. Downloading the archive to reach them cost
// roughly fifteen hundred times the bytes it used, held the whole thing in
// memory (the pipeline is bytes-in-memory from Fetch to the raw snapshot), and
// eventually failed outright when the archive crossed the content size limit.
//
// So the fetch asks for the plans instead: one call for the repository tree,
// then one per plan file, reassembled here into a small zip with the same entry
// layout the archive had. Everything downstream — Parse, the raw snapshot,
// reprocess-from-raw, offline bundle upload — keeps working on a zip and does
// not know the difference.
//
// Two things this deliberately does not do. It does not fall back to the whole
// archive when the tree call fails: that would trade a clear error for a
// several-hundred-megabyte download that fails later and less clearly. And it
// does not authenticate, so it shares the unauthenticated GitHub rate limit —
// one tree call per sync is comfortably inside it, and the plan files
// themselves come from raw.githubusercontent.com, which is not part of that
// budget.

const (
	// treeMaxBytes bounds the repository listing. The real one is about 1.9 MiB
	// over 5220 entries; 16 MiB leaves room for the repository to grow by an
	// order of magnitude and still fails a listing that is absurd.
	treeMaxBytes int64 = 16 << 20

	// planFileMaxBytes bounds one plan YAML. The largest upstream plan is
	// APT29 at about 100 KiB.
	planFileMaxBytes int64 = 8 << 20
)

// zipModTime is the timestamp written into every synthesized entry.
//
// Fixed on purpose: with it, identical plans produce identical zip bytes, so
// the raw snapshot's digest changes when a plan changes and not when a sync
// happens to run. The upstream archive could not offer that — its bytes moved
// with every commit to any file in the repository.
var zipModTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// githubRepo is an upstream the plans can be read from file by file.
type githubRepo struct {
	Owner string
	Name  string
	Ref   string
}

// parseArchiveURL recognizes the GitHub archive URLs the source row is
// configured with, and reports whether raw is one of them.
//
// A URL it does not recognize is not an error: an operator may point this
// source at a mirror, or at a zip served from somewhere else entirely, and
// those still work the way they always did — see [Adapter.Fetch].
func parseArchiveURL(raw string) (githubRepo, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return githubRepo{}, false
	}

	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 {
		return githubRepo{}, false
	}
	owner, name := parts[0], parts[1]

	var rest []string
	switch {
	// https://github.com/{owner}/{repo}/archive/refs/heads/{ref}.zip
	case (host == "github.com" || host == "www.github.com") && parts[2] == "archive":
		rest = parts[3:]
	// https://codeload.github.com/{owner}/{repo}/zip/refs/heads/{ref}
	case host == "codeload.github.com" && (parts[2] == "zip" || parts[2] == "tar.gz"):
		rest = parts[3:]
	default:
		return githubRepo{}, false
	}

	// refs/heads/{ref} and refs/tags/{ref} both name a ref; so does a bare
	// {ref}, which is what the shorter archive URL carries.
	if len(rest) >= 2 && rest[0] == "refs" && (rest[1] == "heads" || rest[1] == "tags") {
		rest = rest[2:]
	}
	ref := strings.Join(rest, "/")
	ref = strings.TrimSuffix(strings.TrimSuffix(ref, ".zip"), ".tar.gz")

	if owner == "" || name == "" || ref == "" {
		return githubRepo{}, false
	}
	return githubRepo{Owner: owner, Name: name, Ref: ref}, true
}

// treeResponse is the part of the git-trees response this adapter reads.
type treeResponse struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
	// Truncated is set when the repository has more entries than one response
	// carries. A truncated listing is refused rather than used: the missing
	// entries could be plans, and a catalog quietly missing an adversary is
	// worse than a sync that says it could not read the repository.
	Truncated bool `json:"truncated"`
}

// fetchPlans reads the plan files of repo and returns them as zip bytes with
// the layout the upstream archive had: {repo}-{ref}/{path}.
func fetchPlans(ctx context.Context, req content.FetchRequest, repo githubRepo) ([]byte, error) {
	paths, err := listPlanPaths(ctx, req, repo)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf(
			"ctid: fetch %s/%s@%s: no plan files under {actor}/Emulation_Plan/yaml/; "+
				"the upstream layout has changed, or the ref is wrong",
			repo.Owner, repo.Name, repo.Ref)
	}

	// Sorted so the zip is byte-for-byte reproducible; the tree API's order is
	// not something to depend on.
	sort.Strings(paths)

	prefix := fmt.Sprintf("%s-%s", repo.Name, strings.ReplaceAll(repo.Ref, "/", "-"))

	// The configured ceiling still bounds the fetch, now across the plans
	// together rather than against one archive: no single file may exceed it,
	// and neither may the total.
	var total int64

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range paths {
		fileMax := planFileMaxBytes
		if req.MaxBytes > 0 && req.MaxBytes < fileMax {
			fileMax = req.MaxBytes
		}
		raw, err := content.ReadAll(ctx, content.HTTPSource{
			URL:      rawFileURL(repo, p),
			MaxBytes: fileMax,
			Client:   req.HTTP,
			Policy:   req.Policy,
		})
		if err != nil {
			return nil, fmt.Errorf("ctid: fetch %s: %w", p, err)
		}
		total += int64(len(raw))
		if req.MaxBytes > 0 && total > req.MaxBytes {
			return nil, &content.ErrTooLarge{Limit: req.MaxBytes, Got: total}
		}

		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     path.Join(prefix, p),
			Method:   zip.Deflate,
			Modified: zipModTime,
		})
		if err != nil {
			return nil, fmt.Errorf("ctid: fetch: build bundle entry %s: %w", p, err)
		}
		if _, err := w.Write(raw); err != nil {
			return nil, fmt.Errorf("ctid: fetch: write bundle entry %s: %w", p, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("ctid: fetch: finish bundle: %w", err)
	}
	return buf.Bytes(), nil
}

// listPlanPaths returns the repository paths of the plan YAML files, chosen by
// the same predicate [Parse] applies to archive entries — so the bundle carries
// exactly what would have been read out of the full archive, and planner YAML,
// documentation and micro-emulation plans are never fetched at all.
func listPlanPaths(ctx context.Context, req content.FetchRequest, repo githubRepo) ([]string, error) {
	listing := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		url.PathEscape(repo.Owner), url.PathEscape(repo.Name), escapePath(repo.Ref))

	max := treeMaxBytes
	if req.MaxBytes > 0 && req.MaxBytes < max {
		max = req.MaxBytes
	}
	raw, err := content.ReadAll(ctx, content.HTTPSource{
		URL:      listing,
		MaxBytes: max,
		Client:   req.HTTP,
		Policy:   req.Policy,
	})
	if err != nil {
		// 403 from this endpoint is nearly always the unauthenticated rate
		// limit, which is a wait rather than a misconfiguration. Saying so is
		// the difference between an operator retrying in an hour and an
		// operator rechecking a URL that was right all along.
		if strings.Contains(err.Error(), "unexpected status 403") {
			return nil, fmt.Errorf("ctid: list %s/%s@%s: %w "+
				"(the GitHub API rate limit for unauthenticated callers is per address and resets hourly)",
				repo.Owner, repo.Name, repo.Ref, err)
		}
		return nil, fmt.Errorf("ctid: list %s/%s@%s: %w", repo.Owner, repo.Name, repo.Ref, err)
	}

	var tree treeResponse
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("ctid: list %s/%s@%s: decode repository listing: %w",
			repo.Owner, repo.Name, repo.Ref, err)
	}
	if tree.Truncated {
		return nil, fmt.Errorf(
			"ctid: list %s/%s@%s: the repository listing came back truncated, so some plans "+
				"would be missing; import an offline bundle instead (docs/content-ctid.md)",
			repo.Owner, repo.Name, repo.Ref)
	}

	var paths []string
	for _, entry := range tree.Tree {
		if entry.Type == "blob" && isPlanYAMLPath(entry.Path) {
			paths = append(paths, entry.Path)
		}
	}
	return paths, nil
}

// rawFileURL is where one file of a ref is served as bytes.
func rawFileURL(repo githubRepo, filePath string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		url.PathEscape(repo.Owner), url.PathEscape(repo.Name),
		escapePath(repo.Ref), escapePath(filePath))
}

// escapePath escapes each segment of a slash-separated path and leaves the
// separators alone — a branch called feature/x and a file three directories
// down are both paths, not single components.
func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}
