package scoring

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Category / Ordinal
// ---------------------------------------------------------------------------

func TestCategoryValid(t *testing.T) {
	for _, c := range AllCategories() {
		if !c.Valid() {
			t.Errorf("Valid(%q) = false, want true", c)
		}
	}
	if Category("bogus").Valid() {
		t.Error("Valid(\"bogus\") = true, want false")
	}
}

func TestOrdinal(t *testing.T) {
	tests := []struct {
		cat  Category
		want int
	}{
		{CategoryNone, 0},
		{CategoryTelemetry, 1},
		{CategoryGeneral, 2},
		{CategoryTactic, 3},
		{CategoryTechnique, 4},
	}
	for _, tt := range tests {
		got, err := Ordinal(tt.cat)
		if err != nil {
			t.Errorf("Ordinal(%q): %v", tt.cat, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Ordinal(%q) = %d, want %d", tt.cat, got, tt.want)
		}
	}

	_, err := Ordinal(Category("bogus"))
	if err == nil {
		t.Error("Ordinal(\"bogus\") should error")
	}
}

func TestOrdinalMonotonicity(t *testing.T) {
	all := AllCategories()
	for i := 1; i < len(all); i++ {
		oi, err := Ordinal(all[i-1])
		if err != nil {
			t.Fatal(err)
		}
		oj, err := Ordinal(all[i])
		if err != nil {
			t.Fatal(err)
		}
		if oi >= oj {
			t.Errorf("Ordinal(%q)=%d >= Ordinal(%q)=%d", all[i-1], oi, all[i], oj)
		}
	}
}

func TestCompareOrdinal(t *testing.T) {
	// Known ordering
	cmp, err := CompareOrdinal(CategoryNone, CategoryTechnique)
	if err != nil {
		t.Fatal(err)
	}
	if cmp >= 0 {
		t.Errorf("CompareOrdinal(none, technique) = %d, want < 0", cmp)
	}

	cmp, err = CompareOrdinal(CategoryTechnique, CategoryNone)
	if err != nil {
		t.Fatal(err)
	}
	if cmp <= 0 {
		t.Errorf("CompareOrdinal(technique, none) = %d, want > 0", cmp)
	}

	cmp, err = CompareOrdinal(CategoryGeneral, CategoryGeneral)
	if err != nil {
		t.Fatal(err)
	}
	if cmp != 0 {
		t.Errorf("CompareOrdinal(general, general) = %d, want 0", cmp)
	}

	// Unknown
	_, err = CompareOrdinal(Category("bogus"), CategoryNone)
	if err == nil {
		t.Error("CompareOrdinal with bogus first should error")
	}
	_, err = CompareOrdinal(CategoryNone, Category("bogus"))
	if err == nil {
		t.Error("CompareOrdinal with bogus second should error")
	}
}

// ---------------------------------------------------------------------------
// Parse helpers
// ---------------------------------------------------------------------------

func TestParseCategoryRoundTrip(t *testing.T) {
	for _, c := range AllCategories() {
		got, err := ParseCategory(string(c))
		if err != nil {
			t.Errorf("ParseCategory(%q): %v", c, err)
			continue
		}
		if got != c {
			t.Errorf("ParseCategory(%q) = %q", c, got)
		}
	}

	_, err := ParseCategory("bogus")
	if err == nil {
		t.Error("ParseCategory(\"bogus\") should error")
	}
}

func TestParseProtectionRoundTrip(t *testing.T) {
	for _, p := range AllProtections() {
		got, err := ParseProtection(string(p))
		if err != nil {
			t.Errorf("ParseProtection(%q): %v", p, err)
			continue
		}
		if got != p {
			t.Errorf("ParseProtection(%q) = %q", p, got)
		}
	}

	_, err := ParseProtection("bogus")
	if err == nil {
		t.Error("ParseProtection(\"bogus\") should error")
	}
}

// ---------------------------------------------------------------------------
// Modifiers
// ---------------------------------------------------------------------------

func TestKnownModifiersAfterSortAreStable(t *testing.T) {
	// Each modifier round-trips through ParseModifier.
	for _, m := range KnownModifiers {
		got, err := ParseModifier(m)
		if err != nil {
			t.Errorf("ParseModifier(%q): %v", m, err)
			continue
		}
		if got != m {
			t.Errorf("ParseModifier(%q) = %q", m, got)
		}
	}

	_, err := ParseModifier("bogus")
	if err == nil {
		t.Error("ParseModifier(\"bogus\") should error")
	}
}

func TestValidateModifiers(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"empty", nil, nil, false},
		{"single valid", []string{"alert"}, []string{"alert"}, false},
		{"multi valid", []string{"alert", "correlated"}, []string{"alert", "correlated"}, false},
		{"all known", KnownModifiers, KnownModifiers, false},
		{"duplicates collapsed", []string{"alert", "alert", "correlated"}, []string{"alert", "correlated"}, false},
		{"unknown rejected", []string{"alert", "bogus"}, nil, true},
		{"empty slice", []string{}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateModifiers(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateModifiers(%v): err=%v, wantErr=%v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ValidateModifiers(%v) = %v (len %d), want %v (len %d)", tt.in, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ValidateModifiers(%v)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Outcome — derived
// ---------------------------------------------------------------------------

func TestDeriveOutcome(t *testing.T) {
	// All category × protection pairs with hand-written expected values.
	tests := []struct {
		cat  Category
		prot Protection
		want Outcome
	}{
		// n/a protection — always not_applicable
		{CategoryNone, ProtectionNA, OutcomeNotApplicable},
		{CategoryTelemetry, ProtectionNA, OutcomeNotApplicable},
		{CategoryGeneral, ProtectionNA, OutcomeNotApplicable},
		{CategoryTactic, ProtectionNA, OutcomeNotApplicable},
		{CategoryTechnique, ProtectionNA, OutcomeNotApplicable},

		// blocked protection — always prevented
		{CategoryNone, ProtectionBlocked, OutcomePrevented},
		{CategoryTelemetry, ProtectionBlocked, OutcomePrevented},
		{CategoryGeneral, ProtectionBlocked, OutcomePrevented},
		{CategoryTactic, ProtectionBlocked, OutcomePrevented},
		{CategoryTechnique, ProtectionBlocked, OutcomePrevented},

		// partial protection — always prevented
		{CategoryNone, ProtectionPartial, OutcomePrevented},
		{CategoryTelemetry, ProtectionPartial, OutcomePrevented},
		{CategoryGeneral, ProtectionPartial, OutcomePrevented},
		{CategoryTactic, ProtectionPartial, OutcomePrevented},
		{CategoryTechnique, ProtectionPartial, OutcomePrevented},

		// not_blocked — depends on category
		{CategoryNone, ProtectionNotBlocked, OutcomeNotDetected},
		{CategoryTelemetry, ProtectionNotBlocked, OutcomeDetected},
		{CategoryGeneral, ProtectionNotBlocked, OutcomeDetected},
		{CategoryTactic, ProtectionNotBlocked, OutcomeDetected},
		{CategoryTechnique, ProtectionNotBlocked, OutcomeDetected},
	}

	for _, tt := range tests {
		got, err := DeriveOutcome(tt.cat, tt.prot)
		if err != nil {
			t.Errorf("DeriveOutcome(%q, %q): %v", tt.cat, tt.prot, err)
			continue
		}
		if got != tt.want {
			t.Errorf("DeriveOutcome(%q, %q) = %q, want %q", tt.cat, tt.prot, got, tt.want)
		}
	}
}

func TestDeriveOutcomeUnknownInputs(t *testing.T) {
	_, err := DeriveOutcome(Category("bogus"), ProtectionBlocked)
	if err == nil {
		t.Error("DeriveOutcome with bogus category should error")
	}
	_, err = DeriveOutcome(CategoryTechnique, Protection("bogus"))
	if err == nil {
		t.Error("DeriveOutcome with bogus protection should error")
	}
}

func TestDeriveOutcomePtr(t *testing.T) {
	// Both nil → empty, no error
	got, err := DeriveOutcomePtr(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("DeriveOutcomePtr(nil, nil) = %q, want \"\"", got)
	}

	// One nil → empty, no error
	cat := CategoryTechnique
	got, err = DeriveOutcomePtr(&cat, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("DeriveOutcomePtr(technique, nil) = %q, want \"\"", got)
	}

	prot := ProtectionBlocked
	got, err = DeriveOutcomePtr(nil, &prot)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("DeriveOutcomePtr(nil, blocked) = %q, want \"\"", got)
	}

	// Both present → derived
	got, err = DeriveOutcomePtr(&cat, &prot)
	if err != nil {
		t.Fatal(err)
	}
	if got != OutcomePrevented {
		t.Errorf("DeriveOutcomePtr(technique, blocked) = %q, want %q", got, OutcomePrevented)
	}
}

// ---------------------------------------------------------------------------
// MTTD
// ---------------------------------------------------------------------------

func TestMTTD(t *testing.T) {
	now := time.Now().UTC()
	tenSecondsAgo := now.Add(-10 * time.Second)

	// Both nil
	_, ok, err := MTTD(nil, nil)
	if err != nil || ok {
		t.Errorf("MTTD(nil, nil): ok=%v, err=%v", ok, err)
	}

	// Started nil
	_, ok, err = MTTD(nil, &now)
	if err != nil || ok {
		t.Errorf("MTTD(nil, now): ok=%v, err=%v", ok, err)
	}

	// Detected nil
	_, ok, err = MTTD(&now, nil)
	if err != nil || ok {
		t.Errorf("MTTD(now, nil): ok=%v, err=%v", ok, err)
	}

	// Both set, positive duration
	d, ok, err := MTTD(&tenSecondsAgo, &now)
	if err != nil || !ok {
		t.Fatalf("MTTD(now-10s, now): ok=%v, err=%v", ok, err)
	}
	if d < 9*time.Second || d > 11*time.Second {
		t.Errorf("MTTD duration = %v, want ~10s", d)
	}

	// Inverted timestamps (detected before started)
	_, ok, err = MTTD(&now, &tenSecondsAgo)
	if err == nil {
		t.Error("MTTD(now, now-10s) should error (inverted)")
	}
	if ok {
		t.Error("MTTD(now, now-10s) ok should be false")
	}
}

// ---------------------------------------------------------------------------
// Package import boundary
// ---------------------------------------------------------------------------

func TestPackageImportBoundary(t *testing.T) {
	// This test exists to document the contract that this package does not
	// import store, http, or sql — it is verified by the compiler and by
	// go list, not at runtime.
	t.Log("scoring package is pure standard library — verified by go list")
}
