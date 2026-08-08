//go:build ignore

// One-shot helper to rebuild rules-mini.zip from the sibling YAML fixtures.
// Not part of the package build; run:
//
//	go run build_zip.go
package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	entries := []struct {
		src string
		dst string
	}{
		{
			src: "rules/windows/process_creation/proc_creation_win_powershell_encoded.yml",
			dst: "sigma-master/rules/windows/process_creation/proc_creation_win_powershell_encoded.yml",
		},
		{
			src: "rules/linux/process_creation/proc_creation_lnx_bash_reverse.yml",
			dst: "sigma-master/rules/linux/process_creation/proc_creation_lnx_bash_reverse.yml",
		},
		{
			src: "rules/windows/process_creation/proc_creation_win_unmapped.yml",
			dst: "sigma-master/rules/windows/process_creation/proc_creation_win_unmapped.yml",
		},
	}
	outPath := "rules-mini.zip"
	f, err := os.Create(outPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, e := range entries {
		body, err := os.ReadFile(e.src)
		if err != nil {
			panic(err)
		}
		w, err := zw.Create(filepath.ToSlash(e.dst))
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
	fmt.Println("wrote", outPath)
}
