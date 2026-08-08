package content

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ErrTooLarge is returned when a download exceeds the configured max bytes.
// The error string names the limit so job failure messages and problem details
// can surface it without further wrapping.
type ErrTooLarge struct {
	Limit int64
	Got   int64 // 0 when unknown (stream stopped at limit+1)
}

func (e *ErrTooLarge) Error() string {
	if e.Got > 0 {
		return fmt.Sprintf("download exceeds content max bytes limit of %d (got at least %d)", e.Limit, e.Got)
	}
	return fmt.Sprintf("download exceeds content max bytes limit of %d", e.Limit)
}

// HTTPSource fetches bytes over HTTPS (or http in tests).
type HTTPSource struct {
	URL      string
	MaxBytes int64
	Client   HTTPDoer
	// UserAgent is sent on the request. Empty defaults to "blacklight-content".
	UserAgent string
}

// Open GETs the URL and returns a limited reader. Size is Content-Length or -1.
func (s HTTPSource) Open(ctx context.Context) (io.ReadCloser, int64, error) {
	if s.URL == "" {
		return nil, 0, errors.New("content: HTTPSource: empty URL")
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("content: build fetch request: %w", err)
	}
	ua := s.UserAgent
	if ua == "" {
		ua = "blacklight-content"
	}
	req.Header.Set("User-Agent", ua)

	res, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("content: fetch %s: %w", s.URL, err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_ = res.Body.Close()
		return nil, 0, fmt.Errorf("content: fetch %s: unexpected status %s", s.URL, res.Status)
	}

	size := res.ContentLength
	if s.MaxBytes > 0 && size > s.MaxBytes {
		_ = res.Body.Close()
		return nil, 0, &ErrTooLarge{Limit: s.MaxBytes, Got: size}
	}

	body := res.Body
	if s.MaxBytes > 0 {
		body = &limitedBody{rc: res.Body, limit: s.MaxBytes}
	}
	return body, size, nil
}

// limitedBody closes with ErrTooLarge when more than limit bytes are read.
type limitedBody struct {
	rc    io.ReadCloser
	limit int64
	read  int64
	err   error
}

func (l *limitedBody) Read(p []byte) (int, error) {
	if l.err != nil {
		return 0, l.err
	}
	if l.read >= l.limit {
		l.err = &ErrTooLarge{Limit: l.limit, Got: l.read + 1}
		return 0, l.err
	}
	// Read at most what remains under the limit, plus one byte to detect overflow.
	remain := l.limit - l.read
	if int64(len(p)) > remain+1 {
		p = p[:remain+1]
	}
	n, err := l.rc.Read(p)
	l.read += int64(n)
	if l.read > l.limit {
		l.err = &ErrTooLarge{Limit: l.limit, Got: l.read}
		return n, l.err
	}
	return n, err
}

func (l *limitedBody) Close() error { return l.rc.Close() }

// FileSource reads a local path (raw snapshot reprocess or uploaded bundle).
type FileSource struct {
	Path string
}

// Open opens the file. Size is the file length.
func (s FileSource) Open(ctx context.Context) (io.ReadCloser, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if s.Path == "" {
		return nil, 0, errors.New("content: FileSource: empty path")
	}
	// Reject path tricks early; the runner also validates against content root.
	if strings.Contains(s.Path, "..") {
		return nil, 0, fmt.Errorf("content: FileSource: path %q must not contain %s", s.Path, `".."`)
	}
	f, err := os.Open(s.Path)
	if err != nil {
		return nil, 0, fmt.Errorf("content: open %s: %w", s.Path, err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("content: stat %s: %w", s.Path, err)
	}
	return f, st.Size(), nil
}

// ReadAll drains src honouring MaxBytes when src is an HTTPSource-style limit.
// sizeHint is the value Open returned (-1 if unknown).
func ReadAll(ctx context.Context, src ByteSource) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rc, _, err := src.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// Bound the whole read with context: copy in chunks and check between.
	var buf strings.Builder // not ideal for binary — use bytes.Buffer
	_ = buf
	out := make([]byte, 0, 4096)
	tmp := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, readErr := rc.Read(tmp)
		if n > 0 {
			out = append(out, tmp[:n]...)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return out, nil
			}
			return nil, readErr
		}
	}
}
