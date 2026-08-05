package attack

import (
	"fmt"
	"strings"

	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// Catalog is the normalized ATT&CK snapshot for one version. Apply consumes it.
type Catalog struct {
	Version     string
	Tactics     []Tactic
	Techniques  []Technique
	Mitigations []Mitigation
	Groups      []Group
	Software    []Software
	DataSources []DataSource
	// TechTactics maps technique external_id → tactic external_ids.
	TechTactics map[string][]string
	// TechMitigations maps technique external_id → mitigation external_ids.
	TechMitigations map[string][]string
}

// ItemCount is the library object total written for this version (excludes join
// rows). The runner prefers this over len([]Object) when the adapter returns a
// single catalog envelope.
func (c *Catalog) ItemCount() int64 {
	if c == nil {
		return 0
	}
	return int64(len(c.Tactics) + len(c.Techniques) + len(c.Mitigations) +
		len(c.Groups) + len(c.Software) + len(c.DataSources))
}

// Tactic is a normalized x-mitre-tactic.
type Tactic struct {
	ExternalID  string
	Name        string
	Description string
	Shortname   string
}

// Technique is a normalized attack-pattern.
type Technique struct {
	ExternalID       string
	Name             string
	Description      string
	IsSubtechnique   bool
	ParentExternalID string
	// KillChainShortnames are tactic shortnames from kill_chain_phases.
	KillChainShortnames []string
}

// Mitigation is a normalized course-of-action.
type Mitigation struct {
	ExternalID  string
	Name        string
	Description string
}

// Group is a normalized intrusion-set.
type Group struct {
	ExternalID  string
	Name        string
	Description string
}

// Software is malware or tool.
type Software struct {
	ExternalID   string
	Name         string
	Description  string
	SoftwareType storecontent.SoftwareType
}

// DataSource is x-mitre-data-source or x-mitre-data-component.
type DataSource struct {
	ExternalID  string
	Name        string
	Description string
}

func normalize(doc *stixDoc) (*Catalog, error) {
	if doc == nil {
		return nil, fmt.Errorf("attack: normalize: nil document")
	}
	if doc.Version == "" {
		return nil, fmt.Errorf("attack: normalize: missing version")
	}

	bySTIX := make(map[string]stixObject, len(doc.Objects))
	for _, o := range doc.Objects {
		if o.ID != "" {
			bySTIX[o.ID] = o
		}
	}

	cat := &Catalog{
		Version:         doc.Version,
		TechTactics:     map[string][]string{},
		TechMitigations: map[string][]string{},
	}

	// shortname → tactic external id
	shortToTactic := map[string]string{}

	for _, o := range doc.Objects {
		if o.Revoked || o.XMitreDeprecated {
			continue
		}
		switch o.Type {
		case "x-mitre-tactic":
			ext := mitreExternalID(o.ExternalReferences)
			if ext == "" {
				return nil, fmt.Errorf("attack: normalize: tactic %s missing mitre-attack external_id", o.ID)
			}
			if o.Name == "" {
				return nil, fmt.Errorf("attack: normalize: tactic %s (%s) missing name", ext, o.ID)
			}
			cat.Tactics = append(cat.Tactics, Tactic{
				ExternalID:  ext,
				Name:        o.Name,
				Description: o.Description,
				Shortname:   o.XMitreShortname,
			})
			if o.XMitreShortname != "" {
				shortToTactic[o.XMitreShortname] = ext
			}
		case "attack-pattern":
			ext := mitreExternalID(o.ExternalReferences)
			if ext == "" {
				return nil, fmt.Errorf("attack: normalize: technique %s missing mitre-attack external_id", o.ID)
			}
			if o.Name == "" {
				return nil, fmt.Errorf("attack: normalize: technique %s (%s) missing name", ext, o.ID)
			}
			var shorts []string
			for _, kc := range o.KillChainPhases {
				if kc.KillChainName == "mitre-attack" || kc.KillChainName == "mitre-mobile-attack" || kc.KillChainName == "mitre-ics-attack" {
					// Enterprise only — skip non-enterprise kill chains if any slip in.
					if kc.KillChainName != "mitre-attack" {
						continue
					}
					if kc.PhaseName != "" {
						shorts = append(shorts, kc.PhaseName)
					}
				}
			}
			cat.Techniques = append(cat.Techniques, Technique{
				ExternalID:          ext,
				Name:                o.Name,
				Description:         o.Description,
				IsSubtechnique:      o.XMitreIsSubtechnique,
				KillChainShortnames: shorts,
			})
		case "course-of-action":
			ext := mitreExternalID(o.ExternalReferences)
			if ext == "" {
				// Some COAs are non-mitigation stubs; skip those without ATT&CK ids.
				continue
			}
			if !strings.HasPrefix(ext, "M") {
				continue
			}
			cat.Mitigations = append(cat.Mitigations, Mitigation{
				ExternalID:  ext,
				Name:        o.Name,
				Description: o.Description,
			})
		case "intrusion-set":
			ext := mitreExternalID(o.ExternalReferences)
			if ext == "" {
				continue
			}
			cat.Groups = append(cat.Groups, Group{
				ExternalID:  ext,
				Name:        o.Name,
				Description: o.Description,
			})
		case "malware", "tool":
			ext := mitreExternalID(o.ExternalReferences)
			if ext == "" {
				continue
			}
			st := storecontent.SoftwareMalware
			if o.Type == "tool" {
				st = storecontent.SoftwareTool
			}
			cat.Software = append(cat.Software, Software{
				ExternalID:   ext,
				Name:         o.Name,
				Description:  o.Description,
				SoftwareType: st,
			})
		case "x-mitre-data-source", "x-mitre-data-component":
			ext := mitreExternalID(o.ExternalReferences)
			if ext == "" {
				// Components sometimes lack external_ids; fall back to STIX id tail.
				ext = o.ID
			}
			cat.DataSources = append(cat.DataSources, DataSource{
				ExternalID:  ext,
				Name:        o.Name,
				Description: o.Description,
			})
		}
	}

	if len(cat.Techniques) == 0 {
		return nil, fmt.Errorf("attack: normalize: empty technique table after parse (version %s)", doc.Version)
	}

	// Resolve kill-chain shortnames → tactic external ids.
	techBySTIX := map[string]string{} // stix id → external id
	for _, o := range doc.Objects {
		if o.Type != "attack-pattern" || o.Revoked || o.XMitreDeprecated {
			continue
		}
		if ext := mitreExternalID(o.ExternalReferences); ext != "" {
			techBySTIX[o.ID] = ext
		}
	}
	mitBySTIX := map[string]string{}
	for _, o := range doc.Objects {
		if o.Type != "course-of-action" || o.Revoked {
			continue
		}
		if ext := mitreExternalID(o.ExternalReferences); ext != "" && strings.HasPrefix(ext, "M") {
			mitBySTIX[o.ID] = ext
		}
	}

	for i := range cat.Techniques {
		t := &cat.Techniques[i]
		seen := map[string]struct{}{}
		var tactics []string
		for _, short := range t.KillChainShortnames {
			tac, ok := shortToTactic[short]
			if !ok {
				continue
			}
			if _, dup := seen[tac]; dup {
				continue
			}
			seen[tac] = struct{}{}
			tactics = append(tactics, tac)
		}
		if len(tactics) > 0 {
			cat.TechTactics[t.ExternalID] = tactics
		}
	}

	// Relationships: subtechnique-of, mitigates.
	for _, o := range doc.Objects {
		if o.Type != "relationship" || o.Revoked {
			continue
		}
		switch o.RelationshipType {
		case "subtechnique-of":
			childExt := techBySTIX[o.SourceRef]
			parentExt := techBySTIX[o.TargetRef]
			if childExt == "" || parentExt == "" {
				continue
			}
			for i := range cat.Techniques {
				if cat.Techniques[i].ExternalID == childExt {
					cat.Techniques[i].IsSubtechnique = true
					cat.Techniques[i].ParentExternalID = parentExt
					break
				}
			}
		case "mitigates":
			mitExt := mitBySTIX[o.SourceRef]
			techExt := techBySTIX[o.TargetRef]
			if mitExt == "" || techExt == "" {
				continue
			}
			cat.TechMitigations[techExt] = appendUnique(cat.TechMitigations[techExt], mitExt)
		}
	}

	for i := range cat.Techniques {
		t := &cat.Techniques[i]
		if t.IsSubtechnique && t.ParentExternalID == "" {
			if dot := strings.LastIndex(t.ExternalID, "."); dot > 0 {
				t.ParentExternalID = t.ExternalID[:dot]
			} else {
				return nil, fmt.Errorf(
					"attack: normalize: sub-technique %s has no parent link", t.ExternalID,
				)
			}
		}
		if !t.IsSubtechnique {
			t.ParentExternalID = ""
		}
	}

	return cat, nil
}

func appendUnique(in []string, v string) []string {
	for _, x := range in {
		if x == v {
			return in
		}
	}
	return append(in, v)
}
