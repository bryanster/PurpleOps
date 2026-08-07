package httpapi

import (
	"bufio"
	"context"
	"database/sql"
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
	"github.com/bryanster/blacklight/internal/store"
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
			if fr.event == ":ping" || (fr.event == "" && fr.data == "") {
				continue
			}
			// Parse the hub envelope — no event: field since M4-003.
			var envelope struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(fr.data), &envelope); err != nil {
				t.Fatalf("envelope parse: %v\n%s", err, fr.data)
			}
			switch envelope.Type {
			case events.TypeContentJobProgress:
				sawProgress = true
			case events.TypeContentJobTerminal:
				sawTerminal = true
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

func TestSubscribingToContentJobsIsFilteredForMembers(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "member@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})
	cookie := sessionCookie(t, server.login("member@example.com", testPassword))

	rec := server.get(eventsPathTest+"?topics="+events.TopicContentJobs, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("member SSE = %d, want 400\n%s", rec.Code, rec.Body)
	}
}

func TestServiceTokenCannotSubscribeToEvents(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)
	tok := server.createToken(t, admin, authz.TokenScopeContentSync)

	rec := server.withToken(http.MethodGet, eventsPathTest+"?topics="+events.TopicContentJobs, tok.Token)
	if rec.Code != http.StatusBadRequest {
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

// TestMemberCanSubscribeToEngagementTopic verifies that a member of an
// engagement can subscribe to engagement.{id} (M4-001).
func TestMemberCanSubscribeToEngagementTopic(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t, func(u *identity.NewUser) {
		u.PlatformRole = authz.PlatformRoleMember
		u.Email = "purple@example.com"
		u.DisplayName = "Purple Member"
	})
	user, err := identity.NewUsers(server.db).ByEmail(t.Context(), "purple@example.com")
	if err != nil {
		t.Fatalf("lookup seeded user: %v", err)
	}
	cookie := sessionCookie(t, server.login("purple@example.com", testPassword))

	engID := "019385a2-0000-7000-8cf0-ef0123456789"
	seedEngagementPlumbing(t, server.db, engID, user.ID, "blue")

	assertSSEConnects(t, server, events.EngagementTopic(engID), cookie, http.StatusOK)
}

// TestNonMemberCannotSubscribeToEngagementTopic verifies topic filtering
// removes an engagement topic when the caller is not a member (M4-001).
func TestNonMemberCannotSubscribeToEngagementTopic(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t, func(u *identity.NewUser) {
		u.PlatformRole = authz.PlatformRoleMember
		u.Email = "outsider@example.com"
	})
	outsiderCookie := sessionCookie(t, server.login("outsider@example.com", testPassword))

	engID := "019385a2-1111-7000-8cf0-ef0123456789"
	seedEngagementOnly(t, server.db, engID)

	assertSSEConnects(t, server, events.EngagementTopic(engID), outsiderCookie, http.StatusBadRequest)
}

// TestAdminCanSubscribeToAnyEngagementTopic verifies the admin bypass in
// topicAllowed (M4-001).
func TestAdminCanSubscribeToAnyEngagementTopic(t *testing.T) {
	t.Parallel()
	server := newAuthServer(t)
	server.seedUser(t)
	adminCookie := server.signIn(t)
	engID := "019385a2-3333-7000-8cf0-ef0123456789"
	seedEngagementOnly(t, server.db, engID)

	assertSSEConnects(t, server, events.EngagementTopic(engID), adminCookie, http.StatusOK)
}

// assertSSEConnects verifies the SSE handshake returns the expected status.
// For 200 (stream open) it cancels the context after reading headers so the
// goroutine does not leak. For non-200 it reads the full error body.
func assertSSEConnects(t *testing.T, server *authServer, topic string, cookie *http.Cookie, wantStatus int) {
	t.Helper()

	ts := httptest.NewServer(server.handler)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		ts.URL+eventsPathTest+"?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Client timeout on a 200 SSE stream is expected (stream stays open).
		if wantStatus == http.StatusOK && ctx.Err() != nil {
			return
		}
		t.Fatalf("SSE connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		t.Fatalf("SSE status = %d, want %d\n%s", resp.StatusCode, wantStatus, body)
	}
}

func seedEngagementPlumbing(t *testing.T, db *store.DB, engID, userID, role string) {
	t.Helper()
	seedEngagementOnly(t, db, engID)
	if err := db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement_member (engagement_id, user_id, role, added_at)
			 VALUES ($1, $2, $3, NOW())`,
			engID, userID, role)
		return err
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

func seedEngagementOnly(t *testing.T, db *store.DB, engID string) {
	t.Helper()
	if err := db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.engagement (id, name, client, description, status,
			 starts_on, ends_on, attack_version, mode, auto_reveal_on_start,
			 created_by, created_at, updated_at)
			 VALUES ($1, 'Test Eng', 'Client Co', '', 'active',
			 '2025-01-01', '2025-12-31', '16.1', 'standard', false,
			 $1, NOW(), NOW())`,
			engID)
		return err
	}); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
}
