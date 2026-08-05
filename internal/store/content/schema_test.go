package content_test

import (
	"database/sql"
	"testing"

	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// The tests in this file go around the repositories and write SQL directly.
// Field safety comes from the schema (PLAN.md §4 pattern, same as identity).

func TestMigrationCreatesContentTables(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)

	want := []string{
		"content_source",
		"content_source_version",
		"content_sync_job",
		"content_tactic",
		"content_technique",
		"content_technique_tactic",
		"content_mitigation",
		"content_group",
		"content_software",
		"content_data_source",
		"content_procedure_template",
		"content_detection_rule_ref",
		"content_emulation_plan",
		"content_emulation_plan_step",
		"content_note",
	}
	rows, err := db.Read().QueryContext(t.Context(), `
		SELECT table_name FROM information_schema.tables
		WHERE table_catalog = current_database()
		  AND table_schema = 'content'
		ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("missing content.%s after migrate", name)
		}
	}
}

func TestSeedSourcesExist(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	sources := content.NewSources(db)

	list, err := sources.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 5 {
		t.Fatalf("seed sources = %d, want 5", len(list))
	}

	byKind := map[content.Kind]content.Source{}
	for _, s := range list {
		byKind[s.Kind] = s
	}

	// Four upstream disabled, custom enabled.
	for _, kind := range []content.Kind{
		content.KindAttack, content.KindAtomic, content.KindSigma, content.KindCTID,
	} {
		s, ok := byKind[kind]
		if !ok {
			t.Fatalf("missing seed kind %s", kind)
		}
		if s.Enabled {
			t.Errorf("%s: enabled = true, want false", kind)
		}
		if s.LicenseSPDX == "" || s.LicenseName == "" || s.Attribution == "" {
			t.Errorf("%s: license/attribution incomplete: spdx=%q name=%q attr=%q",
				kind, s.LicenseSPDX, s.LicenseName, s.Attribution)
		}
		if s.URL == "" {
			t.Errorf("%s: url empty", kind)
		}
		if s.Status != content.SourceStatusIdle {
			t.Errorf("%s: status = %q, want idle", kind, s.Status)
		}
	}

	custom, ok := byKind[content.KindCustom]
	if !ok {
		t.Fatal("missing custom seed")
	}
	if !custom.Enabled {
		t.Error("custom seed must be enabled")
	}
	if custom.ID != content.SourceIDCustom {
		t.Errorf("custom id = %q, want stable seed id", custom.ID)
	}

	// Stable ids for the four upstreams.
	wantIDs := map[content.Kind]string{
		content.KindAttack: content.SourceIDAttack,
		content.KindAtomic: content.SourceIDAtomic,
		content.KindSigma:  content.SourceIDSigma,
		content.KindCTID:   content.SourceIDCTID,
		content.KindCustom: content.SourceIDCustom,
	}
	for kind, id := range wantIDs {
		if byKind[kind].ID != id {
			t.Errorf("%s id = %q, want %q", kind, byKind[kind].ID, id)
		}
	}
}

func TestSchemaRefusesDuplicateTechniqueNaturalKey(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)

	mustExec(t, db, `
		INSERT INTO content.content_technique
			(id, source_id, version, external_id, name, description,
			 is_subtechnique, parent_external_id, created_at, updated_at)
		VALUES ('t1', ?, '15.1', 'T1059', 'Command and Scripting Interpreter', '',
			false, '', TIMESTAMP '2026-01-01', TIMESTAMP '2026-01-01')`,
		content.SourceIDAttack)

	err := writeSQL(t, db, `
		INSERT INTO content.content_technique
			(id, source_id, version, external_id, name, description,
			 is_subtechnique, parent_external_id, created_at, updated_at)
		VALUES ('t2', ?, '15.1', 'T1059', 'dup', '',
			false, '', TIMESTAMP '2026-01-01', TIMESTAMP '2026-01-01')`,
		content.SourceIDAttack)
	if err == nil {
		t.Fatal("duplicate (source_id, version, external_id) was accepted")
	}
	if !store.IsUniqueViolation(err) {
		t.Fatalf("error = %v, want unique violation", err)
	}
}

func TestSchemaRefusesInvalidSourceKind(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)

	err := writeSQL(t, db, `
		INSERT INTO content.content_source (
			id, kind, name, url, ref, enabled, status, last_synced_at, item_count, error,
			license_spdx, license_name, license_url, attribution, created_at, updated_at
		) VALUES (
			'x', 'wizard', 'W', '', '', false, 'idle', NULL, 0, '',
			'', '', '', '', TIMESTAMP '2026-01-01', TIMESTAMP '2026-01-01'
		)`)
	if err == nil {
		t.Fatal("invalid kind was accepted")
	}
}

func TestSchemaRefusesInvalidJobStatus(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)

	err := writeSQL(t, db, `
		INSERT INTO content.content_sync_job (
			id, source_id, version, kind, status, phase,
			progress_current, progress_total, message, error, created_by,
			created_at, started_at, finished_at, checkpoint
		) VALUES (
			'j1', ?, NULL, 'sync', 'flying', '',
			0, 0, '', '', '',
			TIMESTAMP '2026-01-01', NULL, NULL, '{}'::JSON
		)`, content.SourceIDAttack)
	if err == nil {
		t.Fatal("invalid job status was accepted")
	}
}

func TestSchemaRefusesInvalidSoftwareType(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)

	err := writeSQL(t, db, `
		INSERT INTO content.content_software
			(id, source_id, version, external_id, name, description, software_type, created_at, updated_at)
		VALUES ('s1', ?, '15.1', 'S0001', 'X', '', 'firmware',
			TIMESTAMP '2026-01-01', TIMESTAMP '2026-01-01')`,
		content.SourceIDAttack)
	if err == nil {
		t.Fatal("invalid software_type was accepted")
	}
}

func writeSQL(t *testing.T, db *store.DB, stmt string, args ...any) error {
	t.Helper()
	return db.Write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), stmt, args...)
		return err
	})
}

func mustExec(t *testing.T, db *store.DB, stmt string, args ...any) {
	t.Helper()
	if err := writeSQL(t, db, stmt, args...); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}
