package v1import

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// v1TestcaseRaw is the union of fields seen in testcases.json and
// custom/testcases/*.yaml (including the phase alias and optional uuid).
type v1TestcaseRaw struct {
	Name      string   `json:"name" yaml:"name"`
	MitreID   string   `json:"mitreid" yaml:"mitreid"`
	Tactic    string   `json:"tactic" yaml:"tactic"`
	Phase     string   `json:"phase" yaml:"phase"` // yaml-only alias used in the sample tree
	Objective string   `json:"objective" yaml:"objective"`
	Actions   string   `json:"actions" yaml:"actions"`
	RedNotes  string   `json:"rednotes" yaml:"rednotes"`
	Provider  string   `json:"provider" yaml:"provider"`
	UUID      string   `json:"uuid" yaml:"uuid"`
	Tools     []string `json:"tools" yaml:"tools"`
	Tags      []string `json:"tags" yaml:"tags"`
}

type v1KBRaw struct {
	MitreID  string `json:"mitreid" yaml:"mitreid"`
	Overview string `json:"overview" yaml:"overview"`
	Advice   string `json:"advice" yaml:"advice"`
	Provider string `json:"provider" yaml:"provider"`
	// Optional richer fields some operators added later.
	Title string   `json:"title" yaml:"title"`
	Body  string   `json:"body" yaml:"body"`
	Tags  []string `json:"tags" yaml:"tags"`
}

func parseTestcasesJSON(data []byte, path string) (Bundle, error) {
	out := Bundle{Format: FormatTestcasesJSON}
	trim := bytes.TrimSpace(data)

	var items []v1TestcaseRaw
	switch {
	case len(trim) > 0 && trim[0] == '[':
		if err := json.Unmarshal(trim, &items); err != nil {
			return Bundle{}, fmt.Errorf("decode testcases array: %w", err)
		}
	case len(trim) > 0 && trim[0] == '{':
		// Either {testcases:[…]} or a single object.
		var wrap struct {
			Testcases []v1TestcaseRaw `json:"testcases"`
		}
		if err := json.Unmarshal(trim, &wrap); err != nil {
			return Bundle{}, fmt.Errorf("decode testcases object: %w", err)
		}
		if wrap.Testcases != nil {
			items = wrap.Testcases
		} else {
			var one v1TestcaseRaw
			if err := json.Unmarshal(trim, &one); err != nil {
				return Bundle{}, fmt.Errorf("decode single testcase: %w", err)
			}
			items = []v1TestcaseRaw{one}
		}
	default:
		return Bundle{}, fmt.Errorf("testcases JSON must be an array or object")
	}

	for i, raw := range items {
		itemPath := path
		if len(items) > 1 {
			itemPath = fmt.Sprintf("%s[%d]", path, i)
		}
		tc, warns, err := mapTestcase(raw, itemPath)
		if err != nil {
			out.Errors = append(out.Errors, issue(itemPath, "%v", err))
			continue
		}
		for _, w := range warns {
			out.Warnings = append(out.Warnings, issue(itemPath, "%s", w))
		}
		out.Testcases = append(out.Testcases, tc)
	}
	return out, nil
}

func parseTestcasesYAML(data []byte, path string) (Bundle, error) {
	out := Bundle{Format: FormatTestcasesYAML}
	var raw v1TestcaseRaw
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Bundle{}, fmt.Errorf("decode testcase yaml: %w", err)
	}
	if raw.Name == "" && raw.Objective == "" && raw.Actions == "" && raw.MitreID == "" {
		return Bundle{}, fmt.Errorf("empty testcase document")
	}
	tc, warns, err := mapTestcase(raw, path)
	if err != nil {
		out.Errors = append(out.Errors, issue(path, "%v", err))
		return out, nil
	}
	for _, w := range warns {
		out.Warnings = append(out.Warnings, issue(path, "%s", w))
	}
	out.Testcases = append(out.Testcases, tc)
	return out, nil
}

func parseKnowledgebaseYAML(data []byte, path string) (Bundle, error) {
	out := Bundle{Format: FormatKnowledgebaseYAML}
	var raw v1KBRaw
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Bundle{}, fmt.Errorf("decode knowledgebase yaml: %w", err)
	}
	if raw.MitreID == "" && raw.Overview == "" && raw.Advice == "" && raw.Title == "" && raw.Body == "" {
		return Bundle{}, fmt.Errorf("empty knowledgebase document")
	}
	n, warns, err := mapNote(raw, path)
	if err != nil {
		out.Errors = append(out.Errors, issue(path, "%v", err))
		return out, nil
	}
	for _, w := range warns {
		out.Warnings = append(out.Warnings, issue(path, "%s", w))
	}
	out.Notes = append(out.Notes, n)
	return out, nil
}

