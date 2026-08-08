// Package loadtest proves that concurrent SSE subscribers, catch-up replay,
// presence heartbeats, and slow-client eviction stay correct and responsive
// under war-room load (M4-010). Tests use real HTTP SSE against a real test
// server — never a mock.
//
// Two tests:
//
//   - TestSSEWarRoomConcurrency — CI gate (always on)
//
//   - TestSSEWarRoomLoad — full developer load (BLACKLIGHT_LOADTEST=1)
//
//     BLACKLIGHT_LOADTEST=1 go test -count=1 -timeout 15m ./internal/events/loadtest/ -run TestSSEWarRoomLoad
package loadtest_test

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/events/presence"
	"github.com/bryanster/blacklight/internal/httpapi"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Budgets
// ---------------------------------------------------------------------------

const (
	ssePublishP95       = 200 * time.Millisecond
	ssePublishMax       = 2 * time.Second
	sseSubscriberBuffer = 16
)

// ---------------------------------------------------------------------------
// CI gate test — always runs
// ---------------------------------------------------------------------------

func TestSSEWarRoomConcurrency(t *testing.T) {
	opts := sseLoadOpts{
		users: 5, steps: 10, activityHistory: 30,
		subscribers: 10, stalledSubs: 2,
		duration: 10 * time.Second, writeInterval: 500 * time.Millisecond,
		presenceUsers: 3,
	}
	runSSELoadTest(t, opts)
}

// ---------------------------------------------------------------------------
// Full developer load — gated
// ---------------------------------------------------------------------------

func TestSSEWarRoomLoad(t *testing.T) {
	if !loadtestEnabled() {
		t.Skip("BLACKLIGHT_LOADTEST is not set; skipping full SSE war-room load test")
	}
	opts := sseLoadOpts{
		users: 20, steps: 25, activityHistory: 250,
		subscribers: 40, stalledSubs: 4,
		duration: 20 * time.Second, writeInterval: 200 * time.Millisecond,
		presenceUsers: 10,
	}
	runSSELoadTest(t, opts)
}

// ---------------------------------------------------------------------------
// Options and results
// ---------------------------------------------------------------------------

type sseLoadOpts struct {
	users, steps, activityHistory int
	subscribers, stalledSubs      int
	duration, writeInterval       time.Duration
	presenceUsers                 int
}

type sseLoadResult struct {
	publishLatencies []time.Duration
	eventsReceived   atomic.Int64
	eventsDropped    atomic.Int64
	goroutinesBefore int
	goroutinesAfter  int
}

// ---------------------------------------------------------------------------
// Main test runner
// ---------------------------------------------------------------------------

