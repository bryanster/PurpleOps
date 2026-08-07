// Package scoring holds the ATT&CK Evaluations scoring vocabulary as pure
// functions: detection category ordinals, derived outcome, mean-time-to-detect,
// and modifier validation.
//
// It imports nothing outside the standard library so it can be used from the
// store, the HTTP layer, and any future analytics package without dragging in a
// database or transport dependency.
//
// Outcome is derived from category × protection — it is never stored. MTTD is
// detected_at − started_at when both are set. Modifiers are descriptive only:
// they never alter the ordinal (M3-EPIC).
//
// Every public function is table-driven tested in scoring_test.go; the
// matrix in docs/scoring.md is normative for UI copy.
package scoring

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// ---------------------------------------------------------------------------
// Category — the ordinal detection rating (0–4)
// ---------------------------------------------------------------------------

// Category is the blue-side detection rating. The ordinal order is
// none < telemetry < general < tactic < technique.
type Category string

const (
	CategoryNone      Category = "none"
	CategoryTelemetry Category = "telemetry"
	CategoryGeneral   Category = "general"
	CategoryTactic    Category = "tactic"
	CategoryTechnique Category = "technique"
)

// Valid reports whether c is a known category.
func (c Category) Valid() bool {
	switch c {
	case CategoryNone, CategoryTelemetry, CategoryGeneral, CategoryTactic, CategoryTechnique:
		return true
	}
	return false
}

// Ordinal maps a category to its numeric position (0..4). An unknown category
// returns -1 and an error.
func Ordinal(c Category) (int, error) {
	switch c {
	case CategoryNone:
		return 0, nil
	case CategoryTelemetry:
		return 1, nil
	case CategoryGeneral:
		return 2, nil
	case CategoryTactic:
		return 3, nil
	case CategoryTechnique:
		return 4, nil
	default:
		return -1, fmt.Errorf("scoring: unknown detection category %q", c)
	}
}

// CompareOrdinal compares two categories by their ordinal values.
// Returns -1 if a < b, 0 if equal, +1 if a > b. An error is returned when
// either category is unknown.
func CompareOrdinal(a, b Category) (int, error) {
	oa, err := Ordinal(a)
	if err != nil {
		return 0, err
	}
	ob, err := Ordinal(b)
	if err != nil {
		return 0, err
	}
	return oa - ob, nil
}

// AllCategories returns every known Category in ordinal order.
func AllCategories() []Category {
	return []Category{CategoryNone, CategoryTelemetry, CategoryGeneral, CategoryTactic, CategoryTechnique}
}

// ParseCategory converts a wire string to a Category, rejecting unknowns.
func ParseCategory(s string) (Category, error) {
	c := Category(s)
	if !c.Valid() {
		return "", fmt.Errorf("scoring: unknown detection category %q", s)
	}
	return c, nil
}

// ---------------------------------------------------------------------------
// Protection — blue-side prevention rating
// ---------------------------------------------------------------------------

// Protection is the blue-side prevention rating.
type Protection string

const (
	ProtectionBlocked    Protection = "blocked"
	ProtectionPartial    Protection = "partial"
	ProtectionNotBlocked Protection = "not_blocked"
	ProtectionNA         Protection = "n/a"
)

// Valid reports whether p is a known protection level.
func (p Protection) Valid() bool {
	switch p {
	case ProtectionBlocked, ProtectionPartial, ProtectionNotBlocked, ProtectionNA:
		return true
	}
	return false
}

// AllProtections returns every known Protection value.
func AllProtections() []Protection {
	return []Protection{ProtectionBlocked, ProtectionPartial, ProtectionNotBlocked, ProtectionNA}
}

// ParseProtection converts a wire string to a Protection, rejecting unknowns.
func ParseProtection(s string) (Protection, error) {
	p := Protection(s)
	if !p.Valid() {
		return "", fmt.Errorf("scoring: unknown protection %q", s)
	}
	return p, nil
}

// ---------------------------------------------------------------------------
// Modifiers — descriptive flags, never alter ordinal
// ---------------------------------------------------------------------------

// KnownModifiers is the closed set of detection modifiers from PLAN.md §4.
// Modifiers are multi-select and descriptive only — they never change the
// category ordinal.
var KnownModifiers = []string{
	"alert",
	"correlated",
	"delayed",
	"config_change",
	"residual_artifact",
}

// knownModifierSet is the lookup map built from KnownModifiers.
var knownModifierSet = func() map[string]bool {
	m := make(map[string]bool, len(KnownModifiers))
	for _, mod := range KnownModifiers {
		m[mod] = true
	}
	return m
}()

