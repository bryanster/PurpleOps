package analytics

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store/blind"
)

// ============================================================================
// Helpers
// ============================================================================

func compareScope(baseID, curID string, baseBlind bool, baseSeat authz.EngagementRole, curBlind bool, curSeat authz.EngagementRole) CompareScope {
	return CompareScope{
		Baseline: Scope{
			EngagementID: baseID,
			Blind:        blind.Scope{Blind: baseBlind, Seat: baseSeat},
		},
		Current: Scope{
			EngagementID: curID,
			Blind:        blind.Scope{Blind: curBlind, Seat: curSeat},
		},
	}
}

// compareRowKey builds a sortable key for a compare row.
func compareRowKey(r CompareRow) string {
	return fmt.Sprintf("%s|%s", r.TechniqueID, r.SubtechniqueID)
}

// cmpRowsByID indexes rows by (technique_id, subtechnique_id).
func cmpRowsByID(rows []CompareRow) map[string]CompareRow {
	m := make(map[string]CompareRow, len(rows))
	for _, r := range rows {
		m[compareRowKey(r)] = r
	}
	return m
}

type cmpExpect struct {
	Classification          string
	BaselineCategory        string
	CurrentCategory         string
	OrdinalDelta            *int
	BaselineCategoryOrdinal *int
	CurrentCategoryOrdinal  *int
}

func ordPtr(v int) *int { return &v }

func checkCmpRow(t *testing.T, byID map[string]CompareRow, techID, subTechID string, want cmpExpect) {
	t.Helper()
	key := fmt.Sprintf("%s|%s", techID, subTechID)
	row, ok := byID[key]
	if !ok {
		t.Errorf("row %q not found", key)
		return
	}
	if row.Classification != want.Classification {
		t.Errorf("%s classification: got %q, want %q", key, row.Classification, want.Classification)
	}
	if row.BaselineCategory != want.BaselineCategory {
		t.Errorf("%s baseline category: got %q, want %q", key, row.BaselineCategory, want.BaselineCategory)
	}
	if row.CurrentCategory != want.CurrentCategory {
		t.Errorf("%s current category: got %q, want %q", key, row.CurrentCategory, want.CurrentCategory)
	}
	if want.OrdinalDelta != nil {
		if row.OrdinalDelta == nil || *row.OrdinalDelta != *want.OrdinalDelta {
			var got interface{} = row.OrdinalDelta
			t.Errorf("%s ordinal delta: got %v, want %d", key, got, *want.OrdinalDelta)
		}
	} else if row.OrdinalDelta != nil {
		t.Errorf("%s ordinal delta: got %d, want nil", key, *row.OrdinalDelta)
	}
	if want.BaselineCategoryOrdinal != nil {
		if row.BaselineCategoryOrdinal == nil || *row.BaselineCategoryOrdinal != *want.BaselineCategoryOrdinal {
			var got interface{} = row.BaselineCategoryOrdinal
			t.Errorf("%s baseline ordinal: got %v, want %d", key, got, *want.BaselineCategoryOrdinal)
		}
	} else if row.BaselineCategoryOrdinal != nil {
		t.Errorf("%s baseline ordinal: got %d, want nil", key, *row.BaselineCategoryOrdinal)
	}
	if want.CurrentCategoryOrdinal != nil {
		if row.CurrentCategoryOrdinal == nil || *row.CurrentCategoryOrdinal != *want.CurrentCategoryOrdinal {
			var got interface{} = row.CurrentCategoryOrdinal
			t.Errorf("%s current ordinal: got %v, want %d", key, got, *want.CurrentCategoryOrdinal)
		}
	} else if row.CurrentCategoryOrdinal != nil {
		t.Errorf("%s current ordinal: got %d, want nil", key, *row.CurrentCategoryOrdinal)
	}
}

// ============================================================================
// Compare tests — all seats (baseline blind, retest standard)
// ============================================================================

