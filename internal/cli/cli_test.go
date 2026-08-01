package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// toolEnv is every variable popsctl reads (config.Config.bindings, the ones
// marked tool). The helper below empties all of them, so a developer's own
// exported PURPLEOPS_* cannot change what a test is testing — config treats an
// empty value as unset.
var toolEnv = []string{
	"PURPLEOPS_DB_PATH",
	"PURPLEOPS_LOG_LEVEL",
	"PURPLEOPS_LOG_FORMAT",
}

// result is one invocation of the CLI: what it printed, where, and what it
// would have exited with.
type result struct {
	stdout string
	stderr string
	code   int
}

// run invokes the command tree the binary runs, in this process. Nothing is
// spawned, so a failing test can be stepped through in a debugger and the
// coverage profile means something.
func run(t *testing.T, env map[string]string, args ...string) result {
	t.Helper()

	for _, name := range toolEnv {
		t.Setenv(name, env[name])
	}
	for name := range env {
		if !contains(toolEnv, name) {
			t.Fatalf("%s is not a variable popsctl reads; check the name", name)
		}
	}

	var stdout, stderr bytes.Buffer
	code := Main(context.Background(), args, &stdout, &stderr)
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// runIn is run against a database of the test's own, which is what almost every
// case wants.
func runIn(t *testing.T, dbPath string, args ...string) result {
	t.Helper()
	return run(t, nil, append(args, "--db", dbPath)...)
}

// tempDB returns a path in a fresh directory. The file does not exist yet: the
// commands create it, which is part of what is being tested.
func tempDB(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.duckdb")
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// decodeJSON asserts stdout is exactly one JSON document and returns it. "Valid
// JSON" is not enough on its own: a log line or a stray Println before the
// document is the failure this is really looking for, and json.Decoder finds it
// where json.Unmarshal of a prefix would not.
func decodeJSON(t *testing.T, out string) map[string]any {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(out))
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("stdout is not a JSON document: %v\n%s", err, out)
	}
	if rest, err := decoder.Token(); err == nil {
		t.Fatalf("stdout carries more than one document (%v after the first):\n%s", rest, out)
	}
	return document
}

// field reads one value out of a decoded document, failing the test if it is
// missing or of another type. Numbers are float64 and everything else is what
// encoding/json says it is.
func field[T any](t *testing.T, document map[string]any, key string) T {
	t.Helper()

	value, ok := document[key].(T)
	if !ok {
		t.Fatalf("%q is %#v, want a %T", key, document[key], value)
	}
	return value
}

// objects reads an array of JSON objects — a list of migrations, a list of
// tables — out of a decoded document.
func objects(t *testing.T, document map[string]any, key string) []map[string]any {
	t.Helper()

	raw := field[[]any](t, document, key)
	entries := make([]map[string]any, len(raw))
	for i, element := range raw {
		entry, ok := element.(map[string]any)
		if !ok {
			t.Fatalf("%s[%d] is %#v, want an object", key, i, element)
		}
		entries[i] = entry
	}
	return entries
}

// TestHelpListsEveryCommand is the acceptance criterion in test form, plus the
// half of it that a reader cannot check by eye: that no command anywhere in the
// tree was added without a description.
func TestHelpListsEveryCommand(t *testing.T) {
	got := run(t, nil, "--help")

	if got.code != ExitOK {
		t.Errorf("--help exited %d, want %d", got.code, ExitOK)
	}
	if got.stderr != "" {
		t.Errorf("--help wrote to stderr, which is for diagnostics:\n%s", got.stderr)
	}

	for _, cmd := range newRoot(&app{}).Commands() {
		if !strings.Contains(got.stdout, cmd.Name()) {
			t.Errorf("--help does not list %q:\n%s", cmd.Name(), got.stdout)
		}
		if !strings.Contains(got.stdout, cmd.Short) {
			t.Errorf("--help does not describe %q:\n%s", cmd.Name(), got.stdout)
		}
	}

	walk(newRoot(&app{}), func(cmd *cobra.Command) {
		if cmd.Short == "" {
			t.Errorf("%q has no one-line description", cmd.CommandPath())
		}
	})
}

// TestEveryCommandHasItsOwnHelp checks the promise --help makes: that following
// it to a subcommand gets you somewhere, rather than to a bare usage line.
func TestEveryCommandHasItsOwnHelp(t *testing.T) {
	walk(newRoot(&app{}), func(cmd *cobra.Command) {
		if cmd.Name() == "help" || strings.HasPrefix(cmd.CommandPath(), name+" completion") {
			return // cobra's own commands, documented by cobra
		}

		path := strings.Fields(cmd.CommandPath())[1:] // without "popsctl"
		got := run(t, nil, append(path, "--help")...)

		if got.code != ExitOK {
			t.Errorf("`%s --help` exited %d, want %d", cmd.CommandPath(), got.code, ExitOK)
		}
		if !strings.Contains(got.stdout, cmd.Long) {
			t.Errorf("`%s --help` does not print the command's own description:\n%s",
				cmd.CommandPath(), got.stdout)
		}
	})
}

