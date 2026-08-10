package cli

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestBackupCreatesANonEmptyArchiveWithExpectedMembers runs backup against a
// temp directory that has a DuckDB file, an evidence dir, and a generated
// session secret, then inspects the archive.
func TestBackupCreatesANonEmptyArchiveWithExpectedMembers(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	evidenceDir := filepath.Join(dir, "evidence")
	sessionSecret := filepath.Join(dir, "session.secret")
	archivePath := filepath.Join(dir, "backup.tar.gz")

	// Create the evidence directory with a file in it.
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "blob.bin"), []byte("evidence data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a dummy session secret — backup picks it up from the data dir.
	if err := os.WriteFile(sessionSecret, []byte("not-a-real-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a dummy DuckDB. runIn creates the database via store.Open, so
	// we can run `db info` first to create it, then backup.
	env := map[string]string{
		"BLACKLIGHT_DB_PATH":      dbPath,
		"BLACKLIGHT_EVIDENCE_DIR": evidenceDir,
	}
	result := run(t, env, "db", "info", "--db", dbPath, "--json")
	if result.code != ExitOK {
		t.Fatalf("db info failed: %s", result.stderr)
	}

	// Now run backup.
	result = run(t, env, "backup", "--db", dbPath, "-o", archivePath)
	if result.code != ExitOK {
		t.Fatalf("backup failed: %s", result.stderr)
	}

	// Verify the archive exists and is non-empty.
	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("backup archive is empty")
	}

	// Inspect the archive contents.
	members := tarMembers(t, archivePath)
	memberSet := make(map[string]bool)
	for _, m := range members {
		memberSet[m] = true
	}

	// Expected: the database file (named after its base), evidence/, session.secret.
	checkMember := func(name string) {
		if !memberSet[name] {
			t.Errorf("archive is missing expected member %q; got: %v", name, members)
		}
	}
	checkMember("test.duckdb")
	checkMember("evidence")
	checkMember("evidence/blob.bin")
	checkMember("session.secret")
}

// TestBackupRefusesWhenDBPathMissing: the database file must exist. If the
// parent directory does not exist, store.Open returns an error.
func TestBackupRefusesWhenDBPathMissing(t *testing.T) {
	result := run(t, nil, "backup", "--db", "/nonexistent/path/test.duckdb")
	if result.code == ExitOK {
		t.Fatal("backup succeeded against a nonexistent database path")
	}
}

// TestBackupRoundTrip: take a backup, then verify the archive contains the same
// database content by extracting and checking.
func TestBackupRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	evidenceDir := filepath.Join(dir, "evidence")
	archivePath := filepath.Join(dir, "backup.tar.gz")

	// Create evidence.
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	originalContent := []byte("evidence round-trip data")
	if err := os.WriteFile(filepath.Join(evidenceDir, "screenshot.png"), originalContent, 0o644); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"BLACKLIGHT_DB_PATH":      dbPath,
		"BLACKLIGHT_EVIDENCE_DIR": evidenceDir,
	}

	// Create the database.
	result := run(t, env, "db", "info", "--db", dbPath)
	if result.code != ExitOK {
		t.Fatalf("db info failed: %s", result.stderr)
	}

	// Take backup.
	result = run(t, env, "backup", "--db", dbPath, "-o", archivePath)
	if result.code != ExitOK {
		t.Fatalf("backup failed: %s", result.stderr)
	}

	// Extract the archive to a restore directory.
	restoreDir := t.TempDir()
	extractTarGz(t, archivePath, restoreDir)

	// Verify the database file was restored.
	restoredDB := filepath.Join(restoreDir, "test.duckdb")
	if _, err := os.Stat(restoredDB); err != nil {
		t.Fatalf("restored database missing: %v", err)
	}

	// Verify the evidence file was restored.
	restoredEvidence := filepath.Join(restoreDir, "evidence", "screenshot.png")
	got, err := os.ReadFile(restoredEvidence)
	if err != nil {
		t.Fatalf("restored evidence missing: %v", err)
	}
	if string(got) != string(originalContent) {
		t.Errorf("restored evidence content mismatch: got %q, want %q", got, originalContent)
	}
}

// TestBackupJSONOutput verifies the --json flag produces parseable output.
func TestBackupJSONOutput(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	archivePath := filepath.Join(dir, "backup.tar.gz")

	env := map[string]string{"BLACKLIGHT_DB_PATH": dbPath}

	// Create the database.
	result := run(t, env, "db", "info", "--db", dbPath)
	if result.code != ExitOK {
		t.Fatalf("db info failed: %s", result.stderr)
	}

	result = run(t, env, "backup", "--db", dbPath, "-o", archivePath, "--json")
	if result.code != ExitOK {
		t.Fatalf("backup --json failed: %s", result.stderr)
	}

	doc := decodeJSON(t, result.stdout)
	if got := field[string](t, doc, "archive"); got != archivePath {
		t.Errorf("archive = %q, want %q", got, archivePath)
	}
	if got := field[float64](t, doc, "sizeBytes"); got <= 0 {
		t.Errorf("sizeBytes = %v, want > 0", got)
	}
	membersRaw, ok := doc["members"]
	if !ok {
		t.Fatal("members field missing from JSON output")
	}
	members, ok := membersRaw.([]interface{})
	if !ok {
		t.Fatalf("members is %T, want array", membersRaw)
	}
	if len(members) == 0 {
		t.Error("members is empty")
	}
}

// tarMembers lists the names of every entry in a tar.gz archive.
func tarMembers(t *testing.T, path string) []string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var members []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		members = append(members, hdr.Name)
	}
	return members
}

// extractTarGz extracts a tar.gz archive to dst.
func extractTarGz(t *testing.T, archive, dst string) {
	t.Helper()

	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		target := filepath.Join(dst, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			out, err := os.Create(target)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				t.Fatal(err)
			}
			out.Close()
		}
	}
}