func TestCompare_AllSeats(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead, // baseline is blind, lead sees all
		false, authz.EngagementRoleBlue, // retest is standard, any seat sees all
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// Hand-computed expectations (see fixture design in analyticstest/fixture.go):
	//
	// Baseline attempted (all seats, best-of per technique):
	//   T1059:     tactic(3),     not_blocked(1)
	//   T1059.001: unscored
	//   T1027:     general(2),    partial(2)
	//   T1070:     technique(4),  not_blocked(1)
	//   T1190:     general(2),    blocked(3)
	//   T1203:     none(0),       not_blocked(1)
	//   T1566:     telemetry(1),  not_blocked(1)
	//
	// Retest attempted:
	//   T1059:     technique(4),  not_blocked(1)
	//   T1059.001: general(2),    not_blocked(1)
	//   T1059.003: tactic(3),     not_blocked(1)  [only in retest]
	//   T1070:     technique(4),  n/a(0)
	//   T1190:     technique(4),  not_blocked(1)
	//   T1566:     technique(4),  not_blocked(1)
	//
	// Classification:
	//   T1059:     baseline 3→4 current → improved (+1)
	//   T1059.001: baseline unscored → incomparable
	//   T1059.003: baseline absent → newlyAttempted
	//   T1027:     current absent → noLongerAttempted
	//   T1070:     same ordinal(4), prot 1→0 → regressed
	//   T1190:     baseline 2→4 → improved (+2)
	//   T1203:     current absent → noLongerAttempted
	//   T1566:     baseline 1→4 → improved (+3)

	byID := cmpRowsByID(result.Rows)

	checkCmpRow(t, byID, "T1059", "", cmpExpect{
		Classification:          "improved",
		BaselineCategory:        "tactic",
		CurrentCategory:         "technique",
		OrdinalDelta:            ordPtr(1),
		BaselineCategoryOrdinal: ordPtr(3),
		CurrentCategoryOrdinal:  ordPtr(4),
	})
	checkCmpRow(t, byID, "T1059.001", "", cmpExpect{
		Classification:          "incomparable",
		BaselineCategory:        "",
		CurrentCategory:         "general",
		BaselineCategoryOrdinal: nil,
		CurrentCategoryOrdinal:  ordPtr(2),
	})
	checkCmpRow(t, byID, "T1059.003", "", cmpExpect{
		Classification:          "newlyAttempted",
		BaselineCategory:        "",
		CurrentCategory:         "tactic",
		BaselineCategoryOrdinal: nil,
		CurrentCategoryOrdinal:  ordPtr(3),
	})
	checkCmpRow(t, byID, "T1027", "", cmpExpect{
		Classification:          "noLongerAttempted",
		BaselineCategory:        "general",
		CurrentCategory:         "",
		BaselineCategoryOrdinal: ordPtr(2),
		CurrentCategoryOrdinal:  nil,
	})
	checkCmpRow(t, byID, "T1070", "", cmpExpect{
		Classification:          "regressed",
		BaselineCategory:        "technique",
		CurrentCategory:         "technique",
		OrdinalDelta:            ordPtr(0),
		BaselineCategoryOrdinal: ordPtr(4),
		CurrentCategoryOrdinal:  ordPtr(4),
	})
	checkCmpRow(t, byID, "T1190", "", cmpExpect{
		Classification:          "improved",
		BaselineCategory:        "general",
		CurrentCategory:         "technique",
		OrdinalDelta:            ordPtr(2),
		BaselineCategoryOrdinal: ordPtr(2),
		CurrentCategoryOrdinal:  ordPtr(4),
	})
	checkCmpRow(t, byID, "T1203", "", cmpExpect{
		Classification:          "noLongerAttempted",
		BaselineCategory:        "none",
		CurrentCategory:         "",
		BaselineCategoryOrdinal: ordPtr(0),
		CurrentCategoryOrdinal:  nil,
	})
	checkCmpRow(t, byID, "T1566", "", cmpExpect{
		Classification:          "improved",
		BaselineCategory:        "telemetry",
		CurrentCategory:         "technique",
		OrdinalDelta:            ordPtr(3),
		BaselineCategoryOrdinal: ordPtr(1),
		CurrentCategoryOrdinal:  ordPtr(4),
	})

	// Summary counts.
	if result.Improved != 3 {
		t.Errorf("improved: got %d, want 3", result.Improved)
	}
	if result.Regressed != 1 {
		t.Errorf("regressed: got %d, want 1", result.Regressed)
	}
	if result.Unchanged != 0 {
		t.Errorf("unchanged: got %d, want 0", result.Unchanged)
	}
	if result.NewlyAttempted != 1 {
		t.Errorf("newlyAttempted: got %d, want 1", result.NewlyAttempted)
	}
	if result.NoLongerAttempted != 2 {
		t.Errorf("noLongerAttempted: got %d, want 2", result.NoLongerAttempted)
	}
	if result.Incomparable != 1 {
		t.Errorf("incomparable: got %d, want 1", result.Incomparable)
	}

	// Pin mismatch: both pin 99.0.
	if result.PinMismatch != nil {
		t.Errorf("pinMismatch: got %+v, want nil", result.PinMismatch)
	}

	// Row count: 8 technique rows.
	if len(result.Rows) != 8 {
		t.Errorf("row count: got %d, want 8", len(result.Rows))
	}
}