func mapTestcase(raw v1TestcaseRaw, path string) (Testcase, []string, error) {
	var warns []string
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		name = strings.TrimSpace(raw.Objective)
	}
	if name == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if i := strings.IndexByte(base, '['); i >= 0 {
			base = base[:i]
		}
		name = strings.TrimSpace(base)
	}
	if name == "" || name == RootPath || name == "-" {
		return Testcase{}, nil, fmt.Errorf("testcase has no name, objective, or filename to derive a title from")
	}

	command := raw.Actions
	if strings.TrimSpace(command) == "" {
		warns = append(warns, "actions/command is empty; template will have an empty command")
	} else {
		warns = append(warns,
			"v1 source has a flat actions string only; stored in command — cleanup and input_args were absent and were not invented")
	}

	tactic := strings.TrimSpace(raw.Tactic)
	if tactic == "" {
		tactic = strings.TrimSpace(raw.Phase)
	}

	var descParts []string
	if obj := strings.TrimSpace(raw.Objective); obj != "" && obj != name {
		descParts = append(descParts, obj)
	}
	if tactic != "" {
		descParts = append(descParts, "Tactic: "+tactic)
	}
	if p := strings.TrimSpace(raw.Provider); p != "" {
		descParts = append(descParts, "Provider: "+p)
	}
	if len(raw.Tags) > 0 {
		descParts = append(descParts, "Tags: "+strings.Join(raw.Tags, ", "))
	}
	if len(raw.Tools) > 0 {
		descParts = append(descParts, "Tools: "+strings.Join(raw.Tools, ", "))
	}
	if rn := strings.TrimSpace(raw.RedNotes); rn != "" {
		descParts = append(descParts, "Red notes:\n"+rn)
	}

	var techs []string
	if t := NormalizeTechnique(raw.MitreID); t != "" {
		techs = []string{t}
	} else if strings.TrimSpace(raw.MitreID) != "" {
		warns = append(warns, fmt.Sprintf("mitreid %q is not a MITRE technique id (T#### or T####.###); dropped", raw.MitreID))
	}

	tc := Testcase{
		SourcePath:           path,
		ExternalID:           ExternalIDForTestcase(raw.UUID, name, command),
		Name:                 name,
		Description:          strings.Join(descParts, "\n\n"),
		Command:              command,
		TechniqueExternalIDs: techs,
		Warnings:             warns,
	}
	return tc, warns, nil
}

func mapNote(raw v1KBRaw, path string) (Note, []string, error) {
	var warns []string
	title := strings.TrimSpace(raw.Title)
	if title == "" {
		if m := strings.TrimSpace(raw.MitreID); m != "" {
			title = "Knowledge: " + m
		}
	}
	if title == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		title = strings.TrimSpace(base)
	}
	if title == "" || title == RootPath {
		return Note{}, nil, fmt.Errorf("knowledgebase entry has no title, mitreid, or filename")
	}

	var body string
	if b := strings.TrimSpace(raw.Body); b != "" {
		body = b
	} else {
		var parts []string
		if o := strings.TrimSpace(raw.Overview); o != "" {
			parts = append(parts, "## Overview\n\n"+o)
		}
		if a := strings.TrimSpace(raw.Advice); a != "" {
			parts = append(parts, "## Advice\n\n"+a)
		}
		if p := strings.TrimSpace(raw.Provider); p != "" {
			parts = append(parts, "_Provider: "+p+"_")
		}
		body = strings.Join(parts, "\n\n")
	}
	if strings.TrimSpace(body) == "" {
		warns = append(warns, "knowledgebase body is empty (no overview/advice/body)")
	}

	tech := NormalizeTechnique(raw.MitreID)
	if strings.TrimSpace(raw.MitreID) != "" && tech == "" {
		warns = append(warns, fmt.Sprintf("mitreid %q is not a MITRE technique id; stored without technique link", raw.MitreID))
	}

	tags := append([]string(nil), raw.Tags...)
	if p := strings.TrimSpace(raw.Provider); p != "" {
		found := false
		for _, t := range tags {
			if strings.EqualFold(t, p) {
				found = true
				break
			}
		}
		if !found {
			tags = append(tags, p)
		}
	}

	n := Note{
		SourcePath:          path,
		ExternalID:          ExternalIDForNote(raw.MitreID, path),
		Title:               title,
		BodyMarkdown:        body,
		Tags:                tags,
		TechniqueExternalID: tech,
		Warnings:            warns,
	}
	return n, warns, nil
}

