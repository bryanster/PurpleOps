package report

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Registry: duplicate rejection
// ---------------------------------------------------------------------------

func TestRegistryDuplicatePanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()

	reg := NewRegistry()
	reg.Register(Definition{ID: "test", Title: "first"})
	reg.Register(Definition{ID: "test", Title: "second"})
}

// ---------------------------------------------------------------------------
// Registry: Get / MustGet
// ---------------------------------------------------------------------------

func TestRegistryGet(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.Register(Definition{ID: "a", Title: "Block A"})

	def, ok := reg.Get("a")
	if !ok {
		t.Fatal("Get returned false for registered block")
	}
	if def.Title != "Block A" {
		t.Fatalf("Title = %q, want %q", def.Title, "Block A")
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Fatal("Get returned true for unregistered block")
	}
}

func TestRegistryMustGetPanicsOnMissing(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.Register(Definition{ID: "a", Title: "Block A"})

	// MustGet on existing block does not panic.
	_ = reg.MustGet("a")

	// MustGet on missing block panics.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on MustGet of missing block")
		}
	}()
	_ = reg.MustGet("nonexistent")
}

// ---------------------------------------------------------------------------
// Registry: List — stable catalogue order
// ---------------------------------------------------------------------------

func TestRegistryListStableOrder(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	// Register in reverse catalogue order.
	allIDs := AllBlockIDs()
	for i := len(allIDs) - 1; i >= 0; i-- {
		reg.Register(Definition{ID: allIDs[i], Title: string(allIDs[i])})
	}

	list := reg.List()
	if len(list) != len(allIDs) {
		t.Fatalf("List() returned %d blocks, want %d", len(list), len(allIDs))
	}
	for i, def := range list {
		if def.ID != allIDs[i] {
			t.Fatalf("List()[%d].ID = %q, want %q (catalogue order)", i, def.ID, allIDs[i])
		}
	}
}

func TestRegistryListExtraAlphabetical(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.Register(Definition{ID: IDCover, Title: "Cover"})
	reg.Register(Definition{ID: "z_custom", Title: "Z Custom"})
	reg.Register(Definition{ID: "a_custom", Title: "A Custom"})

	list := reg.List()

	// First must be IDCover (catalogue order).
	if len(list) < 1 || list[0].ID != IDCover {
		t.Fatalf("first block = %q, want %q", list[0].ID, IDCover)
	}

	// Remaining must be alphabetical.
	if len(list) != 3 {
		t.Fatalf("List() returned %d blocks, want 3", len(list))
	}
	if list[1].ID != "a_custom" {
		t.Fatalf("list[1].ID = %q, want a_custom", list[1].ID)
	}
	if list[2].ID != "z_custom" {
		t.Fatalf("list[2].ID = %q, want z_custom", list[2].ID)
	}
}

// ---------------------------------------------------------------------------
// ValidateParams: table-driven
// ---------------------------------------------------------------------------

