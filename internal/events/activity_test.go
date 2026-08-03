package events_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/events"
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
