package ctid

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Catalog is the normalized CTID library for one rolling-head sync.
//
// It is a single [content.Object] so the runner's item_count prefers
// [Catalog.ItemCount] over len(objects).
type Catalog struct {
	Plans             []Plan
	MissingTechniques int // steps with empty technique_external_id
}

// ItemCount is the plan total written for this sync.
func (c *Catalog) ItemCount() int64 { return int64(len(c.Plans)) }

// SuccessMessage is the operator-facing job message after a successful apply.
func (c *Catalog) SuccessMessage() string {
	if c == nil {
		return ""
	}
	steps := 0
	for _, p := range c.Plans {
		steps += len(p.Steps)
	}
	msg := fmt.Sprintf("applied %d emulation plans (%d steps)", len(c.Plans), steps)
	if c.MissingTechniques > 0 {
		msg += fmt.Sprintf(", %d steps missing technique", c.MissingTechniques)
	}
	return msg
}

// Plan is one normalized CTID emulation plan ready for Apply.
type Plan struct {
	ExternalID    string
	Name          string
	Description   string
	AdversaryName string
	Metadata      json.RawMessage // JSON object
	Steps         []Step
}

// Step is one ordered procedure under a plan.
type Step struct {
	ExternalID          string
	Position            int // 1-based document order
	Name                string
	Description         string
	TechniqueExternalID string
	Procedure           json.RawMessage // JSON object
}

// techniqueIDRE accepts ATT&CK technique external ids: T1059, T1059.001.
// Conservative — wrong links are worse than empty.
var techniqueIDRE = regexp.MustCompile(`(?i)^t\d{4}(?:\.\d{3})?$`)

func normalize(doc *ctidDoc) (*Catalog, error) {
	if doc == nil {
		return nil, fmt.Errorf("ctid: normalize: nil document")
	}
	if len(doc.Plans) == 0 {
		return nil, fmt.Errorf("ctid: normalize: zero plans parsed")
	}

	cat := &Catalog{Plans: make([]Plan, 0, len(doc.Plans))}
	seenPlan := make(map[string]string, len(doc.Plans)) // external_id → path
	seenStep := make(map[string]string)                 // external_id → path

	for _, f := range doc.Plans {
		extID := planExternalID(f)
		if prev, ok := seenPlan[extID]; ok {
			return nil, fmt.Errorf(
				"ctid: normalize: duplicate plan external_id %q in %s and %s",
				extID, prev, f.Path,
			)
		}
		seenPlan[extID] = f.Path

		meta, err := encodePlanMetadata(f)
		if err != nil {
			return nil, fmt.Errorf("ctid: normalize: %s: metadata: %w", f.Path, err)
		}

		steps := make([]Step, 0, len(f.Steps))
		for i, raw := range f.Steps {
			pos := i + 1
			stepExt := stepExternalID(extID, raw, pos)
			if prev, ok := seenStep[stepExt]; ok {
				return nil, fmt.Errorf(
					"ctid: normalize: duplicate step external_id %q in %s and %s",
					stepExt, prev, f.Path,
				)
			}
			seenStep[stepExt] = f.Path

			tech := normalizeTechniqueID(raw.Technique.AttackID)
			if tech == "" {
				cat.MissingTechniques++
			}

			proc, err := encodeProcedure(raw)
			if err != nil {
				return nil, fmt.Errorf(
					"ctid: normalize: %s step %d: procedure: %w", f.Path, pos, err)
			}

			steps = append(steps, Step{
				ExternalID:          stepExt,
				Position:            pos,
				Name:                raw.Name,
				Description:         raw.Description,
				TechniqueExternalID: tech,
				Procedure:           proc,
			})
		}

		name := f.Details.AdversaryName
		if name == "" {
			name = extID
		}
		cat.Plans = append(cat.Plans, Plan{
			ExternalID:    extID,
			Name:          name,
			Description:   f.Details.AdversaryDescription,
			AdversaryName: f.Details.AdversaryName,
			Metadata:      meta,
			Steps:         steps,
		})
	}
	return cat, nil
}

