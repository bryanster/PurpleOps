package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
)

const eventsPathTest = BasePath + "/events"

func TestSSEHeadersAndProgressEndToEnd(t *testing.T) {
	t.Parallel()

	fixture := content.NewFixtureAdapter(storecontent.KindAtomic)
	fixture.FetchBytes = content.FixtureBundle(storecontent.VersionCurrent, []content.FixtureNote{
		{ExternalID: "n1", Title: "One", Body: "body"},
	})
	// Slow enough that the SSE client can arm before terminal.
	fixture.DelayBatch = 50 * time.Millisecond

	server := newAuthServerDeps(t, func(d *Deps) {
		d.ContentAdapters = map[storecontent.Kind]content.Adapter{
			storecontent.KindAtomic: fixture,
		}
		d.Config.Events.Heartbeat = 50 * time.Millisecond
	})
	server.seedUser(t)
	adminCookie := server.signIn(t)

	enable := server.send(http.MethodPost,
		contentSourcePath(storecontent.SourceIDAtomic)+"/enable", "", adminCookie)
	if enable.Code != http.StatusOK {
		t.Fatalf("enable atomic = %d\n%s", enable.Code, enable.Body)
	}

	ts := httptest.NewServer(server.handler)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		ts.URL+eventsPathTest+"?topics="+events.TopicContentJobs, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(adminCookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE connect: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("SSE status = %d (and body read failed: %v)", resp.StatusCode, err)
		}
		t.Fatalf("SSE status = %d\n%s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}

	type frame struct {
		event string
		data  string
	}
	frames := make(chan frame, 64)
	go func() {
		defer close(frames)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var cur frame
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if cur.event != "" || cur.data != "" {
					frames <- cur
					cur = frame{}
				}
			case strings.HasPrefix(line, ":"):
				frames <- frame{event: ":ping"}
			case strings.HasPrefix(line, "event:"):
				cur.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				cur.data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
	}()

	// Start a sync while the stream is open.
	syncRec := server.post(
		contentSourcePath(storecontent.SourceIDAtomic)+"/sync",
		`{}`,
		adminCookie,
	)
	if syncRec.Code != http.StatusAccepted {
		t.Fatalf("sync = %d\n%s", syncRec.Code, syncRec.Body)
	}
	job := decodeJSON[gen.ContentSyncJob](t, syncRec)
	jobID := job.Id.String()

	var sawProgress, sawTerminal bool
	var terminalStatus string
	deadline := time.After(20 * time.Second)
	for !sawTerminal {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for terminal SSE event")
		case fr, ok := <-frames:
			if !ok {
				t.Fatal("SSE stream closed before terminal event")
			}
			if fr.event == ":ping" || fr.event == "" {
				continue
			}
			switch fr.event {
			case events.TypeContentJobProgress:
				sawProgress = true
			case events.TypeContentJobTerminal:
				sawTerminal = true
				var envelope struct {
					Data json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal([]byte(fr.data), &envelope); err != nil {
					t.Fatalf("terminal data: %v\n%s", err, fr.data)
				}
				var body contentJobEventData
				if err := json.Unmarshal(envelope.Data, &body); err != nil {
					t.Fatalf("terminal body: %v\n%s", err, envelope.Data)
				}
				if body.JobID != jobID {
					t.Fatalf("terminal jobId = %q, want %q", body.JobID, jobID)
				}
				terminalStatus = body.Status
			}
		}
	}
	if !sawProgress {
		t.Fatal("expected at least one content.job.progress event before terminal")
	}

	got := server.get(BasePath+"/content/jobs/"+jobID, adminCookie)
	if got.Code != http.StatusOK {
		t.Fatalf("GET job = %d\n%s", got.Code, got.Body)
	}
	wired := decodeJSON[gen.ContentSyncJob](t, got)
	if string(wired.Status) != terminalStatus {
		t.Fatalf("SSE terminal status %q != GET job status %q", terminalStatus, wired.Status)
	}
	if wired.Status != gen.ContentSyncJobStatusSucceeded {
		t.Fatalf("job status = %s, want succeeded (err=%v)", wired.Status, wired.Error)
	}

	cancel()
}

func TestMemberSubscribingToContentJobsIsForbidden(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "member@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})
	cookie := sessionCookie(t, server.login("member@example.com", testPassword))

	rec := server.get(eventsPathTest+"?topics="+events.TopicContentJobs, cookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member SSE = %d, want 403\n%s", rec.Code, rec.Body)
	}
}

func TestServiceTokenCannotSubscribeToEvents(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)
	tok := server.createToken(t, admin, authz.TokenScopeContentSync)

	rec := server.withToken(http.MethodGet, eventsPathTest+"?topics="+events.TopicContentJobs, tok.Token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("token SSE = %d, want 403\n%s", rec.Code, rec.Body)
	}
}

func TestUnknownTopicIsBadRequest(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)
	rec := server.get(eventsPathTest+"?topics=engagement.nope", admin)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown topic = %d, want 400\n%s", rec.Code, rec.Body)
	}
}

func TestUnauthenticatedSSEIs401(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	rec := server.get(eventsPathTest + "?topics=" + events.TopicContentJobs)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anon SSE = %d, want 401\n%s", rec.Code, rec.Body)
	}
}
