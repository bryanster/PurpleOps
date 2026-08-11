package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestFormatVersion(t *testing.T) {
	if FormatVersion != 1 {
		t.Fatalf("FormatVersion = %d, want 1 — changing this breaks readers", FormatVersion)
	}
}

// TestManifestFieldsOrder verifies formatVersion is the first JSON key.
func TestManifestFieldsOrder(t *testing.T) {
	m := Manifest{
		FormatVersion:  FormatVersion,
		ExportedAt:     "2025-01-01T00:00:00Z",
		ToolVersion:    "dev",
		EngagementID:   "test-id",
		EngagementName: "test",
		Client:         "test",
		Mode:           "standard",
		AttackVersion:  "15.1",
		BlindFiltered:  false,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, `{"formatVersion"`) {
		t.Errorf("manifest JSON does not start with formatVersion: %s", s[:min(120, len(s))])
	}
}

// TestNoSecrets checks that no archive types carry secret-bearing field names.
func TestNoSecrets(t *testing.T) {
	// Fields whose JSON name, if present in an archive type, would be a secret leak.
	forbidden := []string{
		"email", "password", "passwordHash", "secret", "sessionToken",
		"token", "mfaSecret", "recoveryCode", "secretKey",
	}

	// Collect all known archive JSON-serializable types.
	checkType := func(name string, v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		// Parse back to check all field names.
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		checkFields(t, name, m, forbidden)
	}

	checkType("Manifest", Manifest{})
	checkType("EngagementJSON", EngagementJSON{})
	checkType("ScenarioJSON", ScenarioJSON{})
	checkType("StepJSON", StepJSON{})
	checkType("ExecutionJSON", ExecutionJSON{})
	checkType("ExecutionStatusJSON", ExecutionStatusJSON{})
	checkType("FindingJSON", FindingJSON{})
	checkType("CommentJSON", CommentJSON{})
	checkType("UserRef", UserRef{})
	checkType("EvidenceEntry", EvidenceEntry{})
	checkType("EvidenceManifest", EvidenceManifest{})
}

func checkFields(t *testing.T, parent string, m map[string]any, forbidden []string) {
	t.Helper()
	for key, val := range m {
		lower := strings.ToLower(key)
		for _, f := range forbidden {
			if strings.Contains(lower, f) {
				t.Errorf("%s: field %q matches forbidden pattern %q", parent, key, f)
			}
		}
		// Recurse into nested objects.
		if nested, ok := val.(map[string]any); ok {
			child := parent + "." + key
			checkFields(t, child, nested, forbidden)
		}
	}
}

// TestWriteArchiveRoundTrip writes a minimal archive to a buffer, re-reads it,
// and asserts the expected members are present.
func TestWriteArchiveRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	engagementData := EngagementArchive{
		Engagement: EngagementJSON{
			ID:                "eng-1",
			Name:              "Test Engagement",
			Client:            "Test Client",
			Status:            "active",
			Mode:              "standard",
			AttackVersion:     "15.1",
			AutoRevealOnStart: false,
			CreatedBy:         UserRef{ID: "user-1", DisplayName: "Test User"},
		},
	}

	evidenceEntries := []EvidenceEntry{
		{
			ID:          "ev-1",
			BlobSHA256:  "0000000000000000000000000000000000000000000000000000000000000001",
			Filename:    "test.txt",
			Side:        "red",
			ExecutionID: "exec-1",
			UploadedBy:  UserRef{ID: "user-1", DisplayName: "Test User"},
			Size:        5,
			MIME:        "text/plain",
		},
	}

	// Provide a fake evidence opener.
	evidenceOpener := func(sha256hex string) (io.ReadCloser, error) {
		if sha256hex == "0000000000000000000000000000000000000000000000000000000000000001" {
			return io.NopCloser(strings.NewReader("hello")), nil
		}
		return nil, fmt.Errorf("blob not found: %s", sha256hex)
	}

	err := WriteArchive(&buf, WriteOptions{
		EngagementID:   "eng-1",
		EngagementName: "Test Engagement",
		Client:         "Test Client",
		Mode:           "standard",
		AttackVersion:  "15.1",
		BlindFiltered:  false,
		Engagement:     engagementData,
		Evidence:       evidenceEntries,
		EvidenceOpener: evidenceOpener,
	})
	if err != nil {
		t.Fatalf("WriteArchive: %v", err)
	}

	// Re-read the ZIP.
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	// Check expected members.
	members := map[string]bool{}
	for _, f := range zr.File {
		members[f.Name] = true
	}

	expected := []string{
		"manifest.json",
		"engagement.json",
		"evidence.json",
		"activity.jsonl",
		"evidence/0000000000000000000000000000000000000000000000000000000000000001",
	}
	for _, name := range expected {
		if !members[name] {
			t.Errorf("missing archive member: %s", name)
		}
	}

	// Verify manifest formatVersion is first.
	mf := openZipFile(t, zr, "manifest.json")
	var manifest Manifest
	if err := json.Unmarshal(mf, &manifest); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	if manifest.FormatVersion != FormatVersion {
		t.Errorf("manifest FormatVersion = %d, want %d", manifest.FormatVersion, FormatVersion)
	}

	// Verify evidence blob content.
	blob := openZipFile(t, zr, "evidence/0000000000000000000000000000000000000000000000000000000000000001")
	if string(blob) != "hello" {
		t.Errorf("evidence blob = %q, want %q", string(blob), "hello")
	}
}

