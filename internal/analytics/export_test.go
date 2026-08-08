package analytics

import (
	"context"
	"database/sql"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store/blind"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
)

func TestExecutionsExport_LeadSeat(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)
	ctx := context.Background()

	rows, err := q.ExecutionsExport(ctx, scope(fx.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ExecutionsExport: %v", err)
	}
	defer rows.Close()

	count := 0
	var unrevealedTechs []string
	for rows.Next() {
		count++
		var techniqueID sql.NullString
		if err := rows.Scan(
			new(string), new(int), new(int), new(string),
			&techniqueID, new(sql.NullString), new(sql.NullString),
			new(string), new(string),
			new(string), new(string),
			new(sql.NullTime), new(sql.NullTime),
			new(string), new(string), new(string), new(string),
			new(sql.NullString), new(sql.NullString), new(sql.NullString),
			new(sql.NullTime), new(string), new(string), new(string), new(string),
			new(string), new(sql.NullTime),
			new(sql.NullFloat64), new(string),
		); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if techniqueID.Valid {
			unrevealedTechs = append(unrevealedTechs, techniqueID.String)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	if count != 9 {
		t.Errorf("lead rows = %d, want 9", count)
	}

	foundT1203 := false
	foundT1059 := false
	for _, tid := range unrevealedTechs {
		if tid == "T1203" {
			foundT1203 = true
		}
		if tid == "T1059" {
			foundT1059 = true
		}
	}
	if !foundT1203 {
		t.Error("lead: unrevealed T1203 missing")
	}
	if !foundT1059 {
		t.Error("lead: unrevealed T1059 missing")
	}
}

func TestExecutionsExport_BlueSeat(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)
	ctx := context.Background()

	rows, err := q.ExecutionsExport(ctx, scope(fx.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("ExecutionsExport: %v", err)
	}
	defer rows.Close()

	count := 0
	var techniqueIDs []string
	for rows.Next() {
		count++
		var techniqueID sql.NullString
		if err := rows.Scan(
			new(string), new(int), new(int), new(string),
			&techniqueID, new(sql.NullString), new(sql.NullString),
			new(string), new(string),
			new(string), new(string),
			new(sql.NullTime), new(sql.NullTime),
			new(string), new(string), new(string), new(string),
			new(sql.NullString), new(sql.NullString), new(sql.NullString),
			new(sql.NullTime), new(string), new(string), new(string), new(string),
			new(string), new(sql.NullTime),
			new(sql.NullFloat64), new(string),
		); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if techniqueID.Valid {
			techniqueIDs = append(techniqueIDs, techniqueID.String)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	if count != 7 {
		t.Errorf("blue rows = %d, want 7", count)
	}

	for _, tid := range techniqueIDs {
		if tid == "T1203" || tid == "T1059" {
			t.Errorf("blue: unrevealed technique %q should be hidden", tid)
		}
	}
}

func TestExecutionsExport_EmptyEngagement(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)
	ctx := context.Background()

	s := Scope{
		EngagementID: fx.FutureID,
		Blind:        blind.Scope{Blind: false},
	}
	rows, err := q.ExecutionsExport(ctx, s)
	if err != nil {
		t.Fatalf("ExecutionsExport empty: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if count != 0 {
		t.Errorf("empty engagement rows = %d, want 0", count)
	}
}

func TestFindingsExport_LeadSeat(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)
	ctx := context.Background()

	rows, err := q.FindingsExport(ctx, scope(fx.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("FindingsExport: %v", err)
	}
	defer rows.Close()

	count := 0
	var titles []string
	for rows.Next() {
		count++
		var title string
		if err := rows.Scan(
			&title,
			new(string), new(string), new(string), new(string),
			new(string), new(string),
			new(interface{}), new(interface{}),
		); err != nil {
			t.Fatalf("scan: %v", err)
		}
		titles = append(titles, title)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	if count != 6 {
		t.Errorf("lead findings = %d, want 6 (titles: %v)", count, titles)
	}
}

func TestFindingsExport_BlueSeat(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)
	ctx := context.Background()

	rows, err := q.FindingsExport(ctx, scope(fx.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("FindingsExport: %v", err)
	}
	defer rows.Close()

	count := 0
	seenTitles := make(map[string]bool)
	var linkedStepIDs []string
	for rows.Next() {
		count++
		var title, lsid string
		if err := rows.Scan(
			&title,
			new(string), new(string), new(string), new(string),
			new(string), &lsid,
			new(interface{}), new(interface{}),
		); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seenTitles[title] = true
		if lsid != "" {
			linkedStepIDs = append(linkedStepIDs, lsid)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	if count != 5 {
		t.Errorf("blue findings = %d, want 5", count)
	}

	finding3Title := "Finding 01900000-0000-7000-F000-000000000003"
	if seenTitles[finding3Title] {
		t.Error("blue: finding3 (linked to unrevealed step8) should be excluded")
	}

	unrevealedStepID := "01900000-0000-7000-P000-000000000008"
	for _, lsid := range linkedStepIDs {
		if lsid == unrevealedStepID {
			t.Errorf("blue: linked_step_ids contains unrevealed step %q", unrevealedStepID)
		}
	}
}

func TestFindingsExport_EmptyEngagement(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)
	ctx := context.Background()

	s := Scope{
		EngagementID: fx.FutureID,
		Blind:        blind.Scope{Blind: false},
	}
	rows, err := q.FindingsExport(ctx, s)
	if err != nil {
		t.Fatalf("FindingsExport empty: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if count != 0 {
		t.Errorf("empty engagement findings = %d, want 0", count)
	}
}
