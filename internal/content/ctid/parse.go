package ctid

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

// ctidDoc is the adapter-private AST: one entry per successfully decoded plan
// file under Emulation_Plan/yaml/.
type ctidDoc struct {
	Plans []rawPlanFile
}

// rawPlanFile is one parsed plan YAML plus its archive path.
type rawPlanFile struct {
	Path    string
	Details rawPlanDetails
	Steps   []rawStep
}

// rawPlanDetails is the upstream emulation_plan_details header.
type rawPlanDetails struct {
	ID                   string `yaml:"id"`
	AdversaryName        string `yaml:"adversary_name"`
	AdversaryDescription string `yaml:"adversary_description"`
	AttackVersion        string `yaml:"attack_version"`
	FormatVersion        string `yaml:"format_version"`
}

// rawStep is one procedure entry in a plan YAML list.
//
// Platforms / executors / input_arguments stay as generic maps so Apply can
// re-encode them into the procedure JSON column without losing keys.
type rawStep struct {
	ID                     string         `yaml:"id"`
	Name                   string         `yaml:"name"`
	Description            string         `yaml:"description"`
	Tactic                 string         `yaml:"tactic"`
	Technique              rawTechnique   `yaml:"technique"`
	CTISource              any            `yaml:"cti_source"`
	ProcedureGroup         string         `yaml:"procedure_group"`
	ProcedureStep          string         `yaml:"procedure_step"`
	Platforms              map[string]any `yaml:"platforms"`
	Executors              []rawExecutor  `yaml:"executors"`
	InputArguments         map[string]any `yaml:"input_arguments"`
	DependencyExecutorName string         `yaml:"dependency_executor_name"`
	Dependencies           any            `yaml:"dependencies"`
}

type rawTechnique struct {
	AttackID string `yaml:"attack_id"`
	Name     string `yaml:"name"`
}

type rawExecutor struct {
	Name      string `yaml:"name"`
	Command   string `yaml:"command"`
	Cleanup   string `yaml:"cleanup"`
	Timeout   any    `yaml:"timeout"`
	Enabled   any    `yaml:"enabled"`
	Elevation any    `yaml:"elevation_required"`
}

func parseCTID(raw []byte) (*ctidDoc, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return nil, fmt.Errorf("ctid: parse: empty payload")
	}

	// Bare plan YAML (single plan file) — fixtures / tiny imports.
	if looksLikePlanYAML(trim) {
		plan, err := decodePlan("inline.yaml", trim)
		if err != nil {
			return nil, err
		}
		return &ctidDoc{Plans: []rawPlanFile{plan}}, nil
	}

	doc := &ctidDoc{Plans: make([]rawPlanFile, 0, 16)}
	var err error
	switch {
	case len(raw) >= 4 && raw[0] == 'P' && raw[1] == 'K':
		err = walkZip(raw, doc)
	case len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b:
		err = walkGzipTar(raw, doc)
	case looksLikeTar(raw):
		err = walkTar(bytes.NewReader(raw), doc)
	default:
		return nil, fmt.Errorf("ctid: parse: unrecognized payload (not YAML, zip, or tar.gz)")
	}
	if err != nil {
		return nil, err
	}
	if len(doc.Plans) == 0 {
		return nil, fmt.Errorf("ctid: parse: archive contains no Emulation_Plan/yaml plan files")
	}
	return doc, nil
}

func walkZip(raw []byte, doc *ctidDoc) error {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("ctid: parse: zip: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !isPlanYAMLPath(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("ctid: parse: zip open %s: %w", f.Name, err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return fmt.Errorf("ctid: parse: zip read %s: %w", f.Name, err)
		}
		plan, err := decodePlan(f.Name, body)
		if err != nil {
			return err
		}
		doc.Plans = append(doc.Plans, plan)
	}
	return nil
}

func walkGzipTar(raw []byte, doc *ctidDoc) error {
	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("ctid: parse: gzip: %w", err)
	}
	defer gr.Close()
	return walkTar(gr, doc)
}

func walkTar(r io.Reader, doc *ctidDoc) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("ctid: parse: tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !isPlanYAMLPath(hdr.Name) {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("ctid: parse: tar read %s: %w", hdr.Name, err)
		}
		plan, err := decodePlan(hdr.Name, body)
		if err != nil {
			return err
		}
		doc.Plans = append(doc.Plans, plan)
	}
}

