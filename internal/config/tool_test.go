package config

import (
	"reflect"
	"strings"
	"testing"
)

// TestToolReadsWhatItNeedsAndNothingElse is the reason LoadTool exists: an
// admin command must run on a machine where nobody has set a base URL or a
// session secret, because it serves no HTTP and holds no sessions.
func TestToolReadsWhatItNeedsAndNothingElse(t *testing.T) {
	// Deliberately hostile: the two variables a server requires are absent,
	// and one that is present is invalid.
	cfg, errs := parseTool(map[string]string{
		envDBPath: "/srv/purpleops/purpleops.duckdb",
		envAddr:   "not a listen address",
	})
	if len(errs) > 0 {
		t.Fatalf("parseTool = %v, want no errors", errs)
	}

	if got, want := cfg.Database.Path, "/srv/purpleops/purpleops.duckdb"; got != want {
		t.Errorf("Database.Path = %q, want %q", got, want)
	}
	if got, want := cfg.Log.Level, LevelInfo; got != want {
		t.Errorf("Log.Level = %q, want the documented default %q", got, want)
	}
	if got, want := cfg.Log.Format, FormatJSON; got != want {
		t.Errorf("Log.Format = %q, want the documented default %q", got, want)
	}
}

// TestToolStillRejectsWhatItDoesRead: reading less is not reading carelessly.
func TestToolStillRejectsWhatItDoesRead(t *testing.T) {
	_, errs := parseTool(map[string]string{envLogLevel: "loud"})

	if len(errs) != 1 {
		t.Fatalf("parseTool reported %d problems, want exactly the log level: %v", len(errs), errs)
	}
	if got := errs[0].Error(); !strings.Contains(got, envLogLevel) {
		t.Errorf("the error does not name the variable: %s", got)
	}
}

// TestEveryToolFieldIsBoundToAToolVariable ties [Tool] to the bindings marked
// tool, in both directions. Without it, adding a section to Tool would ship a
// field that is always the zero value, and marking a binding tool without
// putting its field in Tool would be a variable read and discarded.
func TestEveryToolFieldIsBoundToAToolVariable(t *testing.T) {
	var cfg Config

	// The bindings marked tool, by the address of the field they fill.
	toolBound := make(map[any]string)
	for _, b := range cfg.bindings() {
		if b.tool {
			toolBound[b.target] = b.name
		}
	}

	// Every field of Tool must be a section of Config filled entirely by
	// tool-marked bindings.
	seen := make(map[string]bool)
	toolType := reflect.TypeOf(Tool{})
	configValue := reflect.ValueOf(&cfg).Elem()
	for i := range toolType.NumField() {
		field := toolType.Field(i)

		section := configValue.FieldByName(field.Name)
		if !section.IsValid() {
			t.Errorf("Tool.%s has no counterpart in Config; the two must stay the same shape",
				field.Name)
			continue
		}
		if section.Type() != field.Type {
			t.Errorf("Tool.%s is %s and Config.%s is %s", field.Name, field.Type,
				field.Name, section.Type())
			continue
		}

		for j := range section.NumField() {
			name, ok := toolBound[section.Field(j).Addr().Interface()]
			if !ok {
				t.Errorf("Tool.%s.%s is filled by no variable marked tool, so it would "+
					"always be the zero value", field.Name, section.Type().Field(j).Name)
				continue
			}
			seen[name] = true
		}
	}

	for _, name := range toolBound {
		if !seen[name] {
			t.Errorf("%s is marked tool but fills no field of Tool: either add the section "+
				"to Tool or drop the mark", name)
		}
	}
}
