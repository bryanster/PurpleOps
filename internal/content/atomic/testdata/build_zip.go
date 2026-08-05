//go:build ignore

// One-shot helper to rebuild atomics-mini.zip from the sibling YAML fixtures.
// Not part of the package build; run:
//
//	go run build_zip.go
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	files := []string{
		"T1059.001.yaml",
		"T1059.004.yaml",
		"T1003.yaml",
	}
	outPath := "atomics-mini.zip"
	f, err := os.Create(outPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, name := range files {
		body, err := os.ReadFile(name)
		if err != nil {
			panic(err)
		}
		// GitHub archive layout: <repo>-<ref>/atomics/Txxxx/Txxxx.yaml
		w, err := zw.Create(filepath.ToSlash(filepath.Join(
			"atomic-red-team-master", "atomics",
			name[:len(name)-len(filepath.Ext(name))], name,
		)))
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(body); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	// touch so the ignore build of broken is unused
	_ = io.Discard
	fmt.Println("wrote", outPath)
}
