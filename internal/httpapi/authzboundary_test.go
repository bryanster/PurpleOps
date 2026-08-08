package httpapi

// The boundary that makes "zero handler checks" a property of the build rather
// than a habit.
//
// PLAN.md §4 records that v1 decided per handler, in forty-odd of them, with two
// definitions of "blue" that disagreed. Moving the decision into one middleware
// fixes today's handlers; what stops tomorrow's from growing their own is that a
// handler cannot reach the vocabulary a role decision is written in. These tests
// fail the build if one does.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const authzPackage = "github.com/bryanster/blacklight/internal/authz"

// TestNoHandlerDecidesForItself is M1-013's import-boundary criterion.
//
// It finds the handlers by looking for methods on the type that implements the
// generated server interface, rather than by a filename convention — so moving a
// handler into a new file does not move it out of the test's reach, which is
// exactly what a filename convention would have allowed. Imports are per file in
// Go, so this is checkable at the granularity that matters even though the
// middleware and the handlers share a package.
func TestNoHandlerDecidesForItself(t *testing.T) {
	t.Parallel()

	files := parsePackage(t, ".")

	var handlerFiles []string
	for path, file := range files {
		if declaresHandler(file) {
			handlerFiles = append(handlerFiles, path)
		}
	}
	slices.Sort(handlerFiles)

	// A rename that emptied this list would leave the test passing about
	// nothing, which is the failure mode of every convention test.
	if len(handlerFiles) == 0 {
		t.Fatal("no file in this package declares a method on *handlers; this test has stopped checking anything. " +
			"If the type has been renamed, rename it here too")
	}

	for _, path := range handlerFiles {
		for _, imported := range imports(files[path]) {
			if imported == authzPackage {
				t.Errorf("%s implements handlers and imports %s. A handler that can name a role is a handler that "+
					"can decide with one; the decision belongs to the middleware in authorize.go, which is given "+
					"the request before any of these functions are (PLAN.md §4)", path, imported)
			}
		}
	}
	t.Logf("checked %d handler file(s): %s", len(handlerFiles), strings.Join(handlerFiles, ", "))

	// The middleware itself must import it, or the test above is satisfied by a
	// build in which nobody decides anything at all.
	if file, ok := files["authorize.go"]; !ok || !slices.Contains(imports(file), authzPackage) {
		t.Error("authorize.go does not import internal/authz, so nothing in this package asks the policy anything")
	}
	if _, decides := files["authorize.go"]; decides && declaresHandler(files["authorize.go"]) {
		t.Error("authorize.go declares a handler method as well as the middleware; keep the two apart, so that the " +
			"file allowed to import internal/authz is the one that does not serve a request's response")
	}
}

// TestOnlyOneFunctionInTheRepositoryAsksThePolicy is the same rule stated over
// the whole tree rather than one package: [authz.Can] has exactly one non-test
// caller, and it is the middleware.
//
// M1-012 made the policy a single function. This is what keeps it a single *call
// site* — a second one is a second place where a resource is assembled and an
// answer is interpreted, and the ways those two can differ are precisely v1's
// bugs.
func TestOnlyOneFunctionInTheRepositoryAsksThePolicy(t *testing.T) {
	t.Parallel()

	// Keyed by path, valued with why that file is allowed to ask.
	allowed := map[string]string{
		"internal/httpapi/authorize.go": "the one middleware, which is the point of M1-013",
	}

	root := filepath.Join("..", "..")
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir():
			return skipVendoredDir(root, path)
		case !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
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
		relative = filepath.ToSlash(relative)
		if _, ok := allowed[relative]; ok {
			return nil
		}
		if position, asks := callsCan(fset, file); asks {
			t.Errorf("%s calls authz.Can. There is one place that decides what a caller may do, and it is the "+
				"middleware in internal/httpapi/authorize.go — a second call site is a second place that builds a "+
				"resource and interprets an answer", position)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
}

// callsCan reports whether a file contains a call to authz.Can, and where.
func callsCan(fset *token.FileSet, file *ast.File) (string, bool) {
	position, found := "", false

	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Can" {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != "authz" {
			return true
		}
		position, found = fset.Position(call.Pos()).String(), true
		return false
	})
	return position, found
}

// parsePackage parses the non-test Go files of one directory, keyed by base
// name.
func parsePackage(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files[name] = file
	}
	return files
}

// declaresHandler reports whether a file declares a method on *handlers — which
// is to say, whether it is part of the implementation of the generated server
// interface.
func declaresHandler(file *ast.File) bool {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || len(function.Recv.List) != 1 {
			continue
		}
		star, ok := function.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		if name, ok := star.X.(*ast.Ident); ok && name.Name == "handlers" {
			return true
		}
	}
	return false
}

// imports returns the import paths of one file.
func imports(file *ast.File) []string {
	paths := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

// skipVendoredDir keeps the walk out of trees nobody here wrote or committed. It
// is the same list internal/authz's boundary test uses, for the same reason.
func skipVendoredDir(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}

	switch filepath.ToSlash(rel) {
	case ".git", "bin", "web/node_modules", "web/dist", "e2e/node_modules", "e2e/test-results":
		return fs.SkipDir
	}
	return nil
}
