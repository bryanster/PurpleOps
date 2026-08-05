package sigma

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Catalog is the normalized Sigma library for one rolling-head sync.
//
// It is a single [content.Object] so the runner's item_count prefers
// [Catalog.ItemCount] over len(objects).
type Catalog struct {
	Rules   []Rule
	Skipped int // rules seen without ATT&CK technique tags
}

// ItemCount is the detection rule total written for this sync.
func (c *Catalog) ItemCount() int64 { return int64(len(c.Rules)) }

// SuccessMessage is the operator-facing job message after a successful apply.
// Includes the skip count when unmapped rules were dropped.
func (c *Catalog) SuccessMessage() string {
	if c == nil {
		return ""
	}
	if c.Skipped <= 0 {
		return fmt.Sprintf("applied %d detection rules", len(c.Rules))
	}
	return fmt.Sprintf("applied %d detection rules, skipped %d unmapped", len(c.Rules), c.Skipped)
}

// Rule is one normalized Sigma detection reference ready for Apply.
type Rule struct {
	ExternalID           string
	Name                 string
	Description          string
	TechniqueExternalIDs []string
	Level                string
	Status               string
	Logsource            json.RawMessage // JSON object
	RuleYAML             string
}

// techniqueTagRE matches Sigma ATT&CK technique tags only:
//
//	attack.t1059
//	attack.t1059.001
//	ATTACK.T1059.001
//
// Tactic tags (attack.execution) and other attack.* labels are deliberately
// rejected — wrong technique links are worse than skips.
var techniqueTagRE = regexp.MustCompile(`(?i)^attack\.t(\d{4}(?:\.\d{3})?)$`)

func normalize(doc *sigmaDoc) (*Catalog, error) {
	if doc == nil {
		return nil, fmt.Errorf("sigma: normalize: nil document")
	}
	cat := &Catalog{Rules: make([]Rule, 0, len(doc.Rules))}
	seen := make(map[string]string, len(doc.Rules)) // external_id → path

	for _, f := range doc.Rules {
		techs := extractTechniques(f.Rule.Tags)
		if len(techs) == 0 {
			cat.Skipped++
			continue
		}

		extID := externalID(f)
		if prev, ok := seen[extID]; ok {
			return nil, fmt.Errorf(
				"sigma: normalize: duplicate external_id %q in %s and %s",
				extID, prev, f.Path,
			)
		}
		seen[extID] = f.Path

		logsource, err := encodeLogsource(f.Rule.Logsource)
		if err != nil {
			return nil, fmt.Errorf("sigma: normalize: %s: logsource: %w", f.Path, err)
		}

		cat.Rules = append(cat.Rules, Rule{
			ExternalID:           extID,
			Name:                 f.Rule.Title,
			Description:          f.Rule.Description,
			TechniqueExternalIDs: techs,
			Level:                f.Rule.Level,
			Status:               f.Rule.Status,
			Logsource:            logsource,
			RuleYAML:             f.Body,
		})
	}

	// Zero mapped rules is OK when skips > 0 (all-unmapped archive). Fail only
	// when we somehow have neither rules nor skips after a non-empty parse.
	if len(cat.Rules) == 0 && cat.Skipped == 0 {
		return nil, fmt.Errorf("sigma: normalize: zero rules parsed")
	}
	return cat, nil
}

// extractTechniques returns sorted unique ATT&CK technique external ids from
// Sigma tags. Conservative: only attack.t####(.###) patterns.
func extractTechniques(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	var out []string
	for _, tag := range tags {
		id, ok := techniqueFromTag(tag)
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// techniqueFromTag normalizes one Sigma tag into an ATT&CK external id.
//
// Accepted:
//
//	attack.t1059       → T1059
//	attack.t1059.001   → T1059.001
//	ATTACK.T1059.001   → T1059.001
//
// Rejected (among others): attack.execution, attack.defense_evasion,
// attack.s0002 (software), bare t1059 without the attack. prefix.
func techniqueFromTag(tag string) (string, bool) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", false
	}
	m := techniqueTagRE.FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	// m[1] is digits with optional .sub — uppercase the T prefix form.
	return "T" + m[1], true
}

// externalID prefers upstream rule id; otherwise the archive-relative path
// with any single GitHub repo-root prefix stripped.
func externalID(f rawRuleFile) string {
	if id := strings.TrimSpace(f.Rule.ID); id != "" {
		return id
	}
	return pathExternalID(f.Path)
}

func pathExternalID(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	// Strip a single leading directory when it looks like a GitHub archive root
	// (sigma-master/, sigma-2.0.0/, …) so external ids stay stable across ref cuts.
	if i := strings.IndexByte(p, '/'); i > 0 {
		first := p[:i]
		rest := p[i+1:]
		if looksLikeArchiveRoot(first) && (strings.HasPrefix(rest, "rules/") || strings.HasPrefix(rest, "rules-")) {
			p = rest
		}
	}
	return p
}

func looksLikeArchiveRoot(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "sigma") ||
		strings.Contains(lower, "sigmahq") ||
		// Generic GitHub archive root: repo-branch.
		strings.Count(name, "-") >= 1 && !strings.Contains(name, ".")
}

func encodeLogsource(in map[string]any) (json.RawMessage, error) {
	if in == nil {
		return json.RawMessage(`{}`), nil
	}
	// Drop nil values; keep a stable object.
	clean := make(map[string]any, len(in))
	for k, v := range in {
		if v == nil {
			continue
		}
		clean[k] = v
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
