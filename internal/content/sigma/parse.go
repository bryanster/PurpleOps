package sigma

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// sigmaDoc is the adapter-private AST: one entry per successfully decoded rule
// file (mapped and unmapped alike — Normalize decides skips).
type sigmaDoc struct {
	Rules []rawRuleFile
}

// rawRuleFile is one parsed YAML document plus its archive path and original body.
type rawRuleFile struct {
	Path string
	Body string // original YAML text for rule_yaml storage
	Rule rawRule
}

// rawRule is the upstream Sigma YAML shape we care about.
type rawRule struct {
	Title       string         `yaml:"title"`
	ID          string         `yaml:"id"`
	Status      string         `yaml:"status"`
	Description string         `yaml:"description"`
	Level       string         `yaml:"level"`
	Tags        []string       `yaml:"tags"`
	Logsource   map[string]any `yaml:"logsource"`
}

func parseSigma(raw []byte) (*sigmaDoc, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return nil, fmt.Errorf("sigma: parse: empty payload")
	}

	// Bare YAML (single rule file) — fixtures / tiny imports.
	if looksLikeSigmaYAML(trim) {
		rule, err := decodeRule("inline.yml", trim)
		if err != nil {
			return nil, err
		}
		return &sigmaDoc{Rules: []rawRuleFile{rule}}, nil
	}

	doc := &sigmaDoc{Rules: make([]rawRuleFile, 0, 64)}
	var err error
	switch {
	case len(raw) >= 4 && raw[0] == 'P' && raw[1] == 'K':
		err = walkZip(raw, doc)
	case len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b:
		err = walkGzipTar(raw, doc)
	case looksLikeTar(raw):
		err = walkTar(bytes.NewReader(raw), doc)
	default:
		return nil, fmt.Errorf("sigma: parse: unrecognized payload (not YAML, zip, or tar.gz)")
	}
	if err != nil {
		return nil, err
	}
	if len(doc.Rules) == 0 {
		return nil, fmt.Errorf("sigma: parse: archive contains no Sigma rule YAML")
	}
	return doc, nil
}

func walkZip(raw []byte, doc *sigmaDoc) error {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("sigma: parse: zip: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !isSigmaRulePath(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("sigma: parse: zip open %s: %w", f.Name, err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return fmt.Errorf("sigma: parse: zip read %s: %w", f.Name, err)
		}
		rule, err := decodeRule(f.Name, body)
		if err != nil {
			return err
		}
		doc.Rules = append(doc.Rules, rule)
	}
	return nil
}

func walkGzipTar(raw []byte, doc *sigmaDoc) error {
	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("sigma: parse: gzip: %w", err)
	}
	defer gr.Close()
	return walkTar(gr, doc)
}

func walkTar(r io.Reader, doc *sigmaDoc) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("sigma: parse: tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !isSigmaRulePath(hdr.Name) {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("sigma: parse: tar read %s: %w", hdr.Name, err)
		}
		rule, err := decodeRule(hdr.Name, body)
		if err != nil {
			return err
		}
		doc.Rules = append(doc.Rules, rule)
	}
}

func decodeRule(pathName string, body []byte) (rawRuleFile, error) {
	var rule rawRule
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(false)
	if err := dec.Decode(&rule); err != nil {
		return rawRuleFile{}, fmt.Errorf("sigma: parse: %s: %w", pathName, err)
	}
	title := strings.TrimSpace(rule.Title)
	if title == "" {
		return rawRuleFile{}, fmt.Errorf("sigma: parse: %s: missing title", pathName)
	}
	rule.Title = title
	rule.ID = strings.TrimSpace(rule.ID)
	rule.Status = strings.TrimSpace(rule.Status)
	rule.Description = strings.TrimSpace(rule.Description)
	rule.Level = strings.TrimSpace(rule.Level)
	return rawRuleFile{
		Path: pathName,
		Body: string(body),
		Rule: rule,
	}, nil
}

func looksLikeSigmaYAML(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	if b[0] == '{' || b[0] == '[' {
		return false
	}
	s := string(b)
	// Sigma rules always have a title; detection is common but not required for
	// our reference store (we keep the body as-is).
	return strings.Contains(s, "title:") || strings.Contains(s, "title :")
}

func looksLikeTar(raw []byte) bool {
	if len(raw) < 262 {
		return false
	}
	return string(raw[257:262]) == "ustar"
}

// isSigmaRulePath accepts rule YAML under SigmaHQ layout:
//
//	rules/windows/process_creation/proc_....yml
//	sigma-master/rules-emerging-threats/...
//	rules/foo.yml (fixture convenience)
//	mapped.yml (bare fixture convenience)
//
// Skips documentation, tests, deprecated placeholders, and non-yaml.
func isSigmaRulePath(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	base := path.Base(name)
	lowerBase := strings.ToLower(base)
	if !strings.HasSuffix(lowerBase, ".yml") && !strings.HasSuffix(lowerBase, ".yaml") {
		return false
	}
	lower := strings.ToLower(name)

	// Hard excludes.
	skipParts := []string{
		"/.git/", "/.github/", "/tests/", "/test/", "/docs/", "/documentation/",
		"/deprecated/", "/rules-placeholder/", "/.sigma-legacy/",
	}
	for _, p := range skipParts {
		if strings.Contains(lower, p) {
			return false
		}
	}
	// Index / config YAML.
	if strings.Contains(lower, "index.yml") || strings.Contains(lower, "index.yaml") {
		return false
	}
	if base == "config.yml" || base == "config.yaml" {
		return false
	}

	// Prefer rules/ trees (rules, rules-emerging-threats, rules-threat-hunting, rules-dfir).
	parts := strings.Split(lower, "/")
	for _, p := range parts {
		if p == "rules" || strings.HasPrefix(p, "rules-") {
			return p != "rules-placeholder"
		}
	}

	// Fixture convenience: bare top-level rule YAML (no directory).
	if !strings.Contains(name, "/") {
		return true
	}
	return false
}
