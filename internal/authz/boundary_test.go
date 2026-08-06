package authz_test

// The two boundaries that keep this package the only place a role is decided.
//
// Both are automated for the same reason: they are conventions, and a
// convention that only a reviewer enforces is a convention that survives until
// the first busy week.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
)

const packagePath = "github.com/bryanster/blacklight/internal/authz"

// allowedImports is everything this package may import directly. Adding to it
// is a design decision, so say why in one line — the constraint is not "no
// imports", it is that nothing here can reach a database, a socket or a clock,
// because [authz.Can] must stay a pure function of its arguments (M1-012).
var allowedImports = map[string]string{
	"context": "Can takes one for signature stability; it reads nothing from it",
	"fmt":     "formatting the reason on a decision",
	"log/slog": "rendering a decision into a log line, in the caller's logger — " +
		"authz opens no sink of its own",
	"slices": "cloning the role slices Rules() hands out",
}

// TestAuthzImportsNothingThatCouldMakeItImpure. M1-012's first acceptance
// criterion: "authz imports no database, HTTP, or store package."
//
// Checked two ways, because the direct list and the transitive graph fail
// differently. A direct import of net/http is somebody adding one; a *module*
// dependency is somebody importing a helper that turns out to drag the store in
// behind it, which is the one that happens by accident.
func TestAuthzImportsNothingThatCouldMakeItImpure(t *testing.T) {
	for _, imported := range golist(t, "-f", "{{join .Imports \"\\n\"}}", packagePath) {
		if _, allowed := allowedImports[imported]; !allowed {
			t.Errorf("internal/authz imports %q. If it belongs here, add it to allowedImports with a reason; "+
				"if it is a database, a transport or a clock, it does not belong here", imported)
		}
	}

	for _, dependency := range golist(t, "-deps", packagePath) {
		if dependency != packagePath && strings.HasPrefix(dependency, "github.com/bryanster/blacklight/") {
			t.Errorf("internal/authz depends on %s. This package must depend on nothing else in the "+
				"repository, so that no rule can ever reach the store, the HTTP layer or the config",
				dependency)
		}
	}
}

// golist runs `go list` and returns its non-empty lines. A subprocess rather
// than go/build, because the question is about the real dependency graph the
// toolchain resolves and not about what the import statements look like.
func golist(t *testing.T, args ...string) []string {
	t.Helper()

	out, err := exec.Command("go", append([]string{"list"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("go list %s: %v\n%s", strings.Join(args, " "), err, out)
	}

	var lines []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// roleLiterals is every string this package defines a role as. A build with two
// spellings of one of these is v1: PLAN.md §4's "two contradictory definitions
// of blue" was exactly that, in two handlers, and the gap between them was
// write access.
var roleLiterals = func() map[string]string {
	literals := map[string]string{}
	for _, role := range authz.PlatformRoles() {
		literals[string(role)] = "platform role"
	}
	for _, role := range authz.EngagementRoles() {
		literals[string(role)] = "engagement role"
	}
	return literals
}()

// justifiedLiterals are the string literals that collide with a role name and
// mean something else entirely. Keyed by "<path>:<literal>".
//
// Every entry is a claim that the string is not a role. Adding one is fine;
// adding one without reading it is how the rule stops meaning anything.
var justifiedLiterals = map[string]string{
	"internal/cli/user.go:admin":                               "the name of the --admin flag, not the role it sets",
	"internal/store/engagement/engagement.go:red":             "EvidenceSide value, not an engagement role",
	"internal/store/engagement/engagement.go:blue":            "EvidenceSide value, not an engagement role",
}

// TestRoleLiteralsLiveOnlyInThisPackage is M1-012's fourth acceptance criterion.
//
// It skips test files and generated code. A test asserting `{"platformRole":
// "admin"}` is asserting the *wire* contract, which api/openapi.yaml owns and
// TestSpecRoleEnumsMatchAuthz holds to these same values; generated code is the
// spec's output and cannot be edited anyway. What is checked is the hand-written
// implementation, where a second definition would be a decision somebody made.
func TestRoleLiteralsLiveOnlyInThisPackage(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir():
			return skipDir(root, path)
		case !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		if ast.IsGenerated(file) {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		reportRoleLiterals(t, fset, file, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
}

// skipDir keeps the walk out of the one package that is allowed these strings,
// and out of trees nobody here wrote or committed.
func skipDir(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}

	switch filepath.ToSlash(rel) {
	case "internal/authz", ".git", "bin", "web/node_modules", "web/dist", "e2e/node_modules", "e2e/test-results":
		return fs.SkipDir
	}
	return nil
}

// reportRoleLiterals fails the test once per unjustified role string in one
// file.
func reportRoleLiterals(t *testing.T, fset *token.FileSet, file *ast.File, path string) {
	t.Helper()

	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}

		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}

		kind, isRole := roleLiterals[value]
		if !isRole {
			return true
		}
		if _, justified := justifiedLiterals[path+":"+value]; justified {
			return true
		}

		t.Errorf("%s: %q is the %s and must come from internal/authz, not from a literal. "+
			"If this string means something else, add %q to justifiedLiterals with the reason",
			fset.Position(literal.Pos()), value, kind, path+":"+value)
		return true
	})
}