// customExportDoc is the M2-011 export shape (JSON or YAML).
type customExportDoc struct {
	Meta                    map[string]any       `json:"meta" yaml:"meta"`
	ProcedureTemplates      []customExportProc   `json:"procedureTemplates" yaml:"procedureTemplates"`
	DetectionRules          []customExportDetect `json:"detectionRules" yaml:"detectionRules"`
	Notes                   []customExportNote   `json:"notes" yaml:"notes"`
	ProcedureTemplatesSnake []customExportProc   `json:"procedure_templates" yaml:"procedure_templates"`
	DetectionRulesSnake     []customExportDetect `json:"detection_rules" yaml:"detection_rules"`
}

type customExportProc struct {
	ExternalID             string   `json:"externalId" yaml:"externalId"`
	ExternalIDSnake        string   `json:"external_id" yaml:"external_id"`
	Name                   string   `json:"name" yaml:"name"`
	Description            string   `json:"description" yaml:"description"`
	Platforms              []string `json:"platforms" yaml:"platforms"`
	Executor               string   `json:"executor" yaml:"executor"`
	ElevationRequired      bool     `json:"elevationRequired" yaml:"elevationRequired"`
	Command                string   `json:"command" yaml:"command"`
	Cleanup                string   `json:"cleanup" yaml:"cleanup"`
	TechniqueExternalIDs   []string `json:"techniqueExternalIds" yaml:"techniqueExternalIds"`
	TechniqueExternalSnake []string `json:"technique_external_ids" yaml:"technique_external_ids"`
	DependencyExecutorName string   `json:"dependencyExecutorName" yaml:"dependencyExecutorName"`
	Dependencies           string   `json:"dependencies" yaml:"dependencies"`
	InputArgs              any      `json:"inputArgs" yaml:"inputArgs"`
}

type customExportDetect struct {
	ExternalID             string         `json:"externalId" yaml:"externalId"`
	ExternalIDSnake        string         `json:"external_id" yaml:"external_id"`
	Name                   string         `json:"name" yaml:"name"`
	Description            string         `json:"description" yaml:"description"`
	TechniqueExternalIDs   []string       `json:"techniqueExternalIds" yaml:"techniqueExternalIds"`
	TechniqueExternalSnake []string       `json:"technique_external_ids" yaml:"technique_external_ids"`
	Level                  string         `json:"level" yaml:"level"`
	RuleStatus             string         `json:"ruleStatus" yaml:"ruleStatus"`
	Logsource              map[string]any `json:"logsource" yaml:"logsource"`
	RuleYAML               string         `json:"ruleYaml" yaml:"ruleYaml"`
	RuleYAMLSnake          string         `json:"rule_yaml" yaml:"rule_yaml"`
}

type customExportNote struct {
	ExternalID             string   `json:"externalId" yaml:"externalId"`
	ExternalIDSnake        string   `json:"external_id" yaml:"external_id"`
	Title                  string   `json:"title" yaml:"title"`
	BodyMarkdown           string   `json:"bodyMarkdown" yaml:"bodyMarkdown"`
	BodyMarkdownSnake      string   `json:"body_markdown" yaml:"body_markdown"`
	Tags                   []string `json:"tags" yaml:"tags"`
	TechniqueExternalID    string   `json:"techniqueExternalId" yaml:"techniqueExternalId"`
	TechniqueExternalSnake string   `json:"technique_external_id" yaml:"technique_external_id"`
}