// ============================================================================
// Blue seat — unrevealed techniques must not leak as newlyAttempted
// ============================================================================

func TestCompare_BlueSeat_BlindLeakPrevention(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	// Blue seat of blind baseline vs standard retest.
	// In the baseline, T1203 and T1059 are unrevealed → blue cannot see them.
	// T1059 exists in both engagements; without leak suppression it would
	// appear as newlyAttempted (absent from baseline, present in retest).
	// The leak filter must suppress it.
	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleBlue, // blue in blind baseline
		false, authz.EngagementRoleBlue, // standard retest
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	byID := cmpRowsByID(result.Rows)

	// T1059 must NOT appear — it's suppressed by the leak filter.
	if _, ok := byID["T1059|"]; ok {
		t.Error("T1059 appeared in blue compare — leak of unrevealed baseline technique")
	}

	// Expected visible techniques:
	//   T1059.001: incomparable (unscored baseline)
	//   T1059.003: newlyAttempted (genuinely not in baseline)
	//   T1027:     noLongerAttempted (not in retest)
	//   T1070:     regressed (same ordinal, weaker protection)
	//   T1190:     improved
	//   T1566:     improved
	//
	// T1203 is suppressed: it exists in baseline storage but is unrevealed
	// (hidden from blue). Without suppression it would be "noLongerAttempted"
	// leaking the fact that T1203 exists in the baseline. The leak filter
	// catches b.technique_id IS NULL (blue can't see it) AND bat.technique_id
	// IS NOT NULL (it's in baseline storage) → suppressed.
	//
	// T1059 is also suppressed: exists in both, but hidden from blue in baseline.

	checkCmpRow(t, byID, "T1059.001", "", cmpExpect{
		Classification:          "incomparable",
		BaselineCategory:        "",
		CurrentCategory:         "general",
		BaselineCategoryOrdinal: nil,
		CurrentCategoryOrdinal:  ordPtr(2),
	})
	checkCmpRow(t, byID, "T1059.003", "", cmpExpect{
		Classification:          "newlyAttempted",
		BaselineCategory:        "",
		CurrentCategory:         "tactic",
		BaselineCategoryOrdinal: nil,
		CurrentCategoryOrdinal:  ordPtr(3),
	})
	checkCmpRow(t, byID, "T1027", "", cmpExpect{
		Classification:          "noLongerAttempted",
		BaselineCategory:        "general",
		CurrentCategory:         "",
		BaselineCategoryOrdinal: ordPtr(2),
		CurrentCategoryOrdinal:  nil,
	})
	checkCmpRow(t, byID, "T1070", "", cmpExpect{
		Classification:          "regressed",
		BaselineCategory:        "technique",
		CurrentCategory:         "technique",
		OrdinalDelta:            ordPtr(0),
		BaselineCategoryOrdinal: ordPtr(4),
		CurrentCategoryOrdinal:  ordPtr(4),
	})
	checkCmpRow(t, byID, "T1190", "", cmpExpect{
		Classification:          "improved",
		BaselineCategory:        "general",
		CurrentCategory:         "technique",
		OrdinalDelta:            ordPtr(2),
		BaselineCategoryOrdinal: ordPtr(2),
		CurrentCategoryOrdinal:  ordPtr(4),
	})
	checkCmpRow(t, byID, "T1566", "", cmpExpect{
		Classification:          "improved",
		BaselineCategory:        "telemetry",
		CurrentCategory:         "technique",
		OrdinalDelta:            ordPtr(3),
		BaselineCategoryOrdinal: ordPtr(1),
		CurrentCategoryOrdinal:  ordPtr(4),
	})

	// Summary: improved=2, regressed=1, unchanged=0, newlyAttempted=1,
	//          noLongerAttempted=1, incomparable=1
	if result.Improved != 2 {
		t.Errorf("improved: got %d, want 2", result.Improved)
	}
	if result.Regressed != 1 {
		t.Errorf("regressed: got %d, want 1", result.Regressed)
	}
	if result.NewlyAttempted != 1 {
		t.Errorf("newlyAttempted: got %d, want 1", result.NewlyAttempted)
	}
	if result.NoLongerAttempted != 1 {
		t.Errorf("noLongerAttempted: got %d, want 1", result.NoLongerAttempted)
	}
	if result.Incomparable != 1 {
		t.Errorf("incomparable: got %d, want 1", result.Incomparable)
	}

	// Verify T1059 and T1203 are NOT in the row set at all.
	for _, tid := range []string{"T1059", "T1203"} {
		for _, r := range result.Rows {
			if r.TechniqueID == tid && r.SubtechniqueID == "" {
				t.Errorf("%s found in blue compare rows — blind leak", tid)
				break
			}
		}
	}

	// 6 visible rows (T1203 and T1059 suppressed).
	if len(result.Rows) != 6 {
		t.Errorf("row count: got %d, want 6", len(result.Rows))
	}
}