func decodePlan(pathName string, body []byte) (rawPlanFile, error) {
	// Upstream plan YAML is a single document: a sequence whose first element
	// is {emulation_plan_details: ...} and the rest are steps.
	var items []map[string]any
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(false)
	if err := dec.Decode(&items); err != nil {
		return rawPlanFile{}, fmt.Errorf("ctid: parse: %s: %w", pathName, err)
	}
	if len(items) == 0 {
		return rawPlanFile{}, fmt.Errorf("ctid: parse: %s: empty YAML sequence", pathName)
	}

	var (
		details rawPlanDetails
		steps   []rawStep
		have    bool
	)
	for i, item := range items {
		if rawDetails, ok := item["emulation_plan_details"]; ok {
			if have {
				return rawPlanFile{}, fmt.Errorf(
					"ctid: parse: %s: multiple emulation_plan_details entries", pathName)
			}
			buf, err := yaml.Marshal(rawDetails)
			if err != nil {
				return rawPlanFile{}, fmt.Errorf("ctid: parse: %s: details: %w", pathName, err)
			}
			if err := yaml.Unmarshal(buf, &details); err != nil {
				return rawPlanFile{}, fmt.Errorf("ctid: parse: %s: details: %w", pathName, err)
			}
			have = true
			continue
		}
		// Skip pure comment / empty maps.
		if len(item) == 0 {
			continue
		}
		buf, err := yaml.Marshal(item)
		if err != nil {
			return rawPlanFile{}, fmt.Errorf("ctid: parse: %s: step %d: %w", pathName, i, err)
		}
		var step rawStep
		if err := yaml.Unmarshal(buf, &step); err != nil {
			return rawPlanFile{}, fmt.Errorf("ctid: parse: %s: step %d: %w", pathName, i, err)
		}
		name := strings.TrimSpace(step.Name)
		if name == "" {
			return rawPlanFile{}, fmt.Errorf(
				"ctid: parse: %s: step %d: missing name", pathName, i)
		}
		step.Name = name
		step.ID = strings.TrimSpace(step.ID)
		step.Description = strings.TrimSpace(step.Description)
		step.Tactic = strings.TrimSpace(step.Tactic)
		step.Technique.AttackID = strings.TrimSpace(step.Technique.AttackID)
		step.Technique.Name = strings.TrimSpace(step.Technique.Name)
		step.ProcedureGroup = strings.TrimSpace(step.ProcedureGroup)
		step.ProcedureStep = strings.TrimSpace(step.ProcedureStep)
		step.DependencyExecutorName = strings.TrimSpace(step.DependencyExecutorName)
		steps = append(steps, step)
	}
	if !have {
		return rawPlanFile{}, fmt.Errorf(
			"ctid: parse: %s: missing emulation_plan_details header", pathName)
	}
	details.ID = strings.TrimSpace(details.ID)
	details.AdversaryName = strings.TrimSpace(details.AdversaryName)
	details.AdversaryDescription = strings.TrimSpace(details.AdversaryDescription)
	details.AttackVersion = strings.TrimSpace(details.AttackVersion)
	details.FormatVersion = strings.TrimSpace(details.FormatVersion)
	if details.AdversaryName == "" {
		return rawPlanFile{}, fmt.Errorf(
			"ctid: parse: %s: emulation_plan_details.adversary_name is required", pathName)
	}
	if len(steps) == 0 {
		return rawPlanFile{}, fmt.Errorf(
			"ctid: parse: %s: plan has no steps", pathName)
	}
	return rawPlanFile{
		Path:    pathName,
		Details: details,
		Steps:   steps,
	}, nil
}

func looksLikePlanYAML(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	if b[0] == '{' || b[0] == '[' {
		return false
	}
	s := string(b)
	return strings.Contains(s, "emulation_plan_details")
}

func looksLikeTar(raw []byte) bool {
	if len(raw) < 262 {
		return false
	}
	return string(raw[257:262]) == "ustar"
}

// isPlanYAMLPath accepts CTID machine-readable plan YAML:
//
//	fin6/Emulation_Plan/yaml/FIN6.yaml
//	adversary_emulation_library-master/apt29/Emulation_Plan/yaml/APT29.yaml
//	inline.yaml (bare fixture convenience)
//
// Skips planners/, documentation, Archive/, micro_emulation_plans, and
// non-yaml.
func isPlanYAMLPath(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	base := path.Base(name)
	lowerBase := strings.ToLower(base)
	if !strings.HasSuffix(lowerBase, ".yml") && !strings.HasSuffix(lowerBase, ".yaml") {
		return false
	}
	lower := strings.ToLower(name)

	// Hard excludes.
	skipParts := []string{
		"/.git/", "/.github/", "/archive/", "/planners/",
		"/micro_emulation_plans/", "/resources/", "/structure/",
		"/docs/", "/documentation/",
	}
	for _, p := range skipParts {
		if strings.Contains(lower, p) {
			return false
		}
	}
	// README / index noise under yaml/.
	if strings.HasPrefix(lowerBase, "readme") {
		return false
	}

	// Canonical layout: .../Emulation_Plan/yaml/<file>.yaml
	if strings.Contains(lower, "/emulation_plan/yaml/") {
		return true
	}

	// Fixture convenience: bare top-level plan YAML (no directory).
	if !strings.Contains(name, "/") {
		return true
	}
	return false
}
