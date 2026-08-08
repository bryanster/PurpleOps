package v1import

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseBytes parses a single file or zip archive from memory.
//
// filename is a hint for auto-detection and issue paths (may be empty).
func ParseBytes(data []byte, filename string, format Format) (Bundle, error) {
	name := filename
	if name == "" {
		name = RootPath
	}
	if format == "" {
		format = FormatAuto
	}
	if isZip(data) {
		return parseZip(data, format)
	}
	return parseOne(data, name, format)
}

// ParsePath parses a file or directory on disk.
//
// Directories are walked non-recursively for *.yaml / *.yml / *.json (and one
// level of subdirs named testcases/ or knowledgebase/, matching v1 layouts).
func ParsePath(path string, format Format) (Bundle, error) {
	if format == "" {
		format = FormatAuto
	}
	info, err := os.Stat(path)
	if err != nil {
		return Bundle{}, fmt.Errorf("v1import: stat %s: %w", path, err)
	}
	if info.IsDir() {
		return parseDir(path, format)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Bundle{}, fmt.Errorf("v1import: read %s: %w", path, err)
	}
	if isZip(data) {
		return parseZip(data, format)
	}
	return parseOne(data, filepath.Base(path), format)
}

func isZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && (data[2] == 3 || data[2] == 5 || data[2] == 7)
}

func parseZip(data []byte, format Format) (Bundle, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Bundle{}, fmt.Errorf("v1import: open zip: %w", err)
	}
	out := Bundle{Format: FormatMixed}
	if format != FormatAuto {
		out.Format = format
	}
	seen := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(f.Name)
		base := filepath.Base(name)
		if strings.HasPrefix(base, ".") || strings.HasPrefix(base, "__MACOSX") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(base))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			out.Errors = append(out.Errors, issue(name, "open: %v", err))
			continue
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			out.Errors = append(out.Errors, issue(name, "read: %v", err))
			continue
		}
		fileFmt := format
		if format == FormatAuto {
			fileFmt = guessFormat(raw, name)
		}
		part, err := parseOne(raw, name, fileFmt)
		if err != nil {
			out.Errors = append(out.Errors, issue(name, "%v", err))
			continue
		}
		mergeBundle(&out, part)
		seen++
	}
	if seen == 0 && len(out.Errors) == 0 {
		return Bundle{}, fmt.Errorf("v1import: zip contained no json/yaml content files")
	}
	if format == FormatAuto {
		out.Format = resolveMixedFormat(out)
	}
	return out, nil
}

func parseDir(root string, format Format) (Bundle, error) {
	out := Bundle{Format: format}
	if format == FormatAuto {
		out.Format = FormatMixed
	}
	// Prefer classic v1 subdirs when present.
	subTargets := []struct {
		sub string
		fmt Format
	}{
		{"testcases", FormatTestcasesYAML},
		{"knowledgebase", FormatKnowledgebaseYAML},
	}
	usedSub := false
	for _, t := range subTargets {
		dir := filepath.Join(root, t.sub)
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			continue
		}
		usedSub = true
		ff := t.fmt
		if format != FormatAuto {
			ff = format
		}
		if err := walkFlat(dir, ff, &out); err != nil {
			return Bundle{}, err
		}
	}
	// Also accept a root testcases.json.
	for _, name := range []string{"testcases.json", "custom-export.json", "custom-export.yaml", "export.json", "export.yaml"} {
		p := filepath.Join(root, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			raw, err := os.ReadFile(p)
			if err != nil {
				out.Errors = append(out.Errors, issue(name, "read: %v", err))
				continue
			}
			ff := format
			if format == FormatAuto {
				ff = guessFormat(raw, name)
			}
			part, err := parseOne(raw, name, ff)
			if err != nil {
				out.Errors = append(out.Errors, issue(name, "%v", err))
				continue
			}
			mergeBundle(&out, part)
			usedSub = true
		}
	}
	if !usedSub {
		// Flat directory of yaml/json.
		if err := walkFlat(root, format, &out); err != nil {
			return Bundle{}, err
		}
	}
	if format == FormatAuto {
		out.Format = resolveMixedFormat(out)
	}
	if len(out.Testcases) == 0 && len(out.Notes) == 0 && len(out.Detections) == 0 && len(out.Errors) == 0 {
		return Bundle{}, fmt.Errorf("v1import: no importable files under %s", root)
	}
	return out, nil
}