// ============================================================================
// Symmetry: swapping baseline and current inverts every improved/regressed
// ============================================================================

func TestCompare_SymmetryInversion(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead,
		false, authz.EngagementRoleBlue,
	)
	forward, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("forward Compare: %v", err)
	}

	// Swap.
	scopeSwapped := compareScope(
		fx.RetestID, fx.BaselineID,
		false, authz.EngagementRoleBlue,
		true, authz.EngagementRoleLead,
	)
	reverse, err := q.Compare(t.Context(), scopeSwapped)
	if err != nil {
		t.Fatalf("reverse Compare: %v", err)
	}

	// For every row in forward, there must be a row in reverse with
	// the same technique, and the classification must invert.
	fwdByID := cmpRowsByID(forward.Rows)
	revByID := cmpRowsByID(reverse.Rows)

	inverted := map[string]string{
		"improved":          "regressed",
		"regressed":         "improved",
		"unchanged":         "unchanged",
		"newlyAttempted":    "noLongerAttempted",
		"noLongerAttempted": "newlyAttempted",
		"incomparable":      "incomparable",
	}

	for key, fwd := range fwdByID {
		rev, ok := revByID[key]
		if !ok {
			t.Errorf("reverse missing %q", key)
			continue
		}
		want := inverted[fwd.Classification]
		if rev.Classification != want {
			t.Errorf("%s: forward=%q, reverse=%q, want %q",
				key, fwd.Classification, rev.Classification, want)
		}
	}

	// Summary counts should also invert.
	if forward.Improved != reverse.Regressed {
		t.Errorf("forward improved(%d) != reverse regressed(%d)", forward.Improved, reverse.Regressed)
	}
	if forward.Regressed != reverse.Improved {
		t.Errorf("forward regressed(%d) != reverse improved(%d)", forward.Regressed, reverse.Improved)
	}
	if forward.Unchanged != reverse.Unchanged {
		t.Errorf("forward unchanged(%d) != reverse unchanged(%d)", forward.Unchanged, reverse.Unchanged)
	}
	if forward.NewlyAttempted != reverse.NoLongerAttempted {
		t.Errorf("forward newlyAttempted(%d) != reverse noLongerAttempted(%d)",
			forward.NewlyAttempted, reverse.NoLongerAttempted)
	}
	if forward.NoLongerAttempted != reverse.NewlyAttempted {
		t.Errorf("forward noLongerAttempted(%d) != reverse newlyAttempted(%d)",
			forward.NoLongerAttempted, reverse.NewlyAttempted)
	}
	if forward.Incomparable != reverse.Incomparable {
		t.Errorf("forward incomparable(%d) != reverse incomparable(%d)",
			forward.Incomparable, reverse.Incomparable)
	}
}