// planExternalID prefers upstream details.id; otherwise the actor directory
// slug derived from the archive path (fin6 from fin6/Emulation_Plan/yaml/…).
func planExternalID(f rawPlanFile) string {
	if id := strings.TrimSpace(f.Details.ID); id != "" {
		return id
	}
	if slug := actorSlug(f.Path); slug != "" {
		return slug
	}
	// Last resort: basename without extension.
	base := path.Base(f.Path)
	base = strings.TrimSuffix(base, path.Ext(base))
	return strings.ToLower(base)
}

// stepExternalID prefers upstream step id; otherwise {plan}/{position}.
func stepExternalID(planExt string, s rawStep, pos int) string {
	if id := strings.TrimSpace(s.ID); id != "" {
		return id
	}
	return fmt.Sprintf("%s/%d", planExt, pos)
}

// actorSlug extracts the actor directory from a CTID archive path.
//
//	adversary_emulation_library-master/fin6/Emulation_Plan/yaml/FIN6.yaml → fin6
//	fin6/Emulation_Plan/yaml/FIN6.yaml → fin6
func actorSlug(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "Emulation_Plan") && i > 0 {
			return strings.ToLower(parts[i-1])
		}
	}
	return ""
}

func normalizeTechniqueID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !techniqueIDRE.MatchString(raw) {
		// Keep non-conforming ids as trimmed uppercase-T form when they look
		// close; otherwise empty so M3 does not inherit garbage.
		upper := strings.ToUpper(raw)
		if techniqueIDRE.MatchString(upper) {
			return upper
		}
		return ""
	}
	// Canonical: leading T upper, rest as upstream digits/dot.
	return "T" + raw[1:]
}

func encodePlanMetadata(f rawPlanFile) (json.RawMessage, error) {
	m := map[string]any{
		"path": f.Path,
	}
	if f.Details.AttackVersion != "" {
		m["attack_version"] = f.Details.AttackVersion
	}
	if f.Details.FormatVersion != "" {
		m["format_version"] = f.Details.FormatVersion
	}
	if slug := actorSlug(f.Path); slug != "" {
		m["actor_slug"] = slug
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func encodeProcedure(s rawStep) (json.RawMessage, error) {
	m := map[string]any{}
	if s.Tactic != "" {
		m["tactic"] = s.Tactic
	}
	if s.Technique.Name != "" {
		m["technique_name"] = s.Technique.Name
	}
	if s.ProcedureGroup != "" {
		m["procedure_group"] = s.ProcedureGroup
	}
	if s.ProcedureStep != "" {
		m["procedure_step"] = s.ProcedureStep
	}
	if s.CTISource != nil {
		m["cti_source"] = s.CTISource
	}
	if len(s.Platforms) > 0 {
		m["platforms"] = s.Platforms
	}
	if len(s.Executors) > 0 {
		execs := make([]map[string]any, 0, len(s.Executors))
		for _, e := range s.Executors {
			em := map[string]any{}
			if name := strings.TrimSpace(e.Name); name != "" {
				em["name"] = name
			}
			if e.Command != "" {
				em["command"] = e.Command
			}
			if e.Cleanup != "" {
				em["cleanup"] = e.Cleanup
			}
			if e.Timeout != nil {
				em["timeout"] = e.Timeout
			}
			if e.Elevation != nil {
				em["elevation_required"] = e.Elevation
			}
			if len(em) > 0 {
				execs = append(execs, em)
			}
		}
		if len(execs) > 0 {
			m["executors"] = execs
		}
	}
	if len(s.InputArguments) > 0 {
		m["input_arguments"] = s.InputArguments
	}
	if s.DependencyExecutorName != "" {
		m["dependency_executor_name"] = s.DependencyExecutorName
	}
	if s.Dependencies != nil {
		m["dependencies"] = s.Dependencies
	}
	if len(m) == 0 {
		return json.RawMessage(`{}`), nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