// TestExitCodesDistinguishTheKindOfFailure is the contract a script depends on:
// a 2 will not be fixed by trying again.
func TestExitCodesDistinguishTheKindOfFailure(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"a command that worked", []string{"version"}, ExitOK},
		{"help", []string{"--help"}, ExitOK},
		{"no command at all", nil, ExitUsage},
		{"an unknown command", []string{"nonsense"}, ExitUsage},
		{"an unknown subcommand", []string{"migrate", "nonsense"}, ExitUsage},
		{"a command group without its subcommand", []string{"migrate"}, ExitUsage},
		{"an unknown flag", []string{"version", "--nonsense"}, ExitUsage},
		{"an unparseable flag value", []string{"version", "--log-level", "loud"}, ExitUsage},
		{"an argument where none is taken", []string{"version", "extra"}, ExitUsage},
		{"a command that ran and failed", []string{"backup"}, ExitFailure},
		{"a database that cannot be opened", []string{"db", "info", "--db", "/nope/x.duckdb"}, ExitFailure},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, nil, tc.args...)
			if got.code != tc.want {
				t.Errorf("`popsctl %s` exited %d, want %d\nstdout: %s\nstderr: %s",
					strings.Join(tc.args, " "), got.code, tc.want, got.stdout, got.stderr)
			}
		})
	}
}

// TestAUsageErrorPrintsUsageOnStderr keeps the two streams apart even when the
// command line was wrong: a shell pipeline that got a typo must not receive a
// usage block where it expected a result.
func TestAUsageErrorPrintsUsageOnStderr(t *testing.T) {
	got := run(t, nil, "migrate", "nonsense")

	if got.stdout != "" {
		t.Errorf("a usage error wrote to stdout:\n%s", got.stdout)
	}
	for _, want := range []string{`unknown subcommand "nonsense"`, "Usage:", "popsctl migrate"} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr does not mention %q:\n%s", want, got.stderr)
		}
	}
}

// TestVersionMatchesTheVersionFlag: `popsctl version` and `popsctl --version`
// are the same answer. The end-to-end harness seeds with one of them and the
// container smoke test uses the other.
func TestVersionMatchesTheVersionFlag(t *testing.T) {
	subcommand := run(t, nil, "version")
	flag := run(t, nil, "--version")

	if subcommand.code != ExitOK || flag.code != ExitOK {
		t.Fatalf("version exited %d, --version exited %d, want 0", subcommand.code, flag.code)
	}
	if subcommand.stdout != flag.stdout {
		t.Errorf("`version` printed %q and `--version` printed %q; they must agree",
			subcommand.stdout, flag.stdout)
	}
}

// TestJSONIsParseableForEveryCommandThatSupportsIt is the acceptance criterion
// for `popsctl db info --json | jq`, for every command that can be piped into
// it. A command added later without a JSON form fails this list, which is the
// point of listing them here rather than discovering them by reflection.
func TestJSONIsParseableForEveryCommandThatSupportsIt(t *testing.T) {
	tests := [][]string{
		{"version"},
		{"migrate", "status"},
		{"migrate", "up"},
		{"db", "info"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			// A database of its own per command, so `migrate up` running here
			// cannot change what another case sees.
			got := runIn(t, tempDB(t), append(args, "--json")...)
			if got.code != ExitOK {
				t.Fatalf("exited %d, want 0\nstderr: %s", got.code, got.stderr)
			}
			decodeJSON(t, got.stdout)
		})
	}
}

// TestLogsStayOnStderrInJSONMode is the other half of the pipe working: the
// migrator narrates what it is doing, and none of it may land in the document.
func TestLogsStayOnStderrInJSONMode(t *testing.T) {
	got := runIn(t, tempDB(t), "migrate", "up", "--json", "--log-level", "debug")

	if got.code != ExitOK {
		t.Fatalf("exited %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	decodeJSON(t, got.stdout)
	if !strings.Contains(got.stderr, "applied migration") {
		t.Errorf("the migrator's progress did not reach stderr:\n%s", got.stderr)
	}
}

// TestTheDatabaseFlagWinsOverTheEnvironment: --db is how the end-to-end harness
// and an operator with two deployments say which database they mean.
func TestTheDatabaseFlagWinsOverTheEnvironment(t *testing.T) {
	fromEnv, fromFlag := tempDB(t), tempDB(t)

	got := run(t, map[string]string{"PURPLEOPS_DB_PATH": fromEnv},
		"db", "info", "--db", fromFlag, "--json")
	if got.code != ExitOK {
		t.Fatalf("exited %d, want 0\nstderr: %s", got.code, got.stderr)
	}

	if path := field[string](t, decodeJSON(t, got.stdout), "path"); path != fromFlag {
		t.Errorf("worked on %q, want the --db path %s", path, fromFlag)
	}
	if _, err := os.Stat(fromEnv); err == nil {
		t.Errorf("%s was created; the environment's database must not be touched", fromEnv)
	}
}

// TestTheEnvironmentIsUsedWhenTheFlagIsAbsent is the same rule from the other
// side: one codebase, two entrypoints, one set of variables (PLAN.md §6).
func TestTheEnvironmentIsUsedWhenTheFlagIsAbsent(t *testing.T) {
	db := tempDB(t)

	got := run(t, map[string]string{"PURPLEOPS_DB_PATH": db}, "db", "info", "--json")
	if got.code != ExitOK {
		t.Fatalf("exited %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	if path := field[string](t, decodeJSON(t, got.stdout), "path"); path != db {
		t.Errorf("worked on %q, want %s from the environment", path, db)
	}
}

// TestABadEnvironmentIsReportedNotIgnored: a mistyped variable is a failure to
// run (exit 1), and names the variable, because that is the only way the
// operator finds it.
func TestABadEnvironmentIsReportedNotIgnored(t *testing.T) {
	got := run(t, map[string]string{"PURPLEOPS_LOG_LEVEL": "loud"}, "db", "info")

	if got.code != ExitFailure {
		t.Errorf("exited %d, want %d", got.code, ExitFailure)
	}
	if !strings.Contains(got.stderr, "PURPLEOPS_LOG_LEVEL") {
		t.Errorf("the error does not name the variable:\n%s", got.stderr)
	}
}

// walk calls fn for cmd and every command below it.
func walk(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, sub := range cmd.Commands() {
		walk(sub, fn)
	}
}