// ============================================================================
// Self-compare: everything unchanged
// ============================================================================

func TestCompare_SelfCompare(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	scope := compareScope(
		fx.BaselineID, fx.BaselineID,
		true, authz.EngagementRoleLead,
		true, authz.EngagementRoleLead,
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// Most rows must be "unchanged", except for unscored techniques which
	// are "incomparable" (both sides have NULL category).
	incomparableCount := 0
	for _, r := range result.Rows {
		switch r.Classification {
		case "unchanged":
			// Ordinal delta must be 0.
			if r.OrdinalDelta == nil || *r.OrdinalDelta != 0 {
				var got interface{} = r.OrdinalDelta
				t.Errorf("%s ordinal delta: got %v, want 0", r.TechniqueID, got)
			}
			// Baseline and current category must be equal.
			if r.BaselineCategory != r.CurrentCategory {
				t.Errorf("%s categories differ: %q vs %q", r.TechniqueID, r.BaselineCategory, r.CurrentCategory)
			}
		case "incomparable":
			incomparableCount++
			// Both sides must be unscored.
			if r.BaselineCategory != "" || r.CurrentCategory != "" {
				t.Errorf("%s: incomparable but baseline=%q current=%q",
					r.TechniqueID, r.BaselineCategory, r.CurrentCategory)
			}
		default:
			t.Errorf("%s: got %q, want unchanged or incomparable", r.TechniqueID, r.Classification)
		}
	}

	// All non-incomparable rows must be unchanged. Incomparable is allowed
	// for unscored techniques.
	if len(result.Rows) == 0 {
		t.Error("self-compare: got 0 rows")
	}
	if result.Unchanged+result.Incomparable != len(result.Rows) {
		t.Errorf("self-compare: unchanged(%d)+incomparable(%d) != rows(%d)",
			result.Unchanged, result.Incomparable, len(result.Rows))
	}
	if result.Improved != 0 || result.Regressed != 0 || result.NewlyAttempted != 0 ||
		result.NoLongerAttempted != 0 {
		t.Error("self-compare: got non-unchanged/incomparable classifications")
	}
}

// ============================================================================
// Sub-technique non-pairing: T1566.001 != T1566
// ============================================================================

func TestCompare_SubtechniqueNonPairing(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead,
		false, authz.EngagementRoleBlue,
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	byID := cmpRowsByID(result.Rows)

	if _, ok := byID["T1566.001|"]; ok {
		t.Error("T1566.001 (skipped) appeared in compare — not attempted")
	}

	checkCmpRow(t, byID, "T1566", "", cmpExpect{
		Classification:          "improved",
		BaselineCategory:        "telemetry",
		CurrentCategory:         "technique",
		OrdinalDelta:            ordPtr(3),
		BaselineCategoryOrdinal: ordPtr(1),
		CurrentCategoryOrdinal:  ordPtr(4),
	})

	for _, r := range result.Rows {
		if r.TechniqueID == "T1566.001" {
			t.Error("T1566.001 row found — should be absent (not attempted)")
		}
	}
}

// ============================================================================
// subtechnique match test: T1059 (parent) != T1059.001 (sub)
// ============================================================================

func TestCompare_SubtechniqueMatch(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	// Both engagements have T1059 (parent) and T1059.001 (sub).
	// They must appear as separate compare rows — parent must NOT
	// pair with sub, and sub must NOT pair with parent.
	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead,
		false, authz.EngagementRoleBlue,
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	byID := cmpRowsByID(result.Rows)

	// Both T1059 and T1059.001 must be separate rows.
	_, hasParent := byID["T1059|"]
	_, hasSub := byID["T1059.001|"]
	if !hasParent {
		t.Error("T1059 (parent) missing from compare rows")
	}
	if !hasSub {
		t.Error("T1059.001 (sub) missing from compare rows")
	}
}

// ============================================================================
// Pin mismatch
// ============================================================================

func TestCompare_PinMismatch(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	// Both engagements pin 99.0 — no mismatch expected.
	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead,
		false, authz.EngagementRoleBlue,
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if result.PinMismatch != nil {
		t.Errorf("pinMismatch: got %+v, want nil (both pin 99.0)", result.PinMismatch)
	}

	// Now update the retest to pin a different version and re-compare.
	err = fx.DB.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(),
			`UPDATE app.engagement SET attack_version = '98.0' WHERE id = ?`, fx.RetestID)
		return err
	})
	if err != nil {
		t.Fatalf("update retest pin: %v", err)
	}

	result2, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare after pin change: %v", err)
	}

	if result2.PinMismatch == nil {
		t.Fatal("pinMismatch: got nil, want advisory (baseline=99.0, current=98.0)")
	}
	if result2.PinMismatch.Baseline != "99.0" {
		t.Errorf("pinMismatch.Baseline: got %q, want 99.0", result2.PinMismatch.Baseline)
	}
	if result2.PinMismatch.Current != "98.0" {
		t.Errorf("pinMismatch.Current: got %q, want 98.0", result2.PinMismatch.Current)
	}

	// Compare should still produce results despite pin mismatch.
	if len(result2.Rows) == 0 {
		t.Error("no rows after pin change — compare should tolerate mismatch")
	}
}

