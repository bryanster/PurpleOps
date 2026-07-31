package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// environmentReaders are the ways a package can reach the process environment
// behind this package's back. os.Setenv and friends are deliberately absent:
// writing to the environment is what t.Setenv does.
var environmentReaders = map[string]bool{
	"Getenv":    true,
	"LookupEnv": true,
	"Environ":   true,
	"ExpandEnv": true,
}

// skippedDirs are not Go source we own.
var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"bin":          true,
}

// TestOnlyConfigReadsTheEnvironment is the automated form of the rule this
// package exists to enforce: a setting read at its point of use is a setting
// that is never validated, never documented, and different in every process
// that reads it. Everything goes through Config instead.
func TestOnlyConfigReadsTheEnvironment(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	scanned := 0

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skippedDirs[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// This package is the one place allowed to read the environment.
		if strings.HasPrefix(rel, filepath.Join("internal", "config")+string(filepath.Separator)) {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("parsing %s: %v", rel, err)
			return nil
		}
		scanned++

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "os" || !environmentReaders[selector.Sel.Name] {
				return true
			}
			t.Errorf("%s:%d: os.%s outside internal/config — add the setting to config.Config "+
				"and take it as a constructor argument instead",
				rel, fset.Position(selector.Pos()).Line, selector.Sel.Name)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if scanned == 0 {
		t.Fatalf("no Go files were scanned under %s; the walk is broken, not the tree", root)
	}
}