func runSSELoadTest(t *testing.T, opts sseLoadOpts) {
	t.Helper()

	db := storetest.Migrated(t)
	cfg := testConfig(t)

	data := seedLoadTestData(t, db, opts)

	pr := presence.New(presence.Options{HeartbeatTTL: 5 * time.Minute})

	handler, err := httpapi.NewServer(httpapi.Deps{
		Config: cfg, Store: db,
		Logger:               slog.New(slog.DiscardHandler),
		UI:                   nil,
		Presence:             pr,
		DisableContentRunner: true,
	})
	if err != nil {
		t.Fatalf("building the server: %v", err)
	}

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	sessions := loginUsers(t, ts.URL, data)

	ctx, cancel := context.WithTimeout(t.Context(), opts.duration+30*time.Second)
	defer cancel()

	result := sseLoadResult{goroutinesBefore: runtime.NumGoroutine()}

	var publishLatMu sync.Mutex
	var wg sync.WaitGroup

	type subState struct {
		frames <-chan sseFrame
		cancel context.CancelFunc
	}
	subs := make([]subState, opts.subscribers)

	engTopic := events.EngagementTopic(data.engagementID)
	eventsURL := fmt.Sprintf("%s%s/events?topics=%s", ts.URL, httpapi.BasePath, engTopic)

	for i := range opts.subscribers {
		sessIdx := i % len(sessions)
		subCtx, subCancel := context.WithCancel(ctx)

		var cursor string
		if i%2 == 0 && len(data.activityIDs) > 10 {
			cursor = data.activityIDs[len(data.activityIDs)/2]
		}

		url := eventsURL
		if cursor != "" {
			url += "&lastEventId=" + cursor
		}

		frames := captureSSE(t, url, sessions[sessIdx], subCtx)
		subs[i] = subState{frames: frames, cancel: subCancel}

		if i >= opts.subscribers-opts.stalledSubs {
			continue // stalled: don't drain
		}

		wg.Add(1)
		go func(ch <-chan sseFrame) {
			defer wg.Done()
			for range ch {
				result.eventsReceived.Add(1)
			}
			result.eventsDropped.Add(1)
		}(frames)
	}

	time.Sleep(500 * time.Millisecond)

	// Presence heartbeats.
	presenceStop := make(chan struct{})
	go heartbeater(ctx, ts.URL, data.engagementID, sessions, opts.presenceUsers, presenceStop)

	// Write workers.
	writeStop := make(chan struct{})
	var writeWG sync.WaitGroup
	writeWorkers := max(1, opts.users/3)

	for w := range writeWorkers {
		writeWG.Add(1)
		go activityWriter(ctx, &writeWG, &result, &publishLatMu,
			data, opts.writeInterval, w, writeStop)
	}

	time.Sleep(opts.duration)

	close(writeStop)
	writeWG.Wait()
	close(presenceStop)

	time.Sleep(1 * time.Second) // let evictions fire

	for i := range subs {
		subs[i].cancel()
	}
	wg.Wait()

	result.goroutinesAfter = runtime.NumGoroutine()

	t.Logf("seed: %d users, %d steps, %d activity rows, %d subscribers (%d stalled)",
		opts.users, opts.steps, opts.activityHistory, opts.subscribers, opts.stalledSubs)

	assertSSEWarRoom(t, &result)
}

func heartbeater(ctx context.Context, baseURL, engID string,
	sessions []*http.Cookie, n int, stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for i := range n {
				if i >= len(sessions) {
					break
				}
				pid := uuid.Must(uuid.NewV7()).String()
				body := fmt.Sprintf(`{"presenceId":%q}`, pid)
				req, err := http.NewRequestWithContext(ctx, http.MethodPut,
					fmt.Sprintf("%s%s/engagements/%s/presence",
						baseURL, httpapi.BasePath, engID),
					strings.NewReader(body))
				if err != nil {
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.AddCookie(sessions[i])
				resp, err := http.DefaultClient.Do(req)
				if err == nil {
					resp.Body.Close()
				}
			}
		case <-stop:
			return
		}
	}
}

