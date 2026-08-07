package events_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestDeltaDropsSecretKeysAndValues(t *testing.T) {
	t.Parallel()
	got := events.Delta(map[string]any{
		"email":         "a@example.com",
		"password":      "hunter2",
		"password_hash": "$argon2id$v=19$m=65536,t=3,p=4$abc",
		"token":         "bl_PREFIX_secretvaluehere",
		"ip":            "1.2.3.4",
		"note":          "bl_looks_like_a_token_value_xxxxxx",
		"safe":          "ok",
	})
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, banned := range []string{
		"hunter2",
		"$argon2",
		"bl_PREFIX",
		"password",
		"token",
	} {
		if strings.Contains(s, banned) {
			t.Errorf("delta still contains %q: %s", banned, s)
		}
	}
	if got["email"] != "a@example.com" || got["ip"] != "1.2.3.4" || got["safe"] != "ok" {
		t.Errorf("lost safe fields: %#v", got)
	}
}

func TestRecordSharesTransactionWithCaller(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	log := events.New(activity.New(db))
	ctx := context.Background()
	sentinel := errors.New("nope")

	err := db.Write(ctx, func(tx *sql.Tx) error {
		if err := log.Record(ctx, tx, events.Entry{
			ActorID:    "actor",
			Verb:       events.VerbSessionLogin,
			ObjectType: events.ObjectSession,
			ObjectID:   "sess",
			Delta:      events.Delta(map[string]any{"ip": "10.0.0.1"}),
		}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v", err)
	}
	rows, _, err := activity.New(db).List(ctx, activity.ListFilter{ScopePlatform: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("rolled back record left rows: %+v", rows)
	}
}

func TestRecordAlonePersists(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	log := events.New(activity.New(db))
	ctx := context.Background()

	if err := log.RecordAlone(ctx, events.Entry{
		Verb:       events.VerbSessionLoginFailed,
		ObjectType: events.ObjectLogin,
		ObjectID:   "unknown",
		Delta:      events.Delta(map[string]any{"email": "x@y.z", "ip": "9.9.9.9", "password": "nope"}),
	}); err != nil {
		t.Fatal(err)
	}
	rows, _, err := activity.New(db).List(ctx, activity.ListFilter{ScopePlatform: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Verb != string(events.VerbSessionLoginFailed) {
		t.Errorf("verb = %s", rows[0].Verb)
	}
	if strings.Contains(string(rows[0].Delta), "nope") {
		t.Errorf("password leaked into delta: %s", rows[0].Delta)
	}
	if !strings.Contains(string(rows[0].Delta), "x@y.z") {
		t.Errorf("email missing from delta: %s", rows[0].Delta)
	}
}

func TestRedactionAcrossM1Verbs(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	log := events.New(activity.New(db))
	ctx := context.Background()

	secrets := []string{
		"$argon2id$v=19$m=65536,t=3,p=4$secretHashValueHere",
		"bl_K7QM4TZB2A_2VHQ5XKPYD3JW8N6RTFA9CBEM4SZUG7LH2QX3KVD5RY",
		"JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP", // totp-shaped
		"session-token-value-should-not-appear",
		"3K9M-2PTV-XA47-QRJH-58WY",
	}
	entries := []events.Entry{
		{Verb: events.VerbSessionLogin, ObjectType: events.ObjectSession, ObjectID: "s1",
			ActorID: "u1", Delta: events.Delta(map[string]any{"ip": "1.1.1.1", "session_token": secrets[3]})},
		{Verb: events.VerbSessionLoginFailed, ObjectType: events.ObjectLogin, ObjectID: "unknown",
			Delta: events.Delta(map[string]any{"email": "a@b.c", "password": "pw", "ip": "2.2.2.2"})},
		{Verb: events.VerbTokenCreated, ObjectType: events.ObjectToken, ObjectID: "t1",
			ActorID: "u1", Delta: events.Delta(map[string]any{"prefix": "ABC", "token": secrets[1]})},
		{Verb: events.VerbMFAEnrolled, ObjectType: events.ObjectTOTP, ObjectID: "u1",
			ActorID: "u1", Delta: events.Delta(map[string]any{"totp_secret": secrets[2]})},
		{Verb: events.VerbMFARecoveryUsed, ObjectType: events.ObjectRecoveryCode, ObjectID: "c1",
			ActorID: "u1", Delta: events.Delta(map[string]any{"recovery_code": secrets[4], "remaining": 9})},
		{Verb: events.VerbUserPasswordChanged, ObjectType: events.ObjectUser, ObjectID: "u1",
			ActorID: "u1", Delta: events.Delta(map[string]any{"password_hash": secrets[0]})},
	}

	err := db.Write(ctx, func(tx *sql.Tx) error {
		for _, e := range entries {
			if err := log.Record(ctx, tx, e); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, _, err := activity.New(db).List(ctx, activity.ListFilter{ScopePlatform: true, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(entries) {
		t.Fatalf("got %d rows, want %d", len(rows), len(entries))
	}
	blob, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range secrets {
		if strings.Contains(string(blob), secret) {
			t.Errorf("serialized activity contains secret %q", secret)
		}
	}
	// password plaintext and hash key names
	for _, banned := range []string{"\"password\"", "hunter", "session-token-value"} {
		if strings.Contains(string(blob), banned) {
			t.Errorf("serialized activity contains %q", banned)
		}
	}
}

// --- M4-002: Activity → SSE fan-out -----------------------------------------
	// Not parallel — uses global PostCommitFanout.
func TestRecordFansOutToEngagementTopic(t *testing.T) {
	db := storetest.Migrated(t)
	hub := events.NewHub(events.Options{})
	log := events.New(activity.New(db))
	log.SetHub(hub)

	// Must set the fanout queue so DB.Write flushes it.
	origFanout := store.PostCommitFanout.Load()
	store.PostCommitFanout.Store(new(store.FanoutQueue))
	t.Cleanup(func() { store.PostCommitFanout.Store(origFanout) })

	engID := "019385a2-1234-7890-abcd-ef0123456789"

	// Subscribe to the engagement topic before publishing.
	ch, unsub, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	// Record activity inside a write transaction.
	err = db.Write(t.Context(), func(tx *sql.Tx) error {
		return log.Record(t.Context(), tx, events.Entry{
			EngagementID: engID,
			ActorID:      "actor-1",
			Verb:         events.VerbExecutionRedUpdated,
			ObjectType:   events.ObjectExecution,
			ObjectID:     "exec-1",
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Receive the fan-out event.
	ev := recvOne(t, ch)
	if ev.Type != string(events.VerbExecutionRedUpdated) {
		t.Fatalf("type = %q, want %q", ev.Type, events.VerbExecutionRedUpdated)
	}
	if ev.Topic != events.EngagementTopic(engID) {
		t.Fatalf("topic = %q, want %q", ev.Topic, events.EngagementTopic(engID))
	}
	if ev.ID == "" {
		t.Fatal("event id is empty")
	}

	// Verify the activity row exists with the same id.
	rows, _, err := activity.New(db).List(t.Context(), activity.ListFilter{
		ScopeEngagement: engID,
		Limit:           10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d activity rows, want 1", len(rows))
	}
	if rows[0].ID != ev.ID {
		t.Fatalf("activity row id = %q, event id = %q", rows[0].ID, ev.ID)
	}
}

	// Not parallel — uses global PostCommitFanout.
func TestRecordRollbackProducesNoSSEEvent(t *testing.T) {
	db := storetest.Migrated(t)
	hub := events.NewHub(events.Options{})
	log := events.New(activity.New(db))
	log.SetHub(hub)

	origFanout := store.PostCommitFanout.Load()
	store.PostCommitFanout.Store(new(store.FanoutQueue))
	t.Cleanup(func() { store.PostCommitFanout.Store(origFanout) })

	engID := "019385a2-1234-7890-abcd-ef0123456789"

	ch, unsub, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	sentinel := errors.New("rollback")
	err = db.Write(t.Context(), func(tx *sql.Tx) error {
		if err := log.Record(t.Context(), tx, events.Entry{
			EngagementID: engID,
			ActorID:      "actor-1",
			Verb:         events.VerbExecutionRedUpdated,
			ObjectType:   events.ObjectExecution,
			ObjectID:     "exec-1",
		}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}

	// No activity rows.
	rows, _, err := activity.New(db).List(t.Context(), activity.ListFilter{
		ScopeEngagement: engID,
		Limit:           10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("rolled back record left %d rows", len(rows))
	}

	// No SSE event.
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event after rollback: %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
}
func TestEventPayloadIsIdRefsOnly(t *testing.T) {
	// Not parallel — uses global PostCommitFanout.
	db := storetest.Migrated(t)
	hub := events.NewHub(events.Options{})
	log := events.New(activity.New(db))
	log.SetHub(hub)

	origFanout := store.PostCommitFanout.Load()
	store.PostCommitFanout.Store(new(store.FanoutQueue))
	t.Cleanup(func() { store.PostCommitFanout.Store(origFanout) })

	engID := "019385a2-1234-7890-abcd-ef0123456789"

	ch, unsub, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	err = db.Write(t.Context(), func(tx *sql.Tx) error {
		return log.Record(t.Context(), tx, events.Entry{
			EngagementID: engID,
			ActorID:      "actor-1",
			Verb:         events.VerbCommentCreated,
			ObjectType:   events.ObjectComment,
			ObjectID:     "comment-1",
			ParentIDs:    map[string]string{"executionId": "exec-1"},
			Delta:        events.Delta(map[string]any{"body": "hello"}),
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	ev := recvOne(t, ch)

	var data map[string]any
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("unmarshal event data: %v", err)
	}

	// Must have id-ref fields.
	for _, key := range []string{"engagementId", "actorId", "verb", "objectType", "objectId"} {
		if _, ok := data[key]; !ok {
			t.Errorf("event data missing key %q", key)
		}
	}
	// Must have parent refs.
	if data["executionId"] != "exec-1" {
		t.Errorf("executionId = %v, want exec-1", data["executionId"])
	}

	// Must NOT have secrets or full bodies.
	for _, banned := range []string{"body", "delta", "password", "secret"} {
		if _, ok := data[banned]; ok {
			t.Errorf("event data contains banned key %q", banned)
		}
	}
}

func TestPlatformActivityDoesNotFanOut(t *testing.T) {
	// Not parallel — uses global PostCommitFanout.
	db := storetest.Migrated(t)
	hub := events.NewHub(events.Options{})
	log := events.New(activity.New(db))
	log.SetHub(hub)
	origFanout := store.PostCommitFanout.Load()
	store.PostCommitFanout.Store(new(store.FanoutQueue))
	t.Cleanup(func() { store.PostCommitFanout.Store(origFanout) })

	engID := "019385a2-1234-7890-abcd-ef0123456789"

	ch, unsub, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	// Record a platform event (no engagement_id).
	err = db.Write(t.Context(), func(tx *sql.Tx) error {
		return log.Record(t.Context(), tx, events.Entry{
			ActorID:    "actor-1",
			Verb:       events.VerbSessionLogin,
			ObjectType: events.ObjectSession,
			ObjectID:   "session-1",
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// No SSE event on the engagement topic.
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event for platform activity: %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRecordAloneFansOutAfterCommit(t *testing.T) {
	db := storetest.Migrated(t)
	hub := events.NewHub(events.Options{})
	log := events.New(activity.New(db))
	log.SetHub(hub)

	engID := "019385a2-1234-7890-abcd-ef0123456789"

	ch, unsub, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	// RecordAlone opens its own transaction and publishes after commit.
	err = log.RecordAlone(t.Context(), events.Entry{
		EngagementID: engID,
		ActorID:      "actor-1",
		Verb:         events.VerbCommentCreated,
		ObjectType:   events.ObjectComment,
		ObjectID:     "comment-1",
		ParentIDs:    map[string]string{"executionId": "exec-1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ev := recvOne(t, ch)
	if ev.Type != string(events.VerbCommentCreated) {
		t.Fatalf("type = %q, want %q", ev.Type, events.VerbCommentCreated)
	}
	if ev.ID == "" {
		t.Fatal("event id is empty")
	}
}

// recvOne receives one event or fails the test.
func recvOne(t *testing.T, ch <-chan events.Event) events.Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
	return events.Event{}
}
