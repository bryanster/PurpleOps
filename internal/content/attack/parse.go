package attack

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// stixDoc is the adapter-private AST.
type stixDoc struct {
	Version string
	Objects []stixObject
}

type stixObject struct {
	Type                 string          `json:"type"`
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	XMitreVersion        string          `json:"x_mitre_version"`
	XMitreIsSubtechnique bool            `json:"x_mitre_is_subtechnique"`
	XMitreShortname      string          `json:"x_mitre_shortname"`
	XMitreDeprecated     bool            `json:"x_mitre_deprecated"`
	Revoked              bool            `json:"revoked"`
	ExternalReferences   []stixExtRef    `json:"external_references"`
	KillChainPhases      []stixKillChain `json:"kill_chain_phases"`
	RelationshipType     string          `json:"relationship_type"`
	SourceRef            string          `json:"source_ref"`
	TargetRef            string          `json:"target_ref"`
}

type stixExtRef struct {
	SourceName string `json:"source_name"`
	ExternalID string `json:"external_id"`
	URL        string `json:"url"`
}

type stixKillChain struct {
	KillChainName string `json:"kill_chain_name"`
	PhaseName     string `json:"phase_name"`
}

type stixBundleJSON struct {
	Type    string       `json:"type"`
	ID      string       `json:"id"`
	Objects []stixObject `json:"objects"`
}

func parseSTIX(raw []byte) (*stixDoc, error) {
	payload, err := extractEnterpriseSTIX(raw)
	if err != nil {
		return nil, fmt.Errorf("attack: parse: %w", err)
	}
	var bundle stixBundleJSON
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&bundle); err != nil {
		return nil, fmt.Errorf("attack: parse: stix json: %w", err)
	}
	if bundle.Type != "" && bundle.Type != "bundle" {
		return nil, fmt.Errorf("attack: parse: expected STIX bundle, got type %q", bundle.Type)
	}
	if len(bundle.Objects) == 0 {
		return nil, fmt.Errorf("attack: parse: bundle contains no objects")
	}

	version := ""
	for _, o := range bundle.Objects {
		if o.Type == "x-mitre-collection" && o.XMitreVersion != "" {
			version = o.XMitreVersion
			break
		}
	}
	return &stixDoc{Version: version, Objects: bundle.Objects}, nil
}

func peekBundleVersion(raw []byte) (string, error) {
	doc, err := parseSTIX(raw)
	if err != nil {
		return "", err
	}
	return doc.Version, nil
}

// extractEnterpriseSTIX returns STIX JSON bytes from a raw JSON document or an
// archive that embeds one. Mobile/ICS paths are skipped.
func extractEnterpriseSTIX(raw []byte) ([]byte, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	if trim[0] == '{' || trim[0] == '[' {
		return trim, nil
	}
	// zip magic
	if len(raw) >= 4 && raw[0] == 'P' && raw[1] == 'K' {
		return extractFromZip(raw)
	}
	// gzip magic
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer gr.Close()
		ungzipped, err := io.ReadAll(gr)
		if err != nil {
			return nil, fmt.Errorf("gzip read: %w", err)
		}
		if len(bytes.TrimSpace(ungzipped)) > 0 && (bytes.TrimSpace(ungzipped)[0] == '{' || bytes.TrimSpace(ungzipped)[0] == '[') {
			return bytes.TrimSpace(ungzipped), nil
		}
		return extractFromTar(ungzipped)
	}
	// bare tar
	if looksLikeTar(raw) {
		return extractFromTar(raw)
	}
	return nil, fmt.Errorf("unrecognized payload (not STIX JSON, zip, or tar.gz)")
}

func extractFromZip(raw []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}
	var candidates []zip.File
	for i := range zr.File {
		f := zr.File[i]
		if f.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		base := path.Base(name)
		if !isEnterpriseSTIXName(name, base) {
			continue
		}
		candidates = append(candidates, *f)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("zip archive has no enterprise-attack STIX JSON")
	}
	// Prefer versioned enterprise-attack-X.Y.json over the unversioned alias.
	best := candidates[0]
	bestScore := scoreSTIXName(path.Base(best.Name))
	for _, c := range candidates[1:] {
		if s := scoreSTIXName(path.Base(c.Name)); s > bestScore {
			best = c
			bestScore = s
		}
	}
	rc, err := best.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSpace(data), nil
}

func extractFromTar(raw []byte) ([]byte, error) {
	tr := tar.NewReader(bytes.NewReader(raw))
	var (
		bestName  string
		bestData  []byte
		bestScore int
		found     bool
	)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Clean(strings.ReplaceAll(hdr.Name, "\\", "/"))
		base := path.Base(name)
		if !isEnterpriseSTIXName(name, base) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		score := scoreSTIXName(base)
		if !found || score > bestScore {
			bestName = name
			bestData = bytes.TrimSpace(data)
			bestScore = score
			found = true
		}
		_ = bestName
	}
	if !found {
		return nil, fmt.Errorf("tar archive has no enterprise-attack STIX JSON")
	}
	return bestData, nil
}

func isEnterpriseSTIXName(full, base string) bool {
	lower := strings.ToLower(full)
	if strings.Contains(lower, "mobile-attack") || strings.Contains(lower, "ics-attack") {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(base), ".json") {
		return false
	}
	b := strings.ToLower(base)
	return strings.HasPrefix(b, "enterprise-attack")
}

func scoreSTIXName(base string) int {
	// versioned file wins over unversioned enterprise-attack.json
	b := strings.ToLower(base)
	if b == "enterprise-attack.json" {
		return 1
	}
	if strings.HasPrefix(b, "enterprise-attack-") {
		return 2
	}
	return 0
}

func looksLikeTar(raw []byte) bool {
	// ustar magic at offset 257
	if len(raw) < 262 {
		return false
	}
	return string(raw[257:262]) == "ustar"
}

func latestEnterpriseVersion(indexJSON []byte) (string, error) {
	var idx struct {
		Collections []struct {
			Name     string `json:"name"`
			Versions []struct {
				Version string `json:"version"`
				URL     string `json:"url"`
			} `json:"versions"`
		} `json:"collections"`
	}
	if err := json.Unmarshal(indexJSON, &idx); err != nil {
		return "", fmt.Errorf("attack: index.json: %w", err)
	}
	for _, c := range idx.Collections {
		name := strings.ToLower(c.Name)
		if !strings.Contains(name, "enterprise") {
			continue
		}
		if len(c.Versions) == 0 {
			return "", fmt.Errorf("attack: index.json: Enterprise collection has no versions")
		}
		// versions[0] is latest per MITRE's index layout
		v := strings.TrimSpace(c.Versions[0].Version)
		if v == "" {
			return "", fmt.Errorf("attack: index.json: empty latest version label")
		}
		return v, nil
	}
	return "", fmt.Errorf("attack: index.json: no Enterprise ATT&CK collection found")
}

func mitreExternalID(refs []stixExtRef) string {
	for _, r := range refs {
		if strings.EqualFold(r.SourceName, "mitre-attack") && r.ExternalID != "" {
			return r.ExternalID
		}
	}
	return ""
}
