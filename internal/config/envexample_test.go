package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestEveryConfigFieldIsBound walks Config by reflection and fails on any field
// that no environment variable fills. Without it, adding a field is a silent
// way to ship a value that is always the zero value.
func TestEveryConfigFieldIsBound(t *testing.T) {
	var cfg Config

	bound := make(map[any]string)
	for _, b := range cfg.bindings() {
		bound[b.target] = b.name
	}

	seen := make(map[string]bool)
	var walk func(v reflect.Value, path string)
	walk = func(v reflect.Value, path string) {
		typ := v.Type()
		for i := range typ.NumField() {
			field := typ.Field(i)
			if !field.IsExported() {
				continue // not reachable by a consumer, so not configurable
			}
			name := path + "." + field.Name
			if envName, ok := bound[v.Field(i).Addr().Interface()]; ok {
				seen[envName] = true
				continue
			}
			if field.Type.Kind() == reflect.Struct {
				walk(v.Field(i), name)
				continue
			}
			t.Errorf("Config%s is filled by nothing: add it to Config.bindings and to .env.example", name)
		}
	}
	walk(reflect.ValueOf(&cfg).Elem(), "")

	for _, b := range cfg.bindings() {
		if !seen[b.name] {
			t.Errorf("binding %s does not point at a field of Config", b.name)
		}
	}
}

// TestEnvExampleDocumentsEveryVariable ties .env.example to the code in both
// directions, and checks it is still usable as a template: an operator copies
// it, changes the secret, and gets the documented defaults.
func TestEnvExampleDocumentsEveryVariable(t *testing.T) {
	byName := make(map[string]assignment)
	for _, a := range readEnvExample(t) {
		if prev, dup := byName[a.name]; dup {
			t.Errorf(".env.example:%d: %s is assigned again (first at line %d)", a.line, a.name, prev.line)
		}
		byName[a.name] = a
	}

	var cfg Config
	for _, b := range cfg.bindings() {
		a, ok := byName[b.name]
		if !ok {
			t.Errorf("%s is read by the server but is not in .env.example", b.name)
			continue
		}
		delete(byName, b.name)

		if !a.documented {
			t.Errorf(".env.example:%d: %s has no comment above it explaining what it does", a.line, a.name)
		}
		switch {
		case b.required:
			if a.commented {
				t.Errorf(".env.example:%d: %s is required, so it must be a live line with a "+
					"placeholder rather than a commented-out one", a.line, a.name)
			}
		case b.def != "":
			if a.commented {
				t.Errorf(".env.example:%d: %s has default %q, so the line must be live and show it",
					a.line, a.name, b.def)
			}
			if a.value != b.def {
				t.Errorf(".env.example:%d: %s documents %q but the default is %q",
					a.line, a.name, a.value, b.def)
			}
		default:
			if !a.commented {
				t.Errorf(".env.example:%d: %s has no default, so an untouched copy of this file "+
					"would change behaviour: comment the line out", a.line, a.name)
			}
		}
	}

	for name, a := range byName {
		t.Errorf(".env.example:%d: %s is documented but the server never reads it", a.line, name)
	}
}

// TestEnvExampleIsRejectedOnlyForItsPlaceholderSecrets proves two things at
// once: an untouched copy of the template cannot start a server (both shipped
// secrets are refused), and nothing else in the template is invalid.
func TestEnvExampleIsRejectedOnlyForItsPlaceholderSecrets(t *testing.T) {
	env := make(map[string]string)
	for _, a := range readEnvExample(t) {
		if !a.commented {
			env[a.name] = a.value
		}
	}

	// parse, not Load: the template's paths are relative, and a test must not
	// create directories in the source tree.
	_, errs := parse(env)

	rejected := make(map[string]bool, len(errs))
	for _, err := range errs {
		var fieldErr *FieldError
		if !errors.As(err, &fieldErr) {
			t.Fatalf("parse(.env.example) = %v, want a *FieldError", err)
		}
		rejected[fieldErr.Name] = true
	}

	for _, name := range []string{envSessionSecret, envEncryptionKey} {
		if !rejected[name] {
			t.Errorf("parse(.env.example) accepted the shipped %s; an untouched copy of the "+
				"template must not be able to start a server", name)
		}
		delete(rejected, name)
	}
	for name := range rejected {
		t.Errorf("parse(.env.example) objected to %s, which the template is supposed to get right",
			name)
	}
}

// assignment is one `NAME=value` line of .env.example.
type assignment struct {
	name  string
	value string
	line  int
	// commented is true for a `#NAME=value` line: documented, not applied.
	commented bool
	// documented is true when the line above is prose explaining the variable.
	documented bool
}

// assignmentPattern matches a live or commented-out assignment. The variable
// name has to follow the "#" immediately for a commented assignment, so prose
// that happens to mention `PURPLEOPS_ENV=production` is not mistaken for one.
var assignmentPattern = regexp.MustCompile(`^(#?)(` + prefix + `[A-Z0-9_]+)=(.*)$`)

func readEnvExample(t *testing.T) []assignment {
	t.Helper()

	path := filepath.Join(repoRoot(t), ".env.example")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("closing %s: %v", path, err)
		}
	}()

	var (
		assignments []assignment
		prose       bool // the previous line was a comment that was not a divider
		line        int
	)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := scanner.Text()

		if m := assignmentPattern.FindStringSubmatch(text); m != nil {
			assignments = append(assignments, assignment{
				name:       m[2],
				value:      strings.TrimSpace(m[3]),
				line:       line,
				commented:  m[1] == "#",
				documented: prose,
			})
			prose = false // each variable needs its own comment, not its neighbour's
			continue
		}
		// A section divider ("# --- Core ---") documents nothing on its own.
		prose = strings.HasPrefix(text, "#") && !strings.HasPrefix(text, "# ---")
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(assignments) == 0 {
		t.Fatalf("%s contains no %s assignments; the parser or the file is wrong", path, prefix)
	}
	return assignments
}

// repoRoot returns the module root, derived from this file's own path so the
// test does not depend on the directory it is invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the repository root")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
