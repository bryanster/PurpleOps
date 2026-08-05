package v1import

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Format names the parser the caller selected or auto-detection resolved.
type Format string

const (
	FormatAuto              Format = "auto"
	FormatTestcasesJSON     Format = "testcases_json"
	FormatTestcasesYAML     Format = "testcases_yaml"
	FormatKnowledgebaseYAML Format = "knowledgebase_yaml"
	FormatCustomExport      Format = "custom_export"
	FormatMixed             Format = "mixed"
)

// Issue is one per-path warning or error.
type Issue struct {
	Path    string
	Message string
}

// Testcase is a v1 custom testcase mapped toward a procedure template.
//
// v1 only had a flat actions string. Command holds that string; Cleanup and
// structured args are intentionally empty unless the source was an M2-011
// custom export (which carries full structure).
type Testcase struct {
	// SourcePath is the file inside the upload (or "-" for a root JSON file).
	SourcePath string
	// ExternalID is the deterministic upsert key.
	ExternalID string
	Name       string
	// Description carries objective plus any non-structured metadata
	// (tactic, provider, tags, tools, rednotes) the v1 shape had.
	Description string
	// Command is the flat v1 actions string. Never fabricated from nothing.
	Command string
	// Optional structured fields — populated for custom-export re-import.
	Platforms              []string
	Executor               string
	ElevationRequired      bool
	Cleanup                string
	InputArgsJSON          []byte // raw JSON array; nil → default empty array
	DependencyExecutorName string
	Dependencies           string
	// TechniqueExternalIDs holds validated MITRE ids when present.
	TechniqueExternalIDs []string
	// Warnings are item-level notes (e.g. missing cleanup/args).
	Warnings []string
}

// Note is a v1 knowledgebase entry mapped toward a content_note.
type Note struct {
	SourcePath          string
	ExternalID          string
	Title               string
	BodyMarkdown        string
	Tags                []string
	TechniqueExternalID string
	Warnings            []string
}

// Detection is a custom-export detection rule ref (not a v1 native shape).
type Detection struct {
	SourcePath           string
	ExternalID           string
	Name                 string
	Description          string
	TechniqueExternalIDs []string
	Level                string
	RuleStatus           string
	LogsourceJSON        []byte // raw JSON object; empty → {}
	RuleYAML             string
	Warnings             []string
}

// Bundle is everything parsed from one upload or filesystem path.
type Bundle struct {
	// Format is the resolved top-level format (auto → concrete).
	Format     Format
	Testcases  []Testcase
	Notes      []Note
	Detections []Detection
	Warnings   []Issue
	Errors     []Issue
}

// RootPath is the synthetic path for a single-file upload with no name.
const RootPath = "-"

var (
	nonSlug = regexp.MustCompile(`[^a-z0-9]+`)
	techRE  = regexp.MustCompile(`(?i)^T\d{4}(\.\d{3})?$`)
)

// ExternalIDForTestcase derives a stable upsert key from a v1 testcase.
//
// Prefer uuid when present, else a slug of the name, else a content hash so
// nameless rows still re-import as the same row.
func ExternalIDForTestcase(uuid, name, command string) string {
	if u := strings.TrimSpace(uuid); u != "" {
		return "v1:testcase:" + strings.ToLower(u)
	}
	if s := slug(name); s != "" {
		return "v1:testcase:" + s
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(command)))
	return "v1:testcase:" + hex.EncodeToString(sum[:8])
}

// ExternalIDForNote derives a stable upsert key from a v1 KB entry.
func ExternalIDForNote(mitreID, sourcePath string) string {
	if m := strings.TrimSpace(mitreID); m != "" {
		return "v1:kb:" + strings.ToUpper(m)
	}
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if s := slug(base); s != "" {
		return "v1:kb:" + s
	}
	return "v1:kb:unnamed"
}

// NormalizeTechnique returns the canonical MITRE id when s looks like one,
// otherwise empty.
func NormalizeTechnique(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	up := strings.ToUpper(s)
	if !techRE.MatchString(up) {
		return ""
	}
	// Preserve subtechnique casing shape: T####.###
	if i := strings.IndexByte(up, '.'); i >= 0 {
		return up[:i] + "." + up[i+1:]
	}
	return up
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	out = nonSlug.ReplaceAllString(out, "-")
	out = strings.Trim(out, "-")
	if len(out) > 80 {
		out = out[:80]
		out = strings.Trim(out, "-")
	}
	return out
}

func issue(path, format string, args ...any) Issue {
	if path == "" {
		path = RootPath
	}
	return Issue{Path: path, Message: fmt.Sprintf(format, args...)}
}