// ValidateModifiers checks every modifier against the closed set. Unknown
// entries are rejected. Duplicates are collapsed silently — the caller
// receives a deduplicated, order-preserving slice.
//
// An empty slice is valid.
func ValidateModifiers(mods []string) ([]string, error) {
	if len(mods) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(mods))
	deduped := make([]string, 0, len(mods))
	for _, m := range mods {
		if !knownModifierSet[m] {
			return nil, fmt.Errorf("scoring: unknown detection modifier %q", m)
		}
		if !seen[m] {
			seen[m] = true
			deduped = append(deduped, m)
		}
	}
	return deduped, nil
}

// ParseModifier converts a wire string to a modifier identifier, rejecting
// unknowns. Unlike ValidateModifiers, this is a single-value round-trip — use
// ValidateModifiers for the multi-select validation path.
func ParseModifier(s string) (string, error) {
	if knownModifierSet[s] {
		return s, nil
	}
	return "", fmt.Errorf("scoring: unknown detection modifier %q", s)
}

// ---------------------------------------------------------------------------
// Outcome — derived, never persisted
// ---------------------------------------------------------------------------

// Outcome is a derived label combining detection category and protection
// into a single human-readable summary. It is never stored in the database
// — the analytics layer (M5) computes it from the two columns.
type Outcome string

const (
	// NotApplicable — protection is n/a (blue did not report).
	OutcomeNotApplicable Outcome = "not_applicable"

	// Prevented — the attack was blocked or partially blocked.
	OutcomePrevented Outcome = "prevented"

	// Detected — detected but not prevented. The ordinal of the detection
	// category controls the quality of the detection.
	OutcomeDetected Outcome = "detected"

	// NotDetected — no detection reported (category is none).
	OutcomeNotDetected Outcome = "not_detected"
)

// DeriveOutcome returns the derived outcome for a given category and
// protection pair. The outcome matrix (see docs/scoring.md):
//
//	               none  telemetry  general  tactic  technique
//	blocked       prev   prev       prev     prev    prev
//	partial       prev   prev       prev     prev    prev
//	not_blocked   none   det        det      det     det
//	n/a           n/a    n/a        n/a      n/a     n/a
//
// Both category and protection must be present (non-nil). Errors are returned
// for unknown values.
func DeriveOutcome(cat Category, prot Protection) (Outcome, error) {
	if !cat.Valid() {
		return "", fmt.Errorf("scoring: unknown detection category %q", cat)
	}
	if !prot.Valid() {
		return "", fmt.Errorf("scoring: unknown protection %q", prot)
	}

	switch prot {
	case ProtectionBlocked, ProtectionPartial:
		// Any detection category + prevented = outcome is Prevention.
		return OutcomePrevented, nil
	case ProtectionNotBlocked:
		if cat == CategoryNone {
			return OutcomeNotDetected, nil
		}
		return OutcomeDetected, nil
	case ProtectionNA:
		return OutcomeNotApplicable, nil
	default:
		return "", fmt.Errorf("scoring: unhandled protection %q", prot)
	}
}

// DeriveOutcomePtr is DeriveOutcome but accepts optional pointers for
// category and protection. When either is nil the outcome is an empty string
// with no error — the blue side hasn't been scored yet.
func DeriveOutcomePtr(cat *Category, prot *Protection) (Outcome, error) {
	if cat == nil || prot == nil {
		return "", nil
	}
	return DeriveOutcome(*cat, *prot)
}

// ---------------------------------------------------------------------------
// MTTD — mean time to detect
// ---------------------------------------------------------------------------

// MTTD computes detected_at − started_at. Returns ok=false when either
// timestamp is nil. Returns an error when detected_at precedes started_at
// (impossible by the product rules — M3-007 enforces detected_at ≥
// started_at on write — but the function still guards).
func MTTD(startedAt, detectedAt *time.Time) (time.Duration, bool, error) {
	if startedAt == nil || detectedAt == nil {
		return 0, false, nil
	}
	d := detectedAt.Sub(*startedAt)
	if d < 0 {
		return 0, false, errors.New("scoring: detected_at precedes started_at")
	}
	return d, true, nil
}

// ---------------------------------------------------------------------------
// Enum sets for OpenAPI drift testing
// ---------------------------------------------------------------------------

// CategoryStrings returns the string values of every known Category.
func CategoryStrings() []string {
	all := AllCategories()
	out := make([]string, len(all))
	for i, c := range all {
		out[i] = string(c)
	}
	return out
}

// ProtectionStrings returns the string values of every known Protection.
func ProtectionStrings() []string {
	all := AllProtections()
	out := make([]string, len(all))
	for i, p := range all {
		out[i] = string(p)
	}
	return out
}

// ModifierStrings returns the known modifier labels as a sorted copy.
func ModifierStrings() []string {
	out := make([]string, len(KnownModifiers))
	copy(out, KnownModifiers)
	slices.Sort(out)
	return out
}