func activityWriter(ctx context.Context, wg *sync.WaitGroup,
	result *sseLoadResult, latMu *sync.Mutex,
	data loadTestData, interval time.Duration, workerIdx int,
	stop <-chan struct{}) {

	defer wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			execIdx := (workerIdx + int(time.Now().UnixNano())) % len(data.executionIDs)
			if execIdx < 0 {
				execIdx = 0
			}
			execID := data.executionIDs[execIdx]
			before := time.Now()

			err := data.activityLog.RecordAlone(ctx, events.Entry{
				EngagementID: data.engagementID,
				ActorID:      data.userIDs[workerIdx%len(data.userIDs)],
				Verb:         events.VerbExecutionRedUpdated,
				ObjectType:   events.ObjectExecution,
				ObjectID:     execID,
				ParentIDs:    map[string]string{"stepId": data.stepIDs[execIdx%len(data.stepIDs)]},
				Delta:        events.Delta(map[string]any{"notes": fmt.Sprintf("worker-%d-%d", workerIdx, time.Now().Unix())}),
			})
			_ = err

			elapsed := time.Since(before)
			latMu.Lock()
			result.publishLatencies = append(result.publishLatencies, elapsed)
			latMu.Unlock()
		case <-stop:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Seed data
// ---------------------------------------------------------------------------

type loadTestData struct {
	engagementID string
	scenarioID   string
	userIDs      []string
	stepIDs      []string
	executionIDs []string
	activityIDs  []string
	activityLog  *events.Log
}

// seedLoadTestData creates users, an engagement with membership, scenarios,
// steps, executions, and activity history.
func seedLoadTestData(t *testing.T, db *store.DB, opts sseLoadOpts) loadTestData {
	t.Helper()
	ctx := t.Context()

	var data loadTestData
	data.activityLog = events.New(activity.New(db))

	// Create users.
	users := identity.NewUsers(db)
	for i := range opts.users {
		u, err := users.Create(ctx, identity.NewUser{
			Email:        fmt.Sprintf("loadtest-user-%d@blacklight.test", i),
			DisplayName:  fmt.Sprintf("LoadTest User %d", i),
			PasswordHash: testPasswordHash(),
			PlatformRole: authz.PlatformRoleMember,
			Status:       identity.StatusActive,
		})
		if err != nil {
			t.Fatalf("creating user %d: %v", i, err)
		}
		data.userIDs = append(data.userIDs, u.ID)
	}

	// Create engagement, scenario, steps, executions.
	engID := uuid.Must(uuid.NewV7()).String()
	scenarioID := uuid.Must(uuid.NewV7()).String()
	now := time.Now().UTC().Truncate(time.Microsecond)

	err := db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app.engagement
			 (id, name, client, description, status, mode,
			  starts_on, ends_on, attack_version, auto_reveal_on_start,
			  created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'active', 'standard',
			  ?, ?, ?, false,
			  ?, ?, ?)`,
			engID, "SSE Load Test", "Load Test Client", "M4-010 load test engagement",
			now, now, "15.1",
			data.userIDs[0], now, now); err != nil {
			return fmt.Errorf("insert engagement: %w", err)
		}
		for _, uid := range data.userIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.engagement_member
				 (engagement_id, user_id, role, added_at)
				 VALUES (?, ?, ?, ?)`,
				engID, uid, "red", now); err != nil {
				return fmt.Errorf("insert member %s: %w", uid, err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.scenario
			 (id, engagement_id, ordinal, name, narrative, source,
			  threat_actor, source_ref, plan_id, created_at, updated_at)
			 VALUES (?, ?, 0, ?, '', 'manual', '', '', '', ?, ?)`,
			scenarioID, engID, "Load Test Scenario", now, now); err != nil {
			return fmt.Errorf("insert scenario: %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seeding engagement: %v", err)
	}
	data.engagementID = engID
	data.scenarioID = scenarioID

	// Create steps and executions.
	err = db.Write(ctx, func(tx *sql.Tx) error {
		for i := range opts.steps {
			stepID := uuid.Must(uuid.NewV7()).String()
			execID := uuid.Must(uuid.NewV7()).String()

			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.step
				 (id, scenario_id, ordinal, name, objective,
				  "procedure", template_id, target_asset, tools,
				  controls_in_scope, attack_version, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?,
				  '{}', '', '', '[]',
				  '[]', '15.1', ?, ?)`,
				stepID, scenarioID, i,
				fmt.Sprintf("Step %d", i),
				fmt.Sprintf("Objective %d", i),
				now, now); err != nil {
				return fmt.Errorf("insert step %d: %w", i, err)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.execution
				 (id, step_id, version, status,
				  executed_by, command_run, source_host, target_host, red_notes,
				  detection_modifiers, detecting_source, detecting_rule_ref,
				  alert_severity, blue_notes, scored_by,
				  created_at, updated_at)
				 VALUES (?, ?, 1, 'pending',
				  ?, '', '', '', '',
				  '[]', '', '', '', '', '',
				  ?, ?)`,
				execID, stepID, data.userIDs[0],
				now, now); err != nil {
				return fmt.Errorf("insert execution %d: %w", i, err)
			}
			data.stepIDs = append(data.stepIDs, stepID)
			data.executionIDs = append(data.executionIDs, execID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seeding steps: %v", err)
	}

	// Create activity history.
	actorID := data.userIDs[0]
	for i := range opts.activityHistory {
		execIdx := i % len(data.executionIDs)
		entry := events.Entry{
			EngagementID: engID,
			ActorID:      actorID,
			Verb:         events.VerbExecutionRedUpdated,
			ObjectType:   events.ObjectExecution,
			ObjectID:     data.executionIDs[execIdx],
			ParentIDs:    map[string]string{"stepId": data.stepIDs[execIdx]},
			Delta:        events.Delta(map[string]any{"notes": fmt.Sprintf("seed-activity-%d", i)}),
		}
		if err := data.activityLog.RecordAlone(ctx, entry); err != nil {
			t.Fatalf("recording seed activity %d: %v", i, err)
		}
	}

	activityIDs, err := listActivityIDs(t, db, engID)
	if err != nil {
		t.Fatalf("listing activity ids: %v", err)
	}
	data.activityIDs = activityIDs

	return data
}

// ---------------------------------------------------------------------------
// Login helper
// ---------------------------------------------------------------------------

func loginUsers(t *testing.T, baseURL string, data loadTestData) []*http.Cookie {
	t.Helper()

	cookies := make([]*http.Cookie, len(data.userIDs))
	for i := range data.userIDs {
		email := fmt.Sprintf("loadtest-user-%d@blacklight.test", i)
		body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, testPasswordPlaintext)
		req, err := http.NewRequestWithContext(t.Context(),
			http.MethodPost,
			baseURL+httpapi.BasePath+"/auth/login",
			strings.NewReader(body))
		if err != nil {
			t.Fatalf("login request for user %d: %v", i, err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("login for user %d: %v", i, err)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("login body for user %d: %v", i, err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("login for user %d = %d, want 200\nbody: %s",
				i, resp.StatusCode, bodyBytes)
		}

		found := false
		for _, c := range resp.Cookies() {
			if c.Name == "bl_session" {
				cookies[i] = c
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no bl_session cookie for user %d", i)
		}
	}
	return cookies
}

// ---------------------------------------------------------------------------
// SSE client
// ---------------------------------------------------------------------------

type sseFrame struct {
	event string
	data  string
	id    string
}

func captureSSE(t *testing.T, url string, cookie *http.Cookie,
	ctx context.Context) <-chan sseFrame {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("SSE new request: %v", err)
	}
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE connect: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			t.Fatalf("SSE status = %d (and body read failed: %v)", resp.StatusCode, err)
		}
		t.Fatalf("SSE status = %d\n%s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	frames := make(chan sseFrame, 256)

	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 4096), 256*1024)

		var fr sseFrame
		for scanner.Scan() {
			line := scanner.Text()

			select {
			case <-ctx.Done():
				return
			default:
			}

			if line == "" {
				if fr.data != "" || fr.event != "" {
					select {
					case frames <- fr:
					case <-ctx.Done():
						return
					}
				}
				fr = sseFrame{}
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue
			}
			switch {
			case strings.HasPrefix(line, "event:"):
				fr.event = strings.TrimSpace(line[6:])
			case strings.HasPrefix(line, "data:"):
				d := strings.TrimSpace(line[5:])
				if fr.data != "" {
					fr.data += "\n"
				}
				fr.data += d
			case strings.HasPrefix(line, "id:"):
				fr.id = strings.TrimSpace(line[3:])
			}
		}
	}()

	return frames
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

func assertSSEWarRoom(t *testing.T, result *sseLoadResult) {
	t.Helper()

	if len(result.publishLatencies) > 0 {
		sortLatencies(result.publishLatencies)
		p95 := percentile(result.publishLatencies, 95)
		pmax := maxDuration(result.publishLatencies)

		t.Logf("publish latencies: p50=%s p95=%s max=%s samples=%d",
			percentile(result.publishLatencies, 50), p95, pmax,
			len(result.publishLatencies))

		if p95 > ssePublishP95 {
			t.Errorf("publish p95 = %s, budget = %s", p95, ssePublishP95)
		}
		if pmax > ssePublishMax {
			t.Errorf("publish max = %s, budget = %s", pmax, ssePublishMax)
		}
	}

	received := result.eventsReceived.Load()
	if received == 0 {
		t.Error("zero events received by any subscriber; SSE fan-out may be broken")
	}
	t.Logf("events received: %d", received)

	dropped := result.eventsDropped.Load()
	t.Logf("subscriber channels closed: %d (includes cancelled + evicted)", dropped)

	delta := result.goroutinesAfter - result.goroutinesBefore
	t.Logf("goroutines: before=%d after=%d delta=%d",
		result.goroutinesBefore, result.goroutinesAfter, delta)
	if delta > 100 {
		t.Errorf("goroutine delta = %d, want <=100 (possible leak)", delta)
	}
}

// ---------------------------------------------------------------------------
// Password helper
// ---------------------------------------------------------------------------

const testPasswordPlaintext = "Str0ng!Pass"

var testPasswordHash = sync.OnceValue(func() string {
	hash, err := password.Hash(password.Plaintext(testPasswordPlaintext))
	if err != nil {
		panic("computing test password hash: " + err.Error())
	}
	return hash
})

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func listActivityIDs(t *testing.T, db *store.DB, engID string) ([]string, error) {
	t.Helper()
	rows, err := db.Read().QueryContext(t.Context(),
		`SELECT id FROM app.activity WHERE engagement_id = ? ORDER BY id ASC`, engID)
	if err != nil {
		return nil, fmt.Errorf("query activity ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan activity id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func testConfig(t *testing.T) config.Config {
	t.Helper()

	var baseURL config.URL
	if err := baseURL.UnmarshalText([]byte("http://localhost:8080")); err != nil {
		t.Fatalf("parsing test base URL: %v", err)
	}

	return config.Config{
		Env: config.EnvProduction,
		Server: config.Server{
			Addr:            "127.0.0.1:0",
			BaseURL:         baseURL,
			RequestTimeout:  30 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		Session: config.Session{
			Secret:      config.NewSecret([]byte("test-session-secret-not-a-real-one")),
			Lifetime:    12 * time.Hour,
			IdleTimeout: 2 * time.Hour,
		},
		Encryption: config.Encryption{
			Key: config.NewSecret([]byte("test-encryption-key-also-not-real")),
		},
		MFA: config.MFA{PendingTTL: 5 * time.Minute},
		Throttle: config.Throttle{
			AccountFailures: 99, AccountLockout: 1 * time.Minute,
			SourceFailures: 999, SourceLockout: 1 * time.Minute,
		},
		Content: config.Content{
			Dir:        t.TempDir(),
			MaxBytes:   512 << 20,
			JobTimeout: 30 * time.Minute,
			WriteBatch: 250,
		},
		Events: config.Events{
			MaxSubscribers:  256,
			Buffer:          sseSubscriberBuffer,
			Heartbeat:       15 * time.Second,
			MaxReplayEvents: 500,
		},
	}
}

func loadtestEnabled() bool { return config.LoadTestEnabled() }

// ---------------------------------------------------------------------------
// Math helpers
// ---------------------------------------------------------------------------

func sortLatencies(samples []time.Duration) {
	for i := range samples {
		for j := i + 1; j < len(samples); j++ {
			if samples[i] > samples[j] {
				samples[i], samples[j] = samples[j], samples[i]
			}
		}
	}
}

func percentile(samples []time.Duration, p int) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	idx := (p*len(samples) - 1) / 100
	if idx < 0 {
		idx = 0
	}
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	return samples[idx]
}

func maxDuration(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	m := samples[0]
	for _, s := range samples[1:] {
		if s > m {
			m = s
		}
	}
	return m
}
