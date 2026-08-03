package activity_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/store/activity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestInsertAndListRoundTrip(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	repo := activity.New(db)
	ctx := context.Background()

	var first, second activity.Row
	err := db.Write(ctx, func(tx *sql.Tx) error {
		var err error
		first, err = repo.Insert(ctx, tx, activity.Entry{
			ActorID:    "actor-1",
			Verb:       "session.login",
			ObjectType: "session",
			ObjectID:   "sess-1",
			Delta:      json.RawMessage(`{"ip":"1.2.3.4"}`),
			At:         time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		})
		if err != nil {
			return err
		}
		second, err = repo.Insert(ctx, tx, activity.Entry{
			EngagementID: "eng-1",
			ActorID:      "actor-1",
			Verb:         "comment.created",
			ObjectType:   "comment",
			ObjectID:     "c-1",
			At:           time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC),
		})
		return err
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if first.ID == "" || second.ID == "" {
		t.Fatal("expected ids")
	}
	if first.EngagementID != "" {
		t.Errorf("platform row engagement = %q, want empty", first.EngagementID)
	}
	if second.EngagementID != "eng-1" {
		t.Errorf("engagement = %q, want eng-1", second.EngagementID)
	}

	platform, next, err := repo.List(ctx, activity.ListFilter{ScopePlatform: true, Limit: 10})
	if err != nil {
		t.Fatalf("list platform: %v", err)
	}
	if next != "" {
		t.Errorf("nextCursor = %q, want empty", next)
	}
	if len(platform) != 1 || platform[0].ID != first.ID {
		t.Fatalf("platform = %+v, want only first", platform)
	}
	if string(platform[0].Delta) != `{"ip":"1.2.3.4"}` {
		t.Errorf("delta = %s", platform[0].Delta)
	}

	eng, _, err := repo.List(ctx, activity.ListFilter{ScopeEngagement: "eng-1", Limit: 10})
	if err != nil {
		t.Fatalf("list engagement: %v", err)
	}
	if len(eng) != 1 || eng[0].ID != second.ID {
		t.Fatalf("engagement = %+v, want only second", eng)
	}
}

func TestInsertRollsBackWithTheTransaction(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	repo := activity.New(db)
	ctx := context.Background()
	sentinel := errors.New("boom")

	err := db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := repo.Insert(ctx, tx, activity.Entry{
			ActorID:    "a",
			Verb:       "session.login",
			ObjectType: "session",
			ObjectID:   "s",
		}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Write error = %v, want sentinel", err)
	}

	rows, _, err := repo.List(ctx, activity.ListFilter{ScopePlatform: true, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rolled-back write left %d rows: %+v", len(rows), rows)
	}
}

func TestListOrdersByAtThenIDAndPaginates(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	repo := activity.New(db)
	ctx := context.Background()

	// Same millisecond for two rows: id (UUIDv7) is the tiebreaker.
	at := time.Date(2026, 6, 1, 12, 0, 0, 123000000, time.UTC)
	var ids []string
	err := db.Write(ctx, func(tx *sql.Tx) error {
		for i := range 5 {
			row, err := repo.Insert(ctx, tx, activity.Entry{
				ActorID:    "a",
				Verb:       "session.login",
				ObjectType: "session",
				ObjectID:   "s",
				At:         at.Add(time.Duration(i) * time.Millisecond),
			})
			if err != nil {
				return err
			}
			ids = append(ids, row.ID)
		}
		// Two more at the same timestamp as the last one.
		same := at.Add(4 * time.Millisecond)
		for range 2 {
			row, err := repo.Insert(ctx, tx, activity.Entry{
				ActorID:    "a",
				Verb:       "session.login",
				ObjectType: "session",
				ObjectID:   "s",
				At:         same,
			})
			if err != nil {
				return err
			}
			ids = append(ids, row.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	page1, cursor, err := repo.List(ctx, activity.ListFilter{ScopePlatform: true, Limit: 3})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if cursor == "" {
		t.Fatal("expected next cursor")
	}
	if len(page1) != 3 {
		t.Fatalf("page1 len = %d, want 3", len(page1))
	}
	// Newest first.
	for i := 1; i < len(page1); i++ {
		if page1[i-1].At.Before(page1[i].At) {
			t.Fatalf("not ordered by at desc: %v then %v", page1[i-1].At, page1[i].At)
		}
		if page1[i-1].At.Equal(page1[i].At) && page1[i-1].ID <= page1[i].ID {
			t.Fatalf("same-at tiebreaker not id desc: %s then %s", page1[i-1].ID, page1[i].ID)
		}
	}

	page2, cursor2, err := repo.List(ctx, activity.ListFilter{ScopePlatform: true, Limit: 3, Cursor: cursor})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 3 {
		t.Fatalf("page2 len = %d, want 3", len(page2))
	}
	seen := map[string]bool{}
	for _, row := range append(page1, page2...) {
		if seen[row.ID] {
			t.Fatalf("duplicate id across pages: %s", row.ID)
		}
		seen[row.ID] = true
	}

	page3, cursor3, err := repo.List(ctx, activity.ListFilter{ScopePlatform: true, Limit: 3, Cursor: cursor2})
	if err != nil {
		t.Fatalf("page3: %v", err)
	}
	if cursor3 != "" {
		t.Errorf("final cursor = %q, want empty", cursor3)
	}
	if len(page3) != 1 {
		t.Fatalf("page3 len = %d, want 1", len(page3))
	}
}

func TestListFilters(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	repo := activity.New(db)
	ctx := context.Background()

	err := db.Write(ctx, func(tx *sql.Tx) error {
		for _, e := range []activity.Entry{
			{ActorID: "a1", Verb: "session.login", ObjectType: "session", ObjectID: "s1"},
			{ActorID: "a2", Verb: "session.login", ObjectType: "session", ObjectID: "s2"},
			{ActorID: "a1", Verb: "token.created", ObjectType: "service_token", ObjectID: "t1"},
		} {
			if _, err := repo.Insert(ctx, tx, e); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, _, err := repo.List(ctx, activity.ListFilter{
		ScopePlatform: true, ActorID: "a1", Verb: "session.login", Limit: 10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].ObjectID != "s1" {
		t.Fatalf("filtered = %+v", rows)
	}
}

func TestInsertRequiresTransaction(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	repo := activity.New(db)
	_, err := repo.Insert(context.Background(), nil, activity.Entry{
		Verb: "x", ObjectType: "y", ObjectID: "z",
	})
	if err == nil {
		t.Fatal("expected error for nil tx")
	}
}