func parseCustomExport(data []byte, path string) (Bundle, error) {
	out := Bundle{Format: FormatCustomExport}
	var doc customExportDoc
	trim := bytes.TrimSpace(data)
	var err error
	if len(trim) > 0 && (trim[0] == '{' || trim[0] == '[') {
		err = json.Unmarshal(trim, &doc)
	} else {
		err = yaml.Unmarshal(trim, &doc)
	}
	if err != nil {
		return Bundle{}, fmt.Errorf("decode custom export: %w", err)
	}

	procs := doc.ProcedureTemplates
	if len(procs) == 0 {
		procs = doc.ProcedureTemplatesSnake
	}
	dets := doc.DetectionRules
	if len(dets) == 0 {
		dets = doc.DetectionRulesSnake
	}

	for i, p := range procs {
		itemPath := fmt.Sprintf("%s#procedureTemplates[%d]", path, i)
		ext := firstNonEmpty(p.ExternalID, p.ExternalIDSnake)
		name := strings.TrimSpace(p.Name)
		if name == "" {
			out.Errors = append(out.Errors, issue(itemPath, "procedure template missing name"))
			continue
		}
		if ext == "" {
			ext = ExternalIDForTestcase("", name, p.Command)
		}
		techs := p.TechniqueExternalIDs
		if len(techs) == 0 {
			techs = p.TechniqueExternalSnake
		}
		var clean []string
		for _, t := range techs {
			if n := NormalizeTechnique(t); n != "" {
				clean = append(clean, n)
			} else if strings.TrimSpace(t) != "" {
				out.Warnings = append(out.Warnings, issue(itemPath, "dropping invalid technique id %q", t))
			}
		}
		var inputJSON []byte
		if p.InputArgs != nil {
			b, mErr := json.Marshal(p.InputArgs)
			if mErr != nil {
				out.Warnings = append(out.Warnings, issue(itemPath, "inputArgs not serializable: %v", mErr))
			} else {
				inputJSON = b
			}
		}
		out.Testcases = append(out.Testcases, Testcase{
			SourcePath:             itemPath,
			ExternalID:             ext,
			Name:                   name,
			Description:            p.Description,
			Command:                p.Command,
			Platforms:              append([]string(nil), p.Platforms...),
			Executor:               p.Executor,
			ElevationRequired:      p.ElevationRequired,
			Cleanup:                p.Cleanup,
			InputArgsJSON:          inputJSON,
			DependencyExecutorName: p.DependencyExecutorName,
			Dependencies:           p.Dependencies,
			TechniqueExternalIDs:   clean,
		})
	}

	for i, d := range dets {
		itemPath := fmt.Sprintf("%s#detectionRules[%d]", path, i)
		ext := firstNonEmpty(d.ExternalID, d.ExternalIDSnake)
		name := strings.TrimSpace(d.Name)
		if name == "" {
			out.Errors = append(out.Errors, issue(itemPath, "detection rule missing name"))
			continue
		}
		if ext == "" {
			ext = "export:detection:" + slug(name)
		}
		techs := d.TechniqueExternalIDs
		if len(techs) == 0 {
			techs = d.TechniqueExternalSnake
		}
		var clean []string
		for _, t := range techs {
			if n := NormalizeTechnique(t); n != "" {
				clean = append(clean, n)
			}
		}
		ruleYAML := firstNonEmpty(d.RuleYAML, d.RuleYAMLSnake)
		var lsJSON []byte
		if d.Logsource != nil {
			b, mErr := json.Marshal(d.Logsource)
			if mErr != nil {
				out.Warnings = append(out.Warnings, issue(itemPath, "logsource not serializable: %v", mErr))
			} else {
				lsJSON = b
			}
		}
		out.Detections = append(out.Detections, Detection{
			SourcePath:           itemPath,
			ExternalID:           ext,
			Name:                 name,
			Description:          d.Description,
			TechniqueExternalIDs: clean,
			Level:                d.Level,
			RuleStatus:           d.RuleStatus,
			LogsourceJSON:        lsJSON,
			RuleYAML:             ruleYAML,
		})
	}

	for i, n := range doc.Notes {
		itemPath := fmt.Sprintf("%s#notes[%d]", path, i)
		ext := firstNonEmpty(n.ExternalID, n.ExternalIDSnake)
		title := strings.TrimSpace(n.Title)
		if title == "" {
			out.Errors = append(out.Errors, issue(itemPath, "note missing title"))
			continue
		}
		if ext == "" {
			ext = ExternalIDForNote("", title)
		}
		body := firstNonEmpty(n.BodyMarkdown, n.BodyMarkdownSnake)
		tech := NormalizeTechnique(firstNonEmpty(n.TechniqueExternalID, n.TechniqueExternalSnake))
		out.Notes = append(out.Notes, Note{
			SourcePath:          itemPath,
			ExternalID:          ext,
			Title:               title,
			BodyMarkdown:        body,
			Tags:                append([]string(nil), n.Tags...),
			TechniqueExternalID: tech,
		})
	}

	if len(out.Testcases) == 0 && len(out.Notes) == 0 && len(out.Detections) == 0 && len(out.Errors) == 0 {
		return Bundle{}, fmt.Errorf("custom export contained no procedureTemplates, detectionRules, or notes")
	}
	return out, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