func walkFlat(dir string, format Format, out *Bundle) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("v1import: readdir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			out.Errors = append(out.Errors, issue(name, "read: %v", err))
			continue
		}
		// Prefer relative-looking path for issues.
		rel := name
		if base := filepath.Base(dir); base == "testcases" || base == "knowledgebase" {
			rel = base + "/" + name
		}
		ff := format
		if format == FormatAuto {
			ff = guessFormat(raw, rel)
		}
		part, err := parseOne(raw, rel, ff)
		if err != nil {
			out.Errors = append(out.Errors, issue(rel, "%v", err))
			continue
		}
		mergeBundle(out, part)
	}
	return nil
}

func parseOne(data []byte, path string, format Format) (Bundle, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return Bundle{}, fmt.Errorf("empty file")
	}
	if format == FormatAuto {
		format = guessFormat(data, path)
	}
	switch format {
	case FormatTestcasesJSON:
		return parseTestcasesJSON(data, path)
	case FormatTestcasesYAML:
		return parseTestcasesYAML(data, path)
	case FormatKnowledgebaseYAML:
		return parseKnowledgebaseYAML(data, path)
	case FormatCustomExport:
		return parseCustomExport(data, path)
	default:
		return Bundle{}, fmt.Errorf("unsupported format %q", format)
	}
}

func guessFormat(data []byte, path string) Format {
	lower := strings.ToLower(path)
	base := filepath.Base(lower)

	// Path heuristics first — they encode the v1 layouts operators actually have.
	if strings.Contains(lower, "/knowledgebase/") || strings.HasPrefix(lower, "knowledgebase/") {
		return FormatKnowledgebaseYAML
	}
	if strings.Contains(lower, "/testcases/") || strings.HasPrefix(lower, "testcases/") {
		if strings.HasSuffix(base, ".json") {
			return FormatTestcasesJSON
		}
		return FormatTestcasesYAML
	}
	if base == "testcases.json" {
		return FormatTestcasesJSON
	}

	trim := bytes.TrimSpace(data)
	if len(trim) == 0 {
		return FormatTestcasesYAML
	}

	// JSON?
	if trim[0] == '[' || trim[0] == '{' {
		var probe any
		if err := json.Unmarshal(trim, &probe); err == nil {
			switch v := probe.(type) {
			case []any:
				return FormatTestcasesJSON
			case map[string]any:
				if _, ok := v["testcases"]; ok {
					return FormatTestcasesJSON
				}
				if _, ok := v["meta"]; ok {
					if _, ok := v["procedureTemplates"]; ok {
						return FormatCustomExport
					}
					if _, ok := v["procedure_templates"]; ok {
						return FormatCustomExport
					}
				}
				if _, ok := v["procedureTemplates"]; ok {
					return FormatCustomExport
				}
				// Single testcase object as JSON.
				if hasAnyKey(v, "actions", "mitreid", "name", "objective") {
					return FormatTestcasesJSON
				}
			}
		}
	}

	// YAML probe.
	var y map[string]any
	if err := yaml.Unmarshal(trim, &y); err == nil && y != nil {
		if _, ok := y["meta"]; ok {
			if hasAnyKey(y, "procedureTemplates", "procedure_templates", "detectionRules", "notes") {
				return FormatCustomExport
			}
		}
		if hasAnyKey(y, "overview", "advice") {
			return FormatKnowledgebaseYAML
		}
		if hasAnyKey(y, "actions", "objective", "rednotes", "phase", "tactic") {
			return FormatTestcasesYAML
		}
		// mitreid-only yaml with no KB keys still could be a thin testcase.
		if hasAnyKey(y, "mitreid", "name") && !hasAnyKey(y, "overview", "advice") {
			return FormatTestcasesYAML
		}
	}
	// Default: treat as testcase yaml (the layout the seeder expected).
	return FormatTestcasesYAML
}

func hasAnyKey(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func mergeBundle(dst *Bundle, src Bundle) {
	dst.Testcases = append(dst.Testcases, src.Testcases...)
	dst.Notes = append(dst.Notes, src.Notes...)
	dst.Detections = append(dst.Detections, src.Detections...)
	dst.Warnings = append(dst.Warnings, src.Warnings...)
	dst.Errors = append(dst.Errors, src.Errors...)
}

func resolveMixedFormat(b Bundle) Format {
	hasTC := len(b.Testcases) > 0
	hasNote := len(b.Notes) > 0
	hasDet := len(b.Detections) > 0
	switch {
	case hasTC && !hasNote && !hasDet:
		// Could still be json vs yaml — mixed zip of testcases stays mixed-ish.
		return FormatTestcasesYAML
	case hasNote && !hasTC && !hasDet:
		return FormatKnowledgebaseYAML
	case hasDet && !hasTC && !hasNote:
		return FormatCustomExport
	case hasDet || (hasTC && hasNote):
		return FormatMixed
	default:
		return FormatMixed
	}
}