// TestEvidenceDedup verifies a blob appears once even when referenced twice.
func TestEvidenceDedup(t *testing.T) {
	var buf bytes.Buffer

	entries := []EvidenceEntry{
		{ID: "ev-1", BlobSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1", ExecutionID: "exec-1", Side: "red", UploadedBy: UserRef{ID: "u1"}, Size: 5, MIME: "text/plain", Filename: "a.txt"},
		{ID: "ev-2", BlobSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1", ExecutionID: "exec-2", Side: "red", UploadedBy: UserRef{ID: "u1"}, Size: 5, MIME: "text/plain", Filename: "a.txt"},
	}

	opener := func(sha256hex string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("shared")), nil
	}

	err := WriteArchive(&buf, WriteOptions{
		EngagementID:   "eng-1",
		EngagementName: "test",
		Client:         "test",
		Mode:           "standard",
		AttackVersion:  "15.1",
		Evidence:       entries,
		EvidenceOpener: opener,
	})
	if err != nil {
		t.Fatalf("WriteArchive: %v", err)
	}

	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len())) //nolint:errcheck

	// Count evidence files.
	evidenceCount := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "evidence/") {
			evidenceCount++
		}
	}
	if evidenceCount != 1 {
		t.Errorf("evidence/ file count = %d, want 1 (dedup)", evidenceCount)
	}

	// evidence.json should have 2 entries.
	ej := openZipFile(t, zr, "evidence.json")
	var em EvidenceManifest
	if err := json.Unmarshal(ej, &em); err != nil {
		t.Fatalf("evidence.json: %v", err)
	}
	if len(em.Entries) != 2 {
		t.Errorf("evidence.json entries = %d, want 2", len(em.Entries))
	}
}

// TestMissingBlobFails verifies a missing blob returns an error.
func TestMissingBlobFails(t *testing.T) {
	var buf bytes.Buffer

	entries := []EvidenceEntry{
		{ID: "ev-1", BlobSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb1", ExecutionID: "exec-1", Side: "red", UploadedBy: UserRef{ID: "u1"}, Size: 5, MIME: "text/plain", Filename: "a.txt"},
	}

	opener := func(sha256hex string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("blob %s not found on disk", sha256hex)
	}

	err := WriteArchive(&buf, WriteOptions{
		EngagementID:   "eng-1",
		EngagementName: "test",
		Client:         "test",
		Mode:           "standard",
		AttackVersion:  "15.1",
		Evidence:       entries,
		EvidenceOpener: opener,
	})
	if err == nil {
		t.Fatal("WriteArchive should fail when a blob is missing")
	}
	if !strings.Contains(err.Error(), "bbbbbbbb") {
		t.Errorf("error should name the missing blob sha256: %v", err)
	}
}

func openZipFile(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer rc.Close()
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return buf.Bytes()
		}
	}
	t.Fatalf("member %s not found in archive", name)
	return nil
}