// ============================================================================
// Empty comparison
// ============================================================================

func TestCompare_EmptyEngagements(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	// Future engagement has no steps → no attempted techniques.
	scope := compareScope(
		fx.FutureID, fx.FutureID,
		false, authz.EngagementRoleLead,
		false, authz.EngagementRoleLead,
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if len(result.Rows) != 0 {
		t.Errorf("empty comparison: got %d rows, want 0", len(result.Rows))
	}
	if result.Improved != 0 || result.Regressed != 0 || result.Unchanged != 0 ||
		result.NewlyAttempted != 0 || result.NoLongerAttempted != 0 || result.Incomparable != 0 {
		t.Error("empty comparison: got non-zero summary counts")
	}
}

// ============================================================================
// Multi-step template_id pairing
// ============================================================================

func TestCompare_TemplateIDPairing(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	// Insert additional steps into the baseline and retest for T1059
	// with distinct non-empty template_ids, to test template_id pairing.
	//
	// Baseline gets a second T1059 step with template_id "tmpl-b".
	// Retest gets two T1059 steps: one with "tmpl-b" (matches baseline)
	// and one with "tmpl-c" (new).
	//
	// Expected: "tmpl-b" pairs across engagements → one row.
	// The unmatched "tmpl-c" (retest only) gets rolled into the best-of
	// for T1059 via template_id-level aggregation.
	//
	// Before additions:
	//   Baseline T1059: tactic(3), not_blocked(1), template_id='{}'
	//   Retest T1059:   technique(4), not_blocked(1), template_id='{}'
	//   → Compare shows T1059 as improved (3→4).
	//
	// After additions:
	//   Baseline T1059 steps:
	//     - original: tactic(3), not_blocked(1), template_id='{}'
	//     - new:      technique(4), blocked(3), template_id='tmpl-b'
	//   Retest T1059 steps:
	//     - original: technique(4), not_blocked(1), template_id='{}'
	//     - new1:     general(2), not_blocked(1), template_id='tmpl-b'
	//     - new2:     none(0), not_blocked(1), template_id='tmpl-c'
	//
	// In the current implementation, template matching is done at the
	// (technique, subtechnique) level rather than at the step level.
	// The best-of aggregation uses the same logic as TechniqueCoverage.
	// Template_id-based pairing within a multi-step technique would
	// produce per-template_id rows alongside the best-of row.
	//
	// For now we verify the compare works with multi-step techniques
	// and that the best-of aggregation handles them correctly.

	err := fx.DB.Write(t.Context(), func(tx *sql.Tx) error {
		// Add a second T1059 step to baseline scenario 2.
		newStepID := "01900000-0000-7000-P000-000000000010"
		newExecID := "01900000-0000-7000-X000-000000000010"
		_, err := tx.ExecContext(t.Context(),
			`INSERT INTO app.step
				(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id,
				 tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope,
				 attack_version, revealed_at, created_at, updated_at)
			 VALUES (?, ?, 6, 'T1059 second step', '', 'T1059', '', '', '{}', 'tmpl-b', '', '[]', '[]',
			         '99.0', '2026-06-01', '2026-06-01', '2026-06-01')`,
			newStepID, fx.BaselineScenarioIDs[1],
		)
		if err != nil {
			return fmt.Errorf("insert baseline T1059 step 2: %w", err)
		}
		_, err = tx.ExecContext(t.Context(),
			`INSERT INTO app.execution
				(id, step_id, version, status, executed_by, started_at, ended_at,
				 command_run, source_host, target_host, red_notes,
				 detection_category, detection_modifiers, protection,
				 detected_at, detecting_source, detecting_rule_ref,
				 alert_severity, blue_notes, scored_by, scored_at,
				 created_at, updated_at)
			 VALUES (?, ?, 1, 'complete', '01900000-0000-7000-U000-000000000001',
			         '2026-06-01', '2026-06-01', '', '', '', '',
			         'technique', '[]', 'blocked',
			         '2026-06-01', '', '', '', '', '01900000-0000-7000-U000-000000000001',
			         '2026-06-01', '2026-06-01', '2026-06-01')`,
			newExecID, newStepID,
		)
		if err != nil {
			return fmt.Errorf("insert baseline T1059 exec 2: %w", err)
		}

		// Add two more T1059 steps to retest scenario 1.
		for i, row := range []struct {
			stepID, execID, tmplID, cat, prot string
		}{
			{"01900000-0000-7000-P000-000000000107", "01900000-0000-7000-X000-000000000107",
				"tmpl-b", "general", "not_blocked"},
			{"01900000-0000-7000-P000-000000000108", "01900000-0000-7000-X000-000000000108",
				"tmpl-c", "none", "not_blocked"},
		} {
			ordinal := 7 + i
			_, err := tx.ExecContext(t.Context(),
				`INSERT INTO app.step
					(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id,
					 tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope,
					 attack_version, revealed_at, created_at, updated_at)
				 VALUES (?, ?, ?, 'T1059 extra step', '', 'T1059', '', '', '{}', ?, '', '[]', '[]',
				         '99.0', '2026-06-01', '2026-06-01', '2026-06-01')`,
				row.stepID, fx.RetestScenarioIDs[0], ordinal, row.tmplID,
			)
			if err != nil {
				return fmt.Errorf("insert retest T1059 step %d: %w", i, err)
			}
			_, err = tx.ExecContext(t.Context(),
				`INSERT INTO app.execution
					(id, step_id, version, status, executed_by, started_at, ended_at,
					 command_run, source_host, target_host, red_notes,
					 detection_category, detection_modifiers, protection,
					 detected_at, detecting_source, detecting_rule_ref,
					 alert_severity, blue_notes, scored_by, scored_at,
					 created_at, updated_at)
				 VALUES (?, ?, 1, 'complete', '01900000-0000-7000-U000-000000000001',
				         '2026-06-01', '2026-06-01', '', '', '', '',
				         ?, '[]', ?,
				         '2026-06-01', '', '', '', '', '01900000-0000-7000-U000-000000000001',
				         '2026-06-01', '2026-06-01', '2026-06-01')`,
				row.execID, row.stepID, row.cat, row.prot,
			)
			if err != nil {
				return fmt.Errorf("insert retest T1059 exec %d: %w", i, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed template-id steps: %v", err)
	}

	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead,
		false, authz.EngagementRoleBlue,
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare after template-id seeding: %v", err)
	}

	byID := cmpRowsByID(result.Rows)

	// T1059 best-of across all steps:
	//   Baseline: tactic(3) + technique(4) → best = technique(4)
	//   Retest:   technique(4) + general(2) + none(0) → best = technique(4)
	//   → unchanged (4==4)
	row, ok := byID["T1059|"]
	if !ok {
		t.Fatal("T1059 missing from compare after multi-step seeding")
	}
	if row.BaselineCategory != "technique" {
		t.Errorf("T1059 baseline category: got %q, want technique", row.BaselineCategory)
	}
	if row.CurrentCategory != "technique" {
		t.Errorf("T1059 current category: got %q, want technique", row.CurrentCategory)
	}

	// T1059 must NOT be a cross product — we should see one row, not many.
	t1059Count := 0
	for _, r := range result.Rows {
		if r.TechniqueID == "T1059" {
			t1059Count++
		}
	}
	if t1059Count != 1 {
		t.Errorf("T1059 row count: got %d, want 1 (no cross product)", t1059Count)
	}

	// Summary: T1059 is regressed (same technique ordinal 4,
	// but baseline protection blocked > current not_blocked).
	// T1070 was already regressed → total 2 regressed.
	if result.Regressed != 2 {
		t.Errorf("regressed: got %d, want 2 (T1070 + T1059)", result.Regressed)
	}
}

// ============================================================================
// Row ordering is deterministic
// ============================================================================

func TestCompare_RowOrdering(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead,
		false, authz.EngagementRoleBlue,
	)

	result1, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare 1: %v", err)
	}
	result2, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare 2: %v", err)
	}

	if len(result1.Rows) != len(result2.Rows) {
		t.Fatalf("row count differs: %d vs %d", len(result1.Rows), len(result2.Rows))
	}
	for i := range result1.Rows {
		if result1.Rows[i].TechniqueID != result2.Rows[i].TechniqueID ||
			result1.Rows[i].SubtechniqueID != result2.Rows[i].SubtechniqueID {
			t.Errorf("row %d differs: %s/%s vs %s/%s",
				i,
				result1.Rows[i].TechniqueID, result1.Rows[i].SubtechniqueID,
				result2.Rows[i].TechniqueID, result2.Rows[i].SubtechniqueID,
			)
		}
	}
}

// ============================================================================
// Classification table completeness — one of each
// ============================================================================

func TestCompare_ClassificationCompleteness(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead,
		false, authz.EngagementRoleBlue,
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	byClass := map[string]int{}
	for _, r := range result.Rows {
		byClass[r.Classification]++
	}

	// Verify each classification appears at least once (per hand-computed).
	for _, cls := range []string{"improved", "regressed", "newlyAttempted", "noLongerAttempted", "incomparable"} {
		if byClass[cls] == 0 {
			t.Errorf("classification %q missing from results", cls)
		}
	}

	// "unchanged" is not present in the default fixture — that's correct.
	// assert that with a comment.
	t.Logf("classification distribution: %v", byClass)
}

// ============================================================================
// Assert compare results are sortable
// ============================================================================

func TestCompare_SortableOutput(t *testing.T) {
	fx := analyticstest.Seed(t)
	q := NewQueries(fx.DB)

	scope := compareScope(
		fx.BaselineID, fx.RetestID,
		true, authz.EngagementRoleLead,
		false, authz.EngagementRoleBlue,
	)

	result, err := q.Compare(t.Context(), scope)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// Verify rows are sorted by (technique_id, subtechnique_id).
	// SQL ORDER BY uses byte-by-byte comparison; verify monotonic ordering.
	var prevID, prevSub string
	for i, r := range result.Rows {
		if i > 0 {
			if r.TechniqueID < prevID {
				t.Errorf("row %d technique_id %q < previous %q — not sorted", i, r.TechniqueID, prevID)
			} else if r.TechniqueID == prevID && r.SubtechniqueID < prevSub {
				t.Errorf("row %d subtechnique_id %q < previous %q — not sorted", i, r.SubtechniqueID, prevSub)
			}
		}
		prevID, prevSub = r.TechniqueID, r.SubtechniqueID
	}
}
