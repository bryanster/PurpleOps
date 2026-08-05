//go:build ignore

// One-shot helper to rebuild plans-mini.zip from the sibling YAML fixtures.
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
	// Layout mirrors the GitHub archive of the CTID library:
	// adversary_emulation_library-master/{actor}/Emulation_Plan/yaml/{Plan}.yaml
	entries := []struct {
		src, dst string
	}{
		{
			src: "mini-plan.yaml",
			dst: "adversary_emulation_library-master/fixture_eagle/Emulation_Plan/yaml/Fixture_Eagle.yaml",
		},
	}
	outPath := "plans-mini.zip"
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
	// Noise the parser must skip.
	skip := map[string]string{
		"adversary_emulation_library-master/fixture_eagle/Emulation_Plan/yaml/README.md":      "# ignore",
		"adversary_emulation_library-master/fixture_eagle/Emulation_Plan/yaml/planners/x.yml": "id: planner\n",
		"adversary_emulation_library-master/micro_emulation_plans/src/webshell/plan.yaml":     "not a full plan\n",
	}
	for name, body := range skip {
		w, err := zw.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	fmt.Println("wrote", outPath)
}