func TestValidateParams(t *testing.T) {
	t.Parallel()

	schema := ParamSchema{
		"title":                {Type: "string", Description: "Title text"},
		"count":                {Type: "number", Default: json.RawMessage(`10`)},
		"show":                 {Type: "boolean", Default: json.RawMessage(`true`)},
		"baselineEngagementId": {Type: "string", Description: "Baseline engagement UUID"},
		"tactic":               {Type: "string", Enum: []string{"recon", "execution", "persistence"}},
		"limit":                {Type: "integer", Description: "Row cap"},
	}

	tests := []struct {
		name    string
		params  json.RawMessage
		want    json.RawMessage // expected output, or empty if error expected
		wantErr string          // substring of error message
	}{
		{
			name:   "valid with all keys",
			params: json.RawMessage(`{"title":"Hello","count":5,"show":false,"baselineEngagementId":"01932c0e-0000-7000-0000-000000000001","tactic":"execution"}`),
			want:   json.RawMessage(`{"baselineEngagementId":"01932c0e-0000-7000-0000-000000000001","count":5,"show":false,"tactic":"execution","title":"Hello"}`),
		},
		{
			name:   "defaults applied for omitted keys",
			params: json.RawMessage(`{"title":"Hello","baselineEngagementId":"01932c0e-0000-7000-0000-000000000001"}`),
			want:   json.RawMessage(`{"baselineEngagementId":"01932c0e-0000-7000-0000-000000000001","count":10,"show":true,"title":"Hello"}`),
		},
		{
			name:   "empty params with full defaults",
			params: json.RawMessage(`{}`),
			want:   json.RawMessage(`{"count":10,"show":true}`),
		},
		{
			name:   "nil params with defaults",
			params: nil,
			want:   json.RawMessage(`{"count":10,"show":true}`),
		},
		{
			name:    "unknown key rejected",
			params:  json.RawMessage(`{"title":"Hello","unknown_field":42}`),
			wantErr: `unknown param "unknown_field"`,
		},
		{
			name:    "wrong type rejected",
			params:  json.RawMessage(`{"title":42}`),
			wantErr: `param "title" must be a string`,
		},
		{
			name:    "enum constraint enforced",
			params:  json.RawMessage(`{"tactic":"unknown_tactic","title":"Hello"}`),
			wantErr: `param "tactic" must be one of`,
		},
		{
			name:   "enum valid value accepted",
			params: json.RawMessage(`{"tactic":"recon","title":"Hello"}`),
			want:   json.RawMessage(`{"count":10,"show":true,"tactic":"recon","title":"Hello"}`),
		},
		{
			// evidence_appendix declares "limit" as an integer and defaults it
			// to 50, so this is the shape the builder sends back on every save
			// after the first.
			name:   "integer accepted",
			params: json.RawMessage(`{"limit":50}`),
			want:   json.RawMessage(`{"count":10,"limit":50,"show":true}`),
		},
		{
			name:    "fractional integer rejected",
			params:  json.RawMessage(`{"limit":1.5}`),
			wantErr: `param "limit" must be a whole number`,
		},
		{
			name:    "non-numeric integer rejected",
			params:  json.RawMessage(`{"limit":"50"}`),
			wantErr: `param "limit" must be an integer`,
		},
		{
			name: "nil schema accepts anything (empty)",
			// No schema registered — ValidateParams should accept empty.
			params: json.RawMessage(`{}`),
			want:   json.RawMessage(`{}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s ParamSchema
			if tt.name != "nil schema accepts anything (empty)" {
				s = schema
			}

			got, err := ValidateParams(s, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Normalize both to map for order-independent comparison.
			var gotMap, wantMap map[string]json.RawMessage
			if err := json.Unmarshal(got, &gotMap); err != nil {
				t.Fatalf("unmarshal got: %v", err)
			}
			if err := json.Unmarshal(tt.want, &wantMap); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}

			if !mapsEqual(gotMap, wantMap) {
				t.Fatalf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registry completeness: every catalogue ID is registered
// ---------------------------------------------------------------------------

func TestRegistryCompleteness(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	for _, id := range AllBlockIDs() {
		reg.Register(Definition{ID: id, Title: string(id)})
	}

	for _, id := range AllBlockIDs() {
		if _, ok := reg.Get(id); !ok {
			t.Fatalf("catalogue block %q not found in registry", id)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidateParams: non-nil schema with nil params
// ---------------------------------------------------------------------------

func TestValidateParamsStrictSchemaNilParams(t *testing.T) {
	t.Parallel()

	schema := ParamSchema{
		"name": {Type: "string"},
	}

	// Nil params with a schema that has no defaults — name is simply absent.
	got, err := ValidateParams(schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var gotMap map[string]json.RawMessage
	if err := json.Unmarshal(got, &gotMap); err != nil {
		t.Fatal(err)
	}
	if len(gotMap) != 0 {
		t.Fatalf("expected empty map, got %d keys", len(gotMap))
	}
}

// ---------------------------------------------------------------------------
// Instance struct round-trip
// ---------------------------------------------------------------------------

func TestInstanceJSONRoundTrip(t *testing.T) {
	t.Parallel()

	inst := Instance{
		InstanceID: "inst-1",
		BlockID:    IDCover,
		Ordinal:    0,
		Params:     json.RawMessage(`{"title":"My Report"}`),
	}

	b, err := json.Marshal(inst)
	if err != nil {
		t.Fatal(err)
	}

	var got Instance
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.InstanceID != inst.InstanceID {
		t.Fatalf("InstanceID = %q, want %q", got.InstanceID, inst.InstanceID)
	}
	if got.BlockID != inst.BlockID {
		t.Fatalf("BlockID = %q, want %q", got.BlockID, inst.BlockID)
	}
	if got.Ordinal != inst.Ordinal {
		t.Fatalf("Ordinal = %d, want %d", got.Ordinal, inst.Ordinal)
	}
}

// ---------------------------------------------------------------------------
// Definition struct completeness
// ---------------------------------------------------------------------------

func TestDefinitionFields(t *testing.T) {
	t.Parallel()

	paramsJSON := json.RawMessage(`{"title":"Fallback"}`)

	def := Definition{
		ID:          IDEngagementCompare,
		Title:       "Engagement Comparison",
		Description: "Compare two engagements side by side.",
		ParamsSchema: ParamSchema{
			"baselineEngagementId": {Type: "string", Description: "Baseline engagement UUID"},
		},
		DefaultParams:      paramsJSON,
		DataDeps:           []DataDep{"compare"},
		AllowInTemplate:    true,
		NeedsEvidenceOptIn: false,
	}

	if def.ID != IDEngagementCompare {
		t.Fatalf("ID = %q", def.ID)
	}
	if def.Title != "Engagement Comparison" {
		t.Fatalf("Title = %q", def.Title)
	}
	if def.Description != "Compare two engagements side by side." {
		t.Fatalf("Description = %q", def.Description)
	}
	if def.ParamsSchema == nil {
		t.Fatal("ParamsSchema is nil")
	}
	if string(def.DefaultParams) != `{"title":"Fallback"}` {
		t.Fatalf("DefaultParams = %s", def.DefaultParams)
	}
	if len(def.DataDeps) != 1 || def.DataDeps[0] != "compare" {
		t.Fatalf("DataDeps = %v", def.DataDeps)
	}
	if !def.AllowInTemplate {
		t.Fatal("AllowInTemplate should be true")
	}
	if def.NeedsEvidenceOptIn {
		t.Fatal("NeedsEvidenceOptIn should be false")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func mapsEqual(a, b map[string]json.RawMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if string(av) != string(bv) {
			return false
		}
	}
	return true
}

// Ensure we don't leak unused imports.
var _ = fmt.Sprintf
